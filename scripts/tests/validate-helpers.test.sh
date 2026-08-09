#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HELPERS="$ROOT/scripts/lib/validate-helpers.sh"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

fail_test() {
  printf 'FAIL %s\n' "$1" >&2
  exit 1
}

# The runner owns scheduling while the caller owns presentation and totals.
# These callbacks mirror validate.sh without testing terminal color codes.
pass=0
fail=0
events="$scratch/events"
ok()  { printf 'PASS %s\n' "$1" >> "$events"; pass=$((pass + 1)); }
bad() { printf 'FAIL %s\n' "$1" >> "$events"; fail=$((fail + 1)); }

parallel_manifest="$scratch/parallel-tests"
printf '%s\n' \
  "$scratch/tests/first.test.sh" \
  "$scratch/tests/second.test.sh" \
  "$scratch/tests/fails.test.sh" \
  "$scratch/tests/blocker.test.sh" \
  "$scratch/tests/race.test.sh" \
  > "$parallel_manifest"
export VALIDATE_PARALLEL_TESTS_FILE="$parallel_manifest"
parallel_audit="$scratch/parallel-tests.audit.tsv"
while IFS= read -r fixture_test; do
  printf '%s\tprivate fixture state with synchronous children\n' "$fixture_test"
done < "$parallel_manifest" > "$parallel_audit"
export VALIDATE_PARALLEL_AUDIT_FILE="$parallel_audit"

# shellcheck source=/dev/null
source "$HELPERS"

# Preserve the historical budget semantics: command substitution removes final
# newlines before byte_len measures the rendered skill body.
byte_len() { printf '%s' "$1" | LC_ALL=C wc -c | tr -d '[:space:]'; }
printf 'body\n\n' > "$scratch/body-with-final-newlines"
[ "$(validate_skill_body_bytes "$scratch/body-with-final-newlines")" -eq 4 ] || fail_test "skill body bytes unexpectedly include final newlines"

pair="$scratch/pair"
mkdir -p "$pair" "$scratch/tests" "$scratch/plugins/megapowers/hooks/tests"
export VALIDATE_HELPER_PAIR="$pair"

for name in first second; do
  other=first
  [ "$name" = first ] && other=second
  script="$scratch/tests/$name.test.sh"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -u' \
    "touch \"\$VALIDATE_HELPER_PAIR/$name.ready\"" \
    'attempt=0' \
    "while [ ! -f \"\$VALIDATE_HELPER_PAIR/$other.ready\" ] && [ \"\$attempt\" -lt 500 ]; do" \
    '  sleep 0.01' \
    '  attempt=$((attempt + 1))' \
    'done' \
    "[ -f \"\$VALIDATE_HELPER_PAIR/$other.ready\" ]" \
    "touch \"\$VALIDATE_HELPER_PAIR/$name.done\"" \
    > "$script"
  chmod +x "$script"
done

failed="$scratch/tests/fails.test.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'touch "$VALIDATE_HELPER_PAIR/failure.started"' \
  'touch "$VALIDATE_HELPER_PAIR/failure.running"' \
  'sleep 0.2' \
  'rm -f "$VALIDATE_HELPER_PAIR/failure.running"' \
  'echo "FAIL intended detail"' \
  'echo "final diagnostic"' \
  'exit 7' \
  > "$failed"
chmod +x "$failed"

# This path is deliberately in the runner's quiet lane. It must start only
# after every parallel worker has exited.
quiet="$scratch/plugins/megapowers/hooks/tests/skill-router.test.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'attempt=0' \
  'while [ ! -f "$VALIDATE_HELPER_PAIR/failure.started" ] && [ "$attempt" -lt 500 ]; do' \
  '  sleep 0.01' \
  '  attempt=$((attempt + 1))' \
  'done' \
  'test -f "$VALIDATE_HELPER_PAIR/failure.started"' \
  'test ! -f "$VALIDATE_HELPER_PAIR/failure.running"' \
  'test -f "$VALIDATE_HELPER_PAIR/first.done"' \
  'test -f "$VALIDATE_HELPER_PAIR/second.done"' \
  > "$quiet"
chmod +x "$quiet"

validate_test_requires_quiet "$quiet" || fail_test "skill-router fixture did not enter the quiet lane"
validate_test_requires_quiet "$ROOT/scripts/tests/validate-helpers.test.sh" || fail_test "scheduler contract test did not enter the quiet lane"
grep -Fxq 'scripts/tests/validate-helpers.test.sh' "$ROOT/scripts/validate-parallel-tests.txt" && fail_test "shipped manifest enrolled the scheduler contract test"
resolver_test="plugins/mega-orchestration/skills/multi-agent-delegation/scripts/tests/delegate-resolve.test.sh"
validate_test_requires_quiet "$ROOT/$resolver_test" || fail_test "nested-concurrency resolver test did not enter the quiet lane"
grep -Fxq "$resolver_test" "$ROOT/scripts/validate-parallel-tests.txt" && fail_test "shipped manifest enrolled the nested-concurrency resolver test"
if validate_test_requires_quiet "$scratch/tests/first.test.sh"; then
  fail_test "manifest-listed fixture unexpectedly entered the quiet lane"
