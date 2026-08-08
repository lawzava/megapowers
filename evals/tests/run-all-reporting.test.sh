#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
json="$tmp/results.jsonl"
log="$tmp/run.log"

bash "$ROOT/evals/run-all.sh" --json "$json" > "$log"
summary="$(sed -n 's/^== evals: \([0-9][0-9]*\) passed, \([0-9][0-9]*\) failed, \([0-9][0-9]*\) indeterminate, \([0-9][0-9]*\) harness errors.*/\1 \2 \3 \4/p' "$log")"
read -r pass fail indeterminate harness_errors <<< "$summary"
expected=$((pass + fail + indeterminate + harness_errors))
actual="$(wc -l < "$json" | tr -d '[:space:]')"
[ "$actual" -eq "$expected" ] || {
  echo "FAIL JSON has $actual rows but console counted $expected checks" >&2
  exit 1
}
jq -se '
  ([.[] | select(.kind == "selftest")] | length) == 4 and
  ([.[] | select(.kind == "selftest") | .scenario] | unique | length) == 4
' "$json" >/dev/null || {
  echo 'FAIL JSON omits or duplicates local selftest rows' >&2
  exit 1
}

echo 'run-all reporting contract: ok'
