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
. "$ROOT/studies/lib.sh"

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

# Model-routing calibration compares effort as well as model. Exercise the
# shared runner through fake CLIs so the assertion proves the actual argv, not
# only that a variable name appears in lib.sh.
mkdir -p "$scratch/bin" "$scratch/repo" "$scratch/codex-run" "$scratch/claude-run"
printf 'calibrate this model\n' > "$scratch/prompt.txt"
cat > "$scratch/bin/codex" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$STUDY_ARGS_FILE"
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
EOF
cat > "$scratch/bin/claude" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$STUDY_ARGS_FILE"
printf '%s\n' '{"type":"result","result":"ok"}'
EOF
chmod +x "$scratch/bin/codex" "$scratch/bin/claude"

STUDY_ARGS_FILE="$scratch/codex.args" STUDY_EFFORT=max PATH="$scratch/bin:$PATH" \
  study_exec codex gpt-test "$scratch/repo" "$scratch/prompt.txt" "$scratch/codex-run" 10 2
check "study_exec passes Codex max effort" grep -qF 'model_reasoning_effort="max"' "$scratch/codex.args"

STUDY_ARGS_FILE="$scratch/claude.args" STUDY_EFFORT=high PATH="$scratch/bin:$PATH" \
  study_exec claude claude-test "$scratch/repo" "$scratch/prompt.txt" "$scratch/claude-run" 10 2
check "study_exec passes Claude high effort" grep -qF -- '--effort high' "$scratch/claude.args"

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
