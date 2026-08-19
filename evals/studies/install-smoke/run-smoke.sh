#!/usr/bin/env bash
# Fresh-home install and cached-byte smoke test for Claude Code and Codex.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_DEFAULT="$(cd "$HERE/../../.." && pwd)"
. "$HERE/../lib.sh"

QUOTE_SENTENCE="A research conclusion is not authority to implement or publish."

quote_ok() {
  tr '\n' ' ' < "$1" 2>/dev/null | grep -qF "$QUOTE_SENTENCE"
}

installed_skill_ok() { # <source-skill> <cached-skill>
  cmp -s "$1" "$2" && quote_ok "$2"
}

results_ok() { # <results.tsv> <fail-on-skip:0|1>
  local results="$1" fail_on_skip="$2"
  grep -q $'\tFAIL\t' "$results" && return 1
  [ "$fail_on_skip" -eq 1 ] && grep -q $'\tSKIP\t' "$results" && return 1
  grep -q $'\tPASS\t' "$results"
}

if [ "${1:-}" = "--selftest" ]; then
  st="$(mktemp -d)"; trap 'rm -rf "$st"' EXIT; sf=0
  printf '%s\n' "$QUOTE_SENTENCE" > "$st/verbatim.out"
  cp "$st/verbatim.out" "$st/cached.out"
  printf 'Research does not automatically allow a change.\n' > "$st/generic.out"
  if quote_ok "$st/verbatim.out"; then echo "ok   verbatim sentence matches"; else echo "FAIL verbatim sentence not matched"; sf=1; fi
  printf '%s\n' "${QUOTE_SENTENCE/research/Research}" > "$st/midcase.out"
  if quote_ok "$st/midcase.out"; then echo "FAIL case change matched"; sf=1; else echo "ok   case change rejected"; fi
  if quote_ok "$st/generic.out"; then echo "FAIL generic phrasing matched"; sf=1; else echo "ok   generic phrasing rejected"; fi
  if installed_skill_ok "$st/verbatim.out" "$st/cached.out"; then echo "ok   identical installed bytes accepted"; else echo "FAIL identical installed bytes rejected"; sf=1; fi
  printf 'mutation\n' >> "$st/cached.out"
  if installed_skill_ok "$st/verbatim.out" "$st/cached.out"; then echo "FAIL mutated installed bytes accepted"; sf=1; else echo "ok   mutated installed bytes rejected"; fi
  printf 'claude\tSKIP\tCLI unavailable\ncodex\tSKIP\tCLI unavailable\n' > "$st/all-skip.tsv"
  if results_ok "$st/all-skip.tsv" 0; then echo "FAIL all-SKIP results accepted"; sf=1; else echo "ok   all-SKIP results rejected"; fi
  printf 'claude\tPASS\tinstalled\ncodex\tSKIP\tCLI unavailable\n' > "$st/mixed.tsv"
  if results_ok "$st/mixed.tsv" 0; then echo "ok   optional SKIP accepted with a PASS"; else echo "FAIL optional SKIP rejected"; sf=1; fi
  if results_ok "$st/mixed.tsv" 1; then echo "FAIL strict SKIP accepted"; sf=1; else echo "ok   strict SKIP rejected"; fi
  if [ "$sf" -eq 0 ]; then echo "install-smoke selftest: PASS"; else echo "install-smoke selftest: FAIL"; fi
  exit "$sf"
fi

OUT="" REPO="$REPO_DEFAULT" HARNESSES="claude,codex"
SOURCE="" REF="" VERSION="" FAIL_ON_SKIP=0 FETCHED=""
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --source) SOURCE="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --fail-on-skip) FAIL_ON_SKIP=1; shift ;;
    --harnesses) HARNESSES="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ -n "$OUT" ] || { echo "usage: run-smoke.sh --out DIR [--harnesses claude,codex] [--repo DIR | --source OWNER/REPO --ref TAG --version VERSION]" >&2; exit 2; }
for requested in ${HARNESSES//,/ }; do
  case "$requested" in claude|codex) ;; *) echo "unsupported harness: $requested" >&2; exit 2 ;; esac
