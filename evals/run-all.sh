#!/usr/bin/env bash
# run-all.sh — run every scenario and summarize. Deterministic by default:
# artifact scenarios run for real; behavior/trigger scenarios run against the mock
# agent. Exits non-zero if any scenario fails (indeterminate does not fail the run).
#
# Usage: run-all.sh [--agent <name>] [--json <file>] [--paired]
#   --paired: also run each behavior/trigger scenario in CONTROL mode (skill withheld),
#             so the JSON has the paired skill+control rows score.go needs to compute an
#             effect size. Without it, score.go still reports pass rates but no effect
#             size (there is no control data to compare against).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVALS="$ROOT/evals"

agent="mock"; jsonout=""; paired=0
while [ $# -gt 0 ]; do
  case "$1" in
    --agent) agent="$2"; shift 2 ;;
    --json) jsonout="$2"; shift 2 ;;
    --paired) paired=1; shift ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

rows="$(mktemp)"; trap 'rm -f "$rows"' EXIT
pass=0; fail=0; indet=0; failed_ids=""

tally() {  # $1 = a JSON result row
  echo "$1" >> "$rows"
  local v; v="$(printf '%s' "$1" | sed -n 's/.*"verdict":"\([a-z]*\)".*/\1/p')"
  case "$v" in
    pass) pass=$((pass+1)); printf '  \033[32mPASS\033[0m %s\n' "$2" ;;
    indeterminate) indet=$((indet+1)); printf '  \033[33mINDET\033[0m %s\n' "$2" ;;
    *) fail=$((fail+1)); failed_ids="$failed_ids $2"; printf '  \033[31mFAIL\033[0m %s\n' "$2" ;;
  esac
}

for sdir in "$EVALS"/scenarios/*/; do
  id="$(basename "$sdir")"
  [ -f "$sdir/scenario.toml" ] || continue
  tally "$(bash "$EVALS/run.sh" "$id" --agent "$agent")" "$id"
  # paired control run for behavior/trigger scenarios (artifact scenarios have no
  # control notion). The control row feeds score.go's effect-size comparison; it does
  # not affect pass/fail of the run (a control that fails/indet is expected data).
  if [ "$paired" -eq 1 ]; then
    kind="$(sed -n 's/^[[:space:]]*kind[[:space:]]*=[[:space:]]*//p' "$sdir/scenario.toml" | head -1 | tr -d '"')"
    case "$kind" in
      behavior|trigger)
        crow="$(bash "$EVALS/run.sh" "$id" --agent "$agent" --control)"
        echo "$crow" >> "$rows"
        printf '  \033[34mCTRL\033[0m  %s (control)\n' "$id"
        ;;
    esac
  fi
done

# score.go statistics self-test (Fisher exact known values); a stats regression
# fails the run. Guarded so environments without a Go toolchain still run evals.
if command -v go >/dev/null 2>&1; then
  if go run "$EVALS/score.go" --selftest >/dev/null 2>&1; then
    pass=$((pass+1)); printf '  \033[32mPASS\033[0m score.go --selftest\n'
  else
    fail=$((fail+1)); failed_ids="$failed_ids score.go--selftest"; printf '  \033[31mFAIL\033[0m score.go --selftest\n'
  fi
else
  indet=$((indet+1)); printf '  \033[33mINDET\033[0m score.go --selftest (go not installed)\n'
fi

[ -n "$jsonout" ] && cp "$rows" "$jsonout"
echo
echo "== evals: $pass passed, $fail failed, $indet indeterminate (agent: $agent) =="
[ -n "$failed_ids" ] && echo "   failed:$failed_ids"
[ "$fail" -eq 0 ]
