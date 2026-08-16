#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

hash='sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
plugin_hash='sha256:1111111111111111111111111111111111111111111111111111111111111111'
treatment_inventory_hash='sha256:88e40f75e81a425ed2408db9ee50732561aef45d592f1dc5dc6d7a977260903e'
control_inventory_hash='sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570'
repo=''
cert=''

new_repo() {
  local name="$1"
  repo="$tmp/$name-repo"
  cert="$tmp/$name-cert"
  mkdir -p \
    "$repo/scripts" "$repo/evals" \
    "$repo/plugins/megapowers/.claude-plugin" \
    "$repo/plugins/megapowers/.codex-plugin" \
    "$repo/docs" "$repo/evals/studies/install-smoke" "$repo/evals/studies/installed-ab"
  cp "$ROOT/scripts/release.sh" "$repo/scripts/release.sh"
  cp "$ROOT/evals/score.go" "$repo/evals/score.go"
  cp "$ROOT/evals/studies/installed-ab/gates.json" "$repo/evals/studies/installed-ab/gates.json"
  printf '%s\n' '{"schema_version":"1","cases":[{"id":"case-1","kind":"code_quality"}]}' \
    > "$repo/evals/studies/installed-ab/cases.json"
  printf '%s\n' '# Changelog' '' '## 1.2.3 - test' > "$repo/CHANGELOG.md"
  printf '%s\n' '{"name":"megapowers","version":"0.0.0"}' > "$repo/plugins/megapowers/.claude-plugin/plugin.json"
  printf '%s\n' '{"name":"megapowers","version":"0.0.0"}' > "$repo/plugins/megapowers/.codex-plugin/plugin.json"
  printf '%s\n' 'git clone --branch v0.0.0 repo megapowers-v0.0.0' > "$repo/docs/install.md"
  printf '%s\n' 'tag=v0.0.0' > "$repo/evals/studies/install-smoke/README.md"
  printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "deterministic\\n" >> gate.log' \
    '[[ ${1:-} == --json && -n ${2:-} ]]' \
    'cp "$RELEASE_TEST_RESULTS" "$2"' \
    > "$repo/evals/run-all.sh"
  printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "fresh\\n" >> gate.log' \
    > "$repo/scripts/check-freshness.sh"
  chmod +x "$repo/evals/run-all.sh" "$repo/scripts/check-freshness.sh" "$repo/scripts/release.sh"
  : > "$repo/gate.log"
  git -C "$repo" init -q
  git -C "$repo" config user.name fixture
  git -C "$repo" config user.email fixture@example.invalid
  git -C "$repo" add .
  git -C "$repo" -c commit.gpgsign=false commit -qm fixture
}

write_cert() {
  local harness="$1" evidence="${2:-credentialed-behavioral-run}" revision="${3:-}" pairs="${4:-10}"
  [[ -n $revision ]] || revision="$(git -C "$repo" rev-parse HEAD)"
  local publish="$cert/$harness/publish"
  mkdir -p "$publish"
  jq -n --arg harness "$harness" --arg evidence "$evidence" --arg hash "$hash" --arg plugin_hash "$plugin_hash" '{
    schema_version:"1",study:"installed-plugin-ab",evidence:$evidence,
    harness:$harness,model:"test-model",effort:"high",
    broker_hash:$plugin_hash,treatment_plugin_hash:$plugin_hash,
    empty_control_plugin_hash:$hash,
    publishability:{publishable:true,minimum_paired_runs:10,minimum_absolute_lift:0.1,confidence_level:0.95,cases:[],reasons:[]},
    arms:[]
  }' > "$publish/manifest.json"
  local pair arm arm_plugin_hash verdict rc task_success
  for pair in $(seq 1 "$pairs"); do
    for arm in treatment control; do
      arm_plugin_hash="$plugin_hash"
      verdict=pass
      rc=0
      task_success=1
      if [[ $arm == control ]]; then
        arm_plugin_hash="$hash"
        verdict=fail
        rc=1
        task_success=0
      fi
      jq -cn --arg harness "$harness" --arg arm "$arm" --arg revision "$revision" \
        --arg hash "$hash" --arg plugin_hash "$arm_plugin_hash" \
        --arg verdict "$verdict" --argjson rc "$rc" --argjson task_success "$task_success" \
        --arg run_id "run-$pair-$arm" --arg block_id "block-$pair" '{
          schema_version:"1",study:"installed-plugin-ab",evidence_class:"behavioral",
          case_id:"case-1",run_id:$run_id,block_id:$block_id,arm:$arm,
          harness:{name:$harness,cli_version:"1.0.0",model:"test-model",effort:"high"},
          source:{repository:"megapowers",revision:$revision},prompt_hash:$hash,
          fixture_hash:$hash,plugin_hash:$plugin_hash,status:"completed",rc:$rc,
          duration_ms:1,verdict:$verdict,metrics:{task_success:$task_success},artifacts:{},
          environment:{os:"linux",arch:"amd64",sandbox:"workspace-write",locale:"C"},
          timestamp:"2026-08-16T00:00:00Z"
      }' >> "$publish/results.jsonl"
    done
  done
  jq -s --arg evidence "$evidence" \
    --arg treatment_inventory_hash "$treatment_inventory_hash" \
    --arg control_inventory_hash "$control_inventory_hash" '
      map({case_id,block_id,arm,prompt_hash,fixture_hash,plugin_hash,
        plugin_inventory:(if .arm == "treatment" then ["megapowers"] else [] end),
        inventory_hash:(if .arm == "treatment" then $treatment_inventory_hash else $control_inventory_hash end),
        evidence:$evidence})
    ' "$publish/results.jsonl" > "$tmp/arms.json"
  jq --slurpfile arms "$tmp/arms.json" '.arms = $arms[0]' \
    "$publish/manifest.json" > "$tmp/manifest.json"
  mv "$tmp/manifest.json" "$publish/manifest.json"
}