done
if [ -n "$SOURCE" ]; then
  [ "$REPO" = "$REPO_DEFAULT" ] || { echo "--repo and --source are mutually exclusive" >&2; exit 2; }
  [ -n "$REF" ] && [ -n "$VERSION" ] || { echo "--source requires --ref and --version" >&2; exit 2; }
  FAIL_ON_SKIP=1
elif [ -n "$REF$VERSION" ]; then
  echo "--ref and --version require --source" >&2
  exit 2
fi
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 2; }
mkdir -p "$OUT"; OUT="$(cd "$OUT" && pwd)"
: > "$OUT/results.tsv"

note() { printf '%s\t%s\t%s\n' "$1" "$2" "$3" | tee -a "$OUT/results.tsv"; }

if [ -n "$SOURCE" ]; then
  FETCHED="$(study_private_tmpdir megapowers-install-fetch)"
  trap '[ -z "$FETCHED" ] || rm -rf "$FETCHED"' EXIT
  case "$SOURCE" in
    *://*|git@*) remote="$SOURCE" ;;
    *) remote="https://github.com/$SOURCE.git" ;;
  esac
  if ! timeout 300 git clone --quiet --depth 1 --branch "$REF" "$remote" "$FETCHED/repo" >"$OUT/fetch.log" 2>&1; then
    note source FAIL "fetch exact ref $SOURCE@$REF, see fetch.log"
    exit 1
  fi
  REPO="$FETCHED/repo"
  sha="$(git -C "$REPO" rev-parse HEAD)"
  if ! git -C "$REPO" tag --points-at HEAD | grep -Fxq "$REF"; then
    note source FAIL "fetched HEAD $sha is not exact tag $REF"
    exit 1
  fi
  jq -n --arg source "$SOURCE" --arg ref "$REF" --arg version "$VERSION" --arg sha "$sha" \
    '{source:$source, ref:$ref, version:$version, sha:$sha, mode:"exact-remote-ref"}' > "$OUT/source.json"
  note source PASS "fetched $SOURCE@$REF at $sha"
fi

mapfile -t PLUGINS < <(find "$REPO/plugins" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort)
if [ "${#PLUGINS[@]}" -ne 1 ] || [ "${PLUGINS[0]:-}" != megapowers ]; then
  note source FAIL "checkout must contain exactly the megapowers plugin"
  exit 1
fi

claude_manifest="$REPO/plugins/megapowers/.claude-plugin/plugin.json"
codex_manifest="$REPO/plugins/megapowers/.codex-plugin/plugin.json"
source_claude_version="$(jq -er .version "$claude_manifest")" || { note source FAIL "Claude manifest has no version"; exit 1; }
source_codex_version="$(jq -er .version "$codex_manifest")" || { note source FAIL "Codex manifest has no version"; exit 1; }
if [ "$source_claude_version" != "$source_codex_version" ]; then
  note source FAIL "source manifest versions differ"
  exit 1
fi
EXPECTED_VERSION="${VERSION:-$source_claude_version}"
if [ "$source_claude_version" != "$EXPECTED_VERSION" ]; then
  note source FAIL "source manifest version is not $EXPECTED_VERSION"
  exit 1
fi

