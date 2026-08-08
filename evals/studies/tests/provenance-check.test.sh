#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
check="$ROOT/evals/studies/provenance-check.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

make_run() {
  local name="$1" sha="$2" dir
  dir="$tmp/probe/model/skill/$name"
  mkdir -p "$dir"
  printf '%s\n' '{"probe":"probe","model":"model","mode":"skill"}' > "$dir/meta.json"
  jq -n --arg sha "$sha" '{schema_version:1,repo_sha:"repo",source_sha256:"source",agent_cli:"codex",cli_version:"v1",model_identity:"model",effort:"high",prompt_sha256:"prompt",harness_sha256:$sha,skill_hashes:[{skill:"test",sha256:"skill"}]}' > "$dir/provenance.json"
}

make_run run-01 same
make_run run-02 same
"$check" "$tmp"
for provenance in "$tmp"/probe/model/skill/run-*/provenance.json; do
  jq '.cli_version = null | .effort = null' "$provenance" > "$tmp/optional-null.json"
  mv "$tmp/optional-null.json" "$provenance"
done
if ! "$check" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL null optional provenance fields rejected'
  exit 1
fi
make_run run-01 same
make_run run-02 same
mkdir -p "$tmp/probe/model/skill/run-03"
printf '%s\n' '{"probe":"probe","model":"model","mode":"skill"}' > "$tmp/probe/model/skill/run-03/meta.json"
if "$check" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL meta-only scored run accepted'
  exit 1
fi
rm -rf "$tmp/probe/model/skill/run-03"
make_run run-02 different
if "$check" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL mixed deterministic provenance accepted'
  exit 1
fi

make_run run-02 same
jq 'del(.repo_sha)' "$tmp/probe/model/skill/run-02/provenance.json" > "$tmp/missing.json"
mv "$tmp/missing.json" "$tmp/probe/model/skill/run-02/provenance.json"
if "$check" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL missing provenance field accepted'
  exit 1
fi

make_run run-02 changed-harness
for oracle in process-behavior autonomy-run gauntlet trigger-recall; do
  if "$ROOT/evals/studies/$oracle/oracle.sh" "$tmp" >/dev/null 2>&1; then
    echo "FAIL $oracle oracle scored mixed provenance"
    exit 1
  fi
done
echo 'provenance compatibility contract: ok'