fi
validate_test_requires_quiet "$scratch/tests/unlisted.test.sh" || fail_test "unlisted fixture did not default to the quiet lane"
saved_parallel_tests=("${validate_parallel_tests[@]}")
validate_parallel_tests=("scripts/tests/enforcement.test.sh")
validate_test_requires_quiet "vendor/upstream/scripts/tests/enforcement.test.sh" || fail_test "suffix-copy path inherited another test's enrollment"
validate_parallel_tests=("${saved_parallel_tests[@]}")

# A stale enrollment is a counted validation failure, not a control-flow exit.
# Keep this true even when a caller later enables errexit.
saved_parallel_tests=("${validate_parallel_tests[@]}")
saved_parallel_tests_file="$validate_parallel_tests_file"
validate_parallel_tests=("scripts/tests/deleted.test.sh")
validate_parallel_tests_file="$scratch/stale-parallel-tests"
continued="$scratch/stale-manifest.continued"
set +e
( set -e; validate_parallel_manifest; touch "$continued" )
stale_rc=$?
set -e
[ "$stale_rc" -eq 0 ] && [ -f "$continued" ] || fail_test "stale manifest aborted an errexit caller"
validate_parallel_tests=("${saved_parallel_tests[@]}")
validate_parallel_tests_file="$saved_parallel_tests_file"
: > "$events"

# An existing test cannot enter the parallel lane without reviewable isolation
# evidence in the paired audit file.
unaudited="$scratch/tests/unaudited.test.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' > "$unaudited"
chmod +x "$unaudited"
saved_parallel_tests=("${validate_parallel_tests[@]}")
validate_parallel_tests=("$unaudited")
: > "$events"
pass=0; fail=0
validate_parallel_manifest
[ "$fail" -eq 1 ] || fail_test "unaudited manifest entry was accepted"
grep -q '^FAIL parallel-safe test has no isolation audit:' "$events" || fail_test "missing isolation audit diagnostic is unclear"
validate_parallel_tests=("${saved_parallel_tests[@]}")
: > "$events"
pass=0; fail=0

export VALIDATE_JOBS=2
run_log="$scratch/run.log"
run_test_group "fixture" "fixture tests" < <(
  printf '%s\n' \
    "$scratch/tests/first.test.sh" \
    "$scratch/tests/second.test.sh" \
    "$failed" \
    "$quiet"
) > "$run_log"

[ "$pass" -eq 3 ] || fail_test "parallel group recorded $pass passes, expected 3"
[ "$fail" -eq 1 ] || fail_test "parallel group recorded $fail failures, expected 1"
cat > "$scratch/expected-events" <<EOF
PASS fixture $scratch/tests/first.test.sh
PASS fixture $scratch/tests/second.test.sh
FAIL fixture $failed
PASS fixture $quiet
EOF
cmp -s "$scratch/expected-events" "$events" || fail_test "parallel results were not reported in source order"
grep -q 'FAIL intended detail' "$run_log" || fail_test "failed test detail was not surfaced"
grep -q 'final diagnostic' "$run_log" || fail_test "failed test final line was not surfaced"

# Reject values outside the documented worker range without evaluating them as
# shell integers. An overflowing decimal used to leave index unchanged forever.
overflow_test="$scratch/tests/overflow.test.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' > "$overflow_test"
chmod +x "$overflow_test"
: > "$events"
pass=0; fail=0
export VALIDATE_JOBS=999999999999999999999999999999
run_test_group "fixture" "fixture tests" < <(printf '%s\n' "$overflow_test") >/dev/null
[ "$pass" -eq 1 ] || fail_test "overflow fallback did not run the test serially"
[ "$fail" -eq 1 ] || fail_test "overflowing VALIDATE_JOBS was not rejected"
grep -q '^FAIL VALIDATE_JOBS must be an integer from 1 to 64' "$events" || fail_test "overflow diagnostic is unclear"
run_test_group "fixture" "fixture tests" < <(printf '%s\n' "$overflow_test") >/dev/null
[ "$pass" -eq 2 ] || fail_test "overflow fallback did not remain serial"
[ "$fail" -eq 1 ] || fail_test "invalid VALIDATE_JOBS was reported more than once"

