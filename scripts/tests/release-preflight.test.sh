#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

repo=''
release_fail_validate=0
release_fail_score=0

new_repo() {
  local name="$1"
  repo="$tmp/$name-repo"
  release_fail_validate=0
  release_fail_score=0
  mkdir -p \
    "$repo/scripts" "$repo/evals/studies/installed-ab" \
    "$repo/plugins/megapowers/.claude-plugin" \
    "$repo/plugins/megapowers/.codex-plugin"
  cp "$ROOT/scripts/release.sh" "$repo/scripts/release.sh"
  cp "$ROOT/evals/studies/installed-ab/run.go" "$repo/evals/studies/installed-ab/run.go"
  printf '%s\n' '# Changelog' '' '## 1.2.3 - test' > "$repo/CHANGELOG.md"
  printf '%s\n' '{"name":"megapowers","version":"1.2.3"}' > "$repo/plugins/megapowers/.claude-plugin/plugin.json"
  printf '%s\n' '{"name":"megapowers","version":"1.2.3"}' > "$repo/plugins/megapowers/.codex-plugin/plugin.json"
  printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "validate\n" >> gate.log' \
    '[[ ${RELEASE_TEST_VALIDATE_FAIL:-0} != 1 ]]' \
    > "$repo/scripts/validate.sh"
  printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "deterministic\n" >> gate.log' \
    '[[ ${1:-} == --json && -n ${2:-} ]]' \
    ': > "$2"' \
    > "$repo/evals/run-all.sh"
  printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "fresh\n" >> gate.log' \
    > "$repo/scripts/check-freshness.sh"
  printf '%s\n' \
    'package main' \
    'import "os"' \
    'func main() {' \
    '  file, err := os.OpenFile("gate.log", os.O_APPEND|os.O_WRONLY, 0o600)' \
    '  if err != nil { panic(err) }' \
    '  if _, err := file.WriteString("score\n"); err != nil { panic(err) }' \
    '  if err := file.Close(); err != nil { panic(err) }' \
    '  if os.Getenv("RELEASE_TEST_SCORE_FAIL") == "1" { os.Exit(1) }' \
    '}' \
    > "$repo/evals/score.go"
  chmod +x "$repo/scripts/release.sh" "$repo/scripts/validate.sh" \
    "$repo/scripts/check-freshness.sh" "$repo/evals/run-all.sh"
  : > "$repo/gate.log"
  git -C "$repo" init -q
  git -C "$repo" config user.name fixture
  git -C "$repo" config user.email fixture@example.invalid
  git -C "$repo" add .
  git -C "$repo" -c commit.gpgsign=false commit -qm fixture
}

run_release() {
  set +e
  RELEASE_TEST_VALIDATE_FAIL="$release_fail_validate" \
    RELEASE_TEST_SCORE_FAIL="$release_fail_score" \
    bash "$repo/scripts/release.sh" 1.2.3 >"$tmp/release.out" 2>&1
  local rc=$?
  set -e
  return "$rc"
}

unchanged_version() {
  [[ $(jq -r .version "$repo/plugins/megapowers/.claude-plugin/plugin.json") == 1.2.3 ]]
}

new_repo certificate-argument
if bash "$repo/scripts/release.sh" 1.2.3 --certificate-root "$tmp/cert" >"$tmp/release.out" 2>&1; then
  echo 'FAIL obsolete certificate argument was accepted'; exit 1
fi
[[ ! -s $repo/gate.log ]] || { echo 'FAIL gates ran for obsolete certificate arguments'; exit 1; }

new_repo unstamped
jq '.version = "0.0.0"' "$repo/plugins/megapowers/.claude-plugin/plugin.json" > "$tmp/manifest.json"
mv "$tmp/manifest.json" "$repo/plugins/megapowers/.claude-plugin/plugin.json"
if run_release; then echo 'FAIL unstamped candidate was accepted'; exit 1; fi
[[ ! -s $repo/gate.log ]] || { echo 'FAIL gates ran before version acceptance'; exit 1; }

new_repo old-version
sed -i '2i ## 1.2.4 - newer' "$repo/CHANGELOG.md"
if run_release; then echo 'FAIL non-latest release version was accepted'; exit 1; fi
[[ ! -s $repo/gate.log ]] || { echo 'FAIL gates ran for a non-latest release version'; exit 1; }

new_repo existing-tag
git -C "$repo" tag v1.2.3
if run_release; then echo 'FAIL existing release tag was accepted'; exit 1; fi
[[ ! -s $repo/gate.log ]] || { echo 'FAIL gates ran for an existing release tag'; exit 1; }

new_repo ignored-plugin
printf '%s\n' 'plugins/megapowers/ignored-payload' >> "$repo/.git/info/exclude"
printf '%s\n' 'unshipped behavior' > "$repo/plugins/megapowers/ignored-payload"
if run_release; then echo 'FAIL ignored plugin payload was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL ignored plugin payload mutated manifests'; exit 1; }

new_repo hidden-plugin-drift
git -C "$repo" update-index --assume-unchanged plugins/megapowers/.claude-plugin/plugin.json
jq '.description = "uncommitted"' "$repo/plugins/megapowers/.claude-plugin/plugin.json" > "$tmp/manifest.json"
mv "$tmp/manifest.json" "$repo/plugins/megapowers/.claude-plugin/plugin.json"
if run_release; then echo 'FAIL hidden plugin-tree drift was accepted'; exit 1; fi

new_repo dirty
printf '%s\n' dirty > "$repo/untracked.txt"
if run_release; then echo 'FAIL dirty candidate was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL dirty candidate mutated manifests'; exit 1; }

new_repo validation-failure
release_fail_validate=1
if run_release; then echo 'FAIL failed canonical validation was accepted'; exit 1; fi
[[ $(cat "$repo/gate.log") == 'validate' ]] || { echo 'FAIL validation did not fail first'; exit 1; }

new_repo score-failure
release_fail_score=1
if run_release; then echo 'FAIL failed deterministic scoring was accepted'; exit 1; fi
[[ $(cat "$repo/gate.log") == $'validate\ndeterministic\nscore' ]] || {
  echo 'FAIL scoring gate order changed'; cat "$repo/gate.log"; exit 1
}

new_repo valid
run_release || { cat "$tmp/release.out"; echo 'FAIL valid deterministic candidate was refused'; exit 1; }
unchanged_version || { echo 'FAIL validated candidate manifest changed'; exit 1; }
[[ $(cat "$repo/gate.log") == $'validate\ndeterministic\nscore\nfresh' ]] || {
  echo 'FAIL deterministic release gate order changed'; cat "$repo/gate.log"; exit 1
}
[[ -z $(git -C "$repo" tag --list) ]] || { echo 'FAIL release preflight created a tag'; exit 1; }

printf 'release preflight contract: ok\n'
