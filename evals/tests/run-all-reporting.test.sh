#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
json="$tmp/results.jsonl"
log="$tmp/run.log"

set +e
bash "$ROOT/evals/run-all.sh" --json "$json" > "$log"
run_rc=$?
set -e
[ "$run_rc" -eq 0 ] || {
  echo "FAIL deterministic regression suite exited $run_rc" >&2
  cat "$log" >&2
  exit 1
}
summary="$(sed -n 's/^== evals: \([0-9][0-9]*\) passed, \([0-9][0-9]*\) failed, \([0-9][0-9]*\) indeterminate, \([0-9][0-9]*\) harness errors.*/\1 \2 \3 \4/p' "$log")"
read -r pass fail indeterminate harness_errors <<< "$summary"
expected=$((pass + fail + indeterminate + harness_errors))
actual="$(wc -l < "$json" | tr -d '[:space:]')"
[ "$actual" -eq "$expected" ] || {
  echo "FAIL JSON has $actual rows but console counted $expected checks" >&2
  exit 1
}
jq -se '
  all(.[]; .schema_version == "1" and .evidence_class == "regression" and .arm == "regression") and
  ([.[] | select(.study == "deterministic-regression/selftest")] | length) == 5 and
  ([.[] | select(.study == "deterministic-regression/selftest") | .case_id] | unique | length) == 5
' "$json" >/dev/null || {
  echo 'FAIL JSON omits or duplicates local selftest rows' >&2
  exit 1
}

go run "$ROOT/evals/score.go" --strict "$json" > "$tmp/scorecard.md"
grep -q '^## Deterministic regressions$' "$tmp/scorecard.md"

# A failing run must still leave parseable result rows at the requested path.
repo="$tmp/failing-repo"
mkdir -p "$repo/evals"
cp "$ROOT/evals/run-all.sh" "$ROOT/evals/score.go" "$repo/evals/"
cp -R "$ROOT/evals/studies" "$repo/evals/studies"
printf '#!/usr/bin/env bash\nexit 9\n' > "$repo/evals/studies/install-smoke/run-smoke.sh"
chmod +x "$repo/evals/run-all.sh" "$repo/evals/studies/install-smoke/run-smoke.sh"
set +e
bash "$repo/evals/run-all.sh" --json "$tmp/failure.jsonl" > "$tmp/failure.log"
failure_rc=$?
set -e
[ "$failure_rc" -ne 0 ]
jq -se 'length > 0 and any(.[]; .verdict == "harness_error" or .verdict == "fail")' "$tmp/failure.jsonl" >/dev/null

echo 'run-all reporting contract: ok'