# A clean ShellCheck batch needs one process. On failure, one diagnostic rerun
# per file preserves exact PASS/FAIL reporting.
mkdir -p "$scratch/bin"
shellcheck_log="$scratch/shellcheck.log"
export VALIDATE_HELPER_SHELLCHECK_LOG="$shellcheck_log"
cat > "$scratch/bin/shellcheck" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$VALIDATE_HELPER_SHELLCHECK_LOG"
for arg in "$@"; do
  if [ "${VALIDATE_HELPER_FAIL_SHELLCHECK:-}" = bad ] && [ "$arg" = bad.sh ]; then
    exit 1
  fi
done
if [ "${VALIDATE_HELPER_FAIL_SHELLCHECK:-}" = batch ] && [ "$#" -gt 3 ]; then
  echo "batch-only diagnostic" >&2
  exit 1
fi
exit 0
EOF
chmod +x "$scratch/bin/shellcheck"
PATH="$scratch/bin:$PATH"
export PATH

: > "$events"
: > "$shellcheck_log"
pass=0; fail=0
unset VALIDATE_HELPER_FAIL_SHELLCHECK
run_shellcheck_group < <(printf '%s\n' good.sh bad.sh)
[ "$(wc -l < "$shellcheck_log" | tr -d '[:space:]')" -eq 1 ] || fail_test "clean ShellCheck batch used more than one process"
[ "$pass" -eq 2 ] && [ "$fail" -eq 0 ] || fail_test "clean ShellCheck batch totals are wrong"

: > "$events"
: > "$shellcheck_log"
pass=0; fail=0
export VALIDATE_HELPER_FAIL_SHELLCHECK=bad
run_shellcheck_group < <(printf '%s\n' good.sh bad.sh)
[ "$(wc -l < "$shellcheck_log" | tr -d '[:space:]')" -eq 3 ] || fail_test "failed ShellCheck batch did not rerun each file once"
[ "$pass" -eq 1 ] && [ "$fail" -eq 1 ] || fail_test "failed ShellCheck batch totals are wrong"

: > "$events"
: > "$shellcheck_log"
pass=0; fail=0
export VALIDATE_HELPER_FAIL_SHELLCHECK=batch
shellcheck_run_log="$scratch/shellcheck-run.log"
run_shellcheck_group < <(printf '%s\n' good.sh bad.sh) > "$shellcheck_run_log"
[ "$(wc -l < "$shellcheck_log" | tr -d '[:space:]')" -eq 3 ] || fail_test "batch-only ShellCheck failure did not rerun each file once"
[ "$pass" -eq 2 ] && [ "$fail" -eq 1 ] || fail_test "batch-only ShellCheck failure was not preserved"
grep -q '^FAIL shellcheck batch failed but no single file reproduced it$' "$events" || fail_test "batch-only ShellCheck failure diagnostic is unclear"
grep -q 'batch-only diagnostic' "$shellcheck_run_log" || fail_test "batch-only ShellCheck output was discarded"

: > "$events"
pass=0; fail=0
run_shellcheck_group </dev/null
[ "$pass" -eq 0 ] && [ "$fail" -eq 1 ] || fail_test "empty ShellCheck discovery did not fail closed"
grep -q '^FAIL no shell scripts found for ShellCheck$' "$events" || fail_test "empty ShellCheck diagnostic is unclear"

# Interrupting a group must terminate its active test and remove its runner
# scratch directory rather than orphaning both.
blocker="$scratch/tests/blocker.test.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'trap "exit 0" INT TERM HUP' \
  'echo "$$" > "$VALIDATE_HELPER_PAIR/blocker.pid"' \
  'while :; do sleep 1; done' \
  > "$blocker"
chmod +x "$blocker"
runner_tmp="$scratch/runner-tmp"
mkdir -p "$runner_tmp"
export TMPDIR="$runner_tmp"
export VALIDATE_JOBS=2
( run_test_group "fixture" "fixture tests" < <(printf '%s\n' "$blocker") >/dev/null ) &
runner_pid=$!
attempt=0
while [ ! -s "$pair/blocker.pid" ] && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
[ -s "$pair/blocker.pid" ] || fail_test "interrupt fixture never started"
blocker_pid="$(cat "$pair/blocker.pid")"
kill -TERM "$runner_pid"
wait "$runner_pid" 2>/dev/null || true
attempt=0
while kill -0 "$blocker_pid" 2>/dev/null && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
if kill -0 "$blocker_pid" 2>/dev/null; then
  kill -TERM "$blocker_pid" 2>/dev/null || true
  fail_test "interrupted group orphaned its active test"
fi
if find "$runner_tmp" -mindepth 1 -print -quit | grep -q .; then
  fail_test "interrupted group leaked its runner scratch directory"
fi