verify_installed_bytes() { # <harness> <fresh-home> <install-path>
  local harness="$1" fresh_home="$2" install_path="$3"
  case "$install_path" in
    "$fresh_home"/plugins/cache/*) ;;
    *) note "$harness" FAIL "reported install path is outside fresh home"; return 1 ;;
  esac
  [ -d "$install_path" ] || { note "$harness" FAIL "reported install path is missing"; return 1; }
  jq -e --arg v "$EXPECTED_VERSION" '.name == "megapowers" and .version == $v' \
    "$install_path/.claude-plugin/plugin.json" >/dev/null 2>&1 || {
      note "$harness" FAIL "cached Claude manifest or version differs"; return 1;
    }
  jq -e --arg v "$EXPECTED_VERSION" '.name == "megapowers" and .version == $v' \
    "$install_path/.codex-plugin/plugin.json" >/dev/null 2>&1 || {
      note "$harness" FAIL "cached Codex manifest or version differs"; return 1;
    }
  installed_skill_ok \
    "$REPO/plugins/megapowers/skills/evidence-research/SKILL.md" \
    "$install_path/skills/evidence-research/SKILL.md" || {
    note "$harness" FAIL "cached evidence-research bytes differ"; return 1;
  }
  note "$harness" PASS "registered megapowers $EXPECTED_VERSION with exact cached skill bytes"
}

smoke_claude() (
  local h=claude cfg install_path
  command -v claude >/dev/null || { note "$h" SKIP "claude CLI not installed"; return; }
  cfg="$(study_private_tmpdir megapowers-claude-home)"
  trap 'rm -rf "$cfg"' EXIT
  if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin marketplace add "$REPO" >"$OUT/claude-marketplace.log" 2>&1; then
    note "$h" PASS "marketplace add (local path)"
  else note "$h" FAIL "marketplace add, see claude-marketplace.log"; return; fi
  if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin install megapowers@megapowers >"$OUT/claude-install.log" 2>&1; then
    note "$h" PASS "plugin install megapowers@megapowers"
  else note "$h" FAIL "plugin install, see claude-install.log"; return; fi
  if ! CLAUDE_CONFIG_DIR="$cfg" timeout 120 claude plugin list --json >"$OUT/claude-list.json" 2>"$OUT/claude-list.err"; then
    note "$h" FAIL "plugin list JSON, see claude-list.err"
    return
  fi
  install_path="$(jq -er --arg v "$EXPECTED_VERSION" \
    '.[] | select(.id == "megapowers@megapowers" and .version == $v and .enabled == true) | .installPath' \
    "$OUT/claude-list.json")" || {
      note "$h" FAIL "installed plugin missing from registration JSON"; return;
    }
  verify_installed_bytes "$h" "$cfg" "$install_path"
)

smoke_codex() (
  local h=codex ch install_path
  command -v codex >/dev/null || { note "$h" SKIP "codex CLI not installed"; return; }
  ch="$(study_private_tmpdir megapowers-codex-home)"
  trap 'rm -rf "$ch"' EXIT
  if CODEX_HOME="$ch" timeout 300 codex plugin marketplace add "$REPO" --json \
    >"$OUT/codex-marketplace.json" 2>"$OUT/codex-marketplace.err"; then
    note "$h" PASS "marketplace add (local path)"
  else note "$h" FAIL "marketplace add, see codex-marketplace.err"; return; fi
  if ! CODEX_HOME="$ch" timeout 300 codex plugin add megapowers@megapowers --json \
    >"$OUT/codex-install.json" 2>"$OUT/codex-install.err"; then
    note "$h" FAIL "plugin add, see codex-install.err"
    return
  fi
  install_path="$(jq -er --arg v "$EXPECTED_VERSION" \
    'select(.pluginId == "megapowers@megapowers" and .version == $v) | .installedPath' \
    "$OUT/codex-install.json")" || {
      note "$h" FAIL "plugin add JSON missing installedPath or expected version"; return;
    }
  if ! CODEX_HOME="$ch" timeout 120 codex plugin list --json \
    >"$OUT/codex-list.json" 2>"$OUT/codex-list.err" || \
    ! jq -e --arg v "$EXPECTED_VERSION" \
      '.installed[] | select(.pluginId == "megapowers@megapowers" and .version == $v and .installed == true and .enabled == true)' \
      "$OUT/codex-list.json" >/dev/null; then
    note "$h" FAIL "installed plugin missing from registration JSON"
    return
  fi
  verify_installed_bytes "$h" "$ch" "$install_path"
)

for harness in ${HARNESSES//,/ }; do "smoke_$harness"; done

echo
echo "== install-smoke summary =="
if command -v column >/dev/null 2>&1; then column -t -s $'\t' "$OUT/results.tsv"; else cat "$OUT/results.tsv"; fi
if results_ok "$OUT/results.tsv" "$FAIL_ON_SKIP"; then
  exit 0
fi
echo "install smoke failed: FAIL, strict SKIP, or no PASS result" >&2
exit 1
