#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
failures=0

check() {
  local description="$1"
  shift
  if "$@"; then
    echo "ok   $description"
  else
    echo "FAIL $description"
    failures=$((failures + 1))
  fi
}

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

runner="$scratch/fanout-runner"
cp "$ROOT/studies/lib.sh" "$scratch/lib.sh"
cat > "$runner" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
. "$(dirname "$0")/lib.sh"
if [ "${1:-}" = "--job" ]; then
  [ "$2" != fail ]
  exit $?
fi
printf 'pass\nfail\n' | study_fanout 2 "$PWD"
EOF
chmod +x "$runner"

check "study_fanout propagates a failed worker" bash -c '"$1" >/dev/null 2>&1; [ $? -ne 0 ]' _ "$runner"

for script in \
  "$ROOT/studies/process-behavior/run-study.sh" \
  "$ROOT/studies/autonomy-run/run-autonomy.sh" \
  "$ROOT/studies/gauntlet/run-gauntlet.sh" \
  "$ROOT/studies/trigger-recall/run-recall.sh"
do
  check "$(basename "$script") records harness_error" grep -q 'run_status.*harness_error' "$script"
  check "$(basename "$script") returns actor status" grep -q 'return "\$rc"' "$script"
done

for oracle in \
  "$ROOT/studies/process-behavior/oracle.sh" \
  "$ROOT/studies/autonomy-run/oracle.sh" \
  "$ROOT/studies/gauntlet/oracle.sh" \
  "$ROOT/studies/trigger-recall/oracle.sh"
do
  check "$(basename "$(dirname "$oracle")") reports harness errors" grep -q 'HARNESS_ERROR' "$oracle"
done

if [ "$failures" -ne 0 ]; then
  echo "$failures runner contract test(s) failed"
  exit 1
fi
echo "runner contract tests: PASS"