write_both() {
  write_cert claude "${1:-credentialed-behavioral-run}" "${2:-}" "${3:-10}"
  write_cert codex "${1:-credentialed-behavioral-run}" "${2:-}" "${3:-10}"
}

run_release() {
  set +e
  RELEASE_TEST_RESULTS="$cert/claude/publish/results.jsonl" \
    bash "$repo/scripts/release.sh" 1.2.3 --certificate-root "$cert" \
    >"$tmp/release.out" 2>&1
  local rc=$?
  set -e
  return "$rc"
}

unchanged_version() {
  [[ $(jq -r .version "$repo/plugins/megapowers/.claude-plugin/plugin.json") == 0.0.0 ]]
}

new_repo missing
if run_release; then echo 'FAIL missing certificate root was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL missing certificate mutated manifests'; exit 1; }
[[ ! -s $repo/gate.log ]] || { echo 'FAIL gates ran before certificate acceptance'; exit 1; }

new_repo selftest
write_both selftest-only-not-certification
if run_release; then echo 'FAIL selftest evidence was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL selftest certificate mutated manifests'; exit 1; }

new_repo revision
write_both credentialed-behavioral-run deadbeef
if run_release; then echo 'FAIL mismatched source revision was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL mismatched revision mutated manifests'; exit 1; }

new_repo broker
write_both
jq 'del(.broker_hash)' "$cert/claude/publish/manifest.json" > "$tmp/manifest.json"
mv "$tmp/manifest.json" "$cert/claude/publish/manifest.json"
if run_release; then echo 'FAIL missing broker attestation was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL missing broker attestation mutated manifests'; exit 1; }

new_repo publishability
write_both
jq '.publishability.publishable = false | .publishability.reasons = ["weak lift"]' \
  "$cert/codex/publish/manifest.json" > "$tmp/manifest.json"
mv "$tmp/manifest.json" "$cert/codex/publish/manifest.json"
if run_release; then echo 'FAIL non-publishable behavioral evidence was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL non-publishable evidence mutated manifests'; exit 1; }

new_repo extra
write_both
printf '%s\n' secret > "$cert/codex/publish/transcript.txt"
if run_release; then echo 'FAIL extra publish artifact was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL extra artifact mutated manifests'; exit 1; }

new_repo duplicate
write_both
head -n 1 "$cert/claude/publish/results.jsonl" >> "$cert/claude/publish/results.jsonl"
if run_release; then echo 'FAIL strict scorer failure was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL scorer failure mutated manifests'; exit 1; }

new_repo insufficient
write_both credentialed-behavioral-run '' 1
if run_release; then echo 'FAIL self-declared publishability bypassed the minimum paired-run gate'; exit 1; fi
unchanged_version || { echo 'FAIL insufficient behavioral evidence mutated manifests'; exit 1; }

new_repo unbound-inventory
write_both
jq '.arms = .arms[:-1]' "$cert/claude/publish/manifest.json" > "$tmp/manifest.json"
mv "$tmp/manifest.json" "$cert/claude/publish/manifest.json"
if run_release; then echo 'FAIL results without matching inventory attestations were accepted'; exit 1; fi
unchanged_version || { echo 'FAIL unbound inventory evidence mutated manifests'; exit 1; }

new_repo forged-report-only
write_both
jq -c '.metrics.report_only = 1' "$cert/claude/publish/results.jsonl" > "$tmp/results.jsonl"
mv "$tmp/results.jsonl" "$cert/claude/publish/results.jsonl"
if run_release; then echo 'FAIL a release-gated case was allowed to self-declare report-only'; exit 1; fi
unchanged_version || { echo 'FAIL forged report-only evidence mutated manifests'; exit 1; }

new_repo dirty
write_both
printf '%s\n' dirty > "$repo/untracked.txt"
if run_release; then echo 'FAIL dirty candidate was accepted'; exit 1; fi
unchanged_version || { echo 'FAIL dirty candidate mutated manifests'; exit 1; }

new_repo valid
write_both
run_release || { cat "$tmp/release.out"; echo 'FAIL valid certificate was refused'; exit 1; }
[[ $(jq -r .version "$repo/plugins/megapowers/.claude-plugin/plugin.json") == 1.2.3 ]] || {
  echo 'FAIL Claude manifest was not stamped'; exit 1;
}
[[ $(jq -r .version "$repo/plugins/megapowers/.codex-plugin/plugin.json") == 1.2.3 ]] || {
  echo 'FAIL Codex manifest was not stamped'; exit 1;
}
[[ $(cat "$repo/gate.log") == $'deterministic\nfresh' ]] || {
  echo 'FAIL deterministic/freshness gate order changed'; cat "$repo/gate.log"; exit 1;
}

printf 'release preflight contract: ok\n'