# Exercise the real Ctrl-C path from a synchronous runner. The parent trap
# converts SIGINT into TERM cleanup for the asynchronous worker and its test.
sigint_blocker="$scratch/tests/sigint-blocker.test.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'trap "exit 0" TERM' \
  'echo "$$" > "$VALIDATE_HELPER_PAIR/sigint-blocker.pid"' \
  'while :; do sleep 1; done' \
  > "$sigint_blocker"
chmod +x "$sigint_blocker"
printf '%s\n' "$sigint_blocker" >> "$parallel_manifest"
printf '%s\tprivate SIGINT fixture state with synchronous cleanup\n' "$sigint_blocker" >> "$parallel_audit"
sigint_runner_script="$scratch/sigint-runner.sh"
cat > "$sigint_runner_script" <<'EOF'
#!/usr/bin/env bash
set -u
ROOT="$VALIDATE_HELPER_ROOT"
ok() { :; }
bad() { :; }
# shellcheck source=/dev/null
source "$VALIDATE_HELPER_HELPERS"
(
  attempt=0
  while [ ! -s "$VALIDATE_HELPER_PAIR/sigint-blocker.pid" ] && [ "$attempt" -lt 500 ]; do
    sleep 0.01
    attempt=$((attempt + 1))
  done
  kill -INT "$$"
) &
run_test_group "fixture" "fixture tests" < <(printf '%s\n' "$VALIDATE_HELPER_SIGINT_BLOCKER") >/dev/null
EOF
chmod +x "$sigint_runner_script"
export VALIDATE_HELPER_ROOT="$ROOT"
export VALIDATE_HELPER_HELPERS="$HELPERS"
export VALIDATE_HELPER_SIGINT_BLOCKER="$sigint_blocker"
rm -f "$pair/sigint-blocker.pid"
set +e
TMPDIR="$runner_tmp" bash "$sigint_runner_script"
sigint_rc=$?
set -e
[ "$sigint_rc" -eq 130 ] || fail_test "SIGINT runner exited $sigint_rc instead of 130"
[ -s "$pair/sigint-blocker.pid" ] || fail_test "SIGINT fixture never started"
sigint_blocker_pid="$(cat "$pair/sigint-blocker.pid")"
attempt=0
while kill -0 "$sigint_blocker_pid" 2>/dev/null && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
if kill -0 "$sigint_blocker_pid" 2>/dev/null; then
  kill -TERM "$sigint_blocker_pid" 2>/dev/null || true
  fail_test "SIGINT orphaned its active test"
fi
if find "$runner_tmp" -mindepth 1 -print -quit | grep -q .; then
  fail_test "SIGINT leaked its runner scratch directory"
fi

# TERM after a worker forks but before its PID is registered must still reap
# the newly launched worker and its active test.
race="$scratch/tests/race.test.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'trap "exit 0" TERM' \
  'echo "$$" > "$VALIDATE_HELPER_PAIR/race.pid"' \
  'while :; do sleep 1; done' \
  > "$race"
chmod +x "$race"
export VALIDATE_HELPER_ROOT="$ROOT"
export VALIDATE_HELPER_HELPERS="$HELPERS"
export VALIDATE_HELPER_RACE="$race"
race_runner_script="$scratch/race-runner.sh"
cat > "$race_runner_script" <<'EOF'
#!/usr/bin/env bash
set -uT
ROOT="$VALIDATE_HELPER_ROOT"
ok() { :; }
bad() { :; }
# shellcheck source=/dev/null
source "$VALIDATE_HELPER_HELPERS"
race_debug_abort() {
  case "$BASH_COMMAND" in
    'pids[$worker]=$!')
      trap - DEBUG
      attempt=0
      while [ ! -s "$VALIDATE_HELPER_PAIR/race.pid" ] && [ "$attempt" -lt 500 ]; do
        sleep 0.01
        attempt=$((attempt + 1))
      done
      kill -TERM "$$"
      ;;
  esac
}
trap race_debug_abort DEBUG
run_test_group "fixture" "fixture tests" < <(printf '%s\n' "$VALIDATE_HELPER_RACE") >/dev/null
EOF
chmod +x "$race_runner_script"
rm -f "$pair/race.pid"
TMPDIR="$runner_tmp" bash "$race_runner_script" &
race_runner=$!
attempt=0
while [ ! -s "$pair/race.pid" ] && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
[ -s "$pair/race.pid" ] || fail_test "PID-registration race fixture never started"
race_pid="$(cat "$pair/race.pid")"
wait "$race_runner" 2>/dev/null || true
attempt=0
while kill -0 "$race_pid" 2>/dev/null && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
if kill -0 "$race_pid" 2>/dev/null; then
  kill -TERM "$race_pid" 2>/dev/null || true
  fail_test "signal-before-PID-registration orphaned its active test"
fi

echo "validate helper contracts: PASS"
