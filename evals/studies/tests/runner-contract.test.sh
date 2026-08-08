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
nested_results="$ROOT/studies/process-behavior/.provenance-contract.$$"
overlap_results="$scratch/fixtures-link"
trap 'rm -rf "$scratch" "$nested_results" "$overlap_results"' EXIT
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

abs_out="$scratch/path-abs"
rel_out="$scratch/path-rel"
STUDY_ARGS_FILE="$scratch/path.args" PATH="$scratch/bin:$PATH" \
  bash "$ROOT/studies/process-behavior/run-study.sh" --job \
  "verify-before-done|gpt-path-test|control|1|$abs_out|2|10"
(
  cd "$ROOT/studies/process-behavior" || exit 1
  STUDY_ARGS_FILE="$scratch/path.args" PATH="$scratch/bin:$PATH" \
    bash ./run-study.sh --job \
    "verify-before-done|gpt-path-test|control|1|$rel_out|2|10"
)
check "runner spelling does not change provenance" jq -se \
  '.[0].harness_sha256 == .[1].harness_sha256' \
  "$abs_out/verify-before-done/gpt-path-test/control/run-01/provenance.json" \
  "$rel_out/verify-before-done/gpt-path-test/control/run-01/provenance.json"

mkdir -p "$scratch/provenance-run"
study_record_provenance "$scratch/provenance-run" codex gpt-test "$scratch/prompt.txt" "$ROOT/studies/process-behavior"
check "study provenance records deterministic inputs" jq -e '.schema_version == 1 and .repo_sha != "unknown" and .source_sha256 != "" and .harness_sha256 != "" and .prompt_sha256 != "" and (.skill_hashes | length > 0)' "$scratch/provenance-run/provenance.json"

mkdir -p "$nested_results/run-01" "$nested_results/run-02"
study_record_provenance "$nested_results/run-01" codex gpt-test "$scratch/prompt.txt" "$ROOT/studies/process-behavior"
study_record_provenance "$nested_results/run-02" codex gpt-test "$scratch/prompt.txt" "$ROOT/studies/process-behavior"
check "nested sequential runs keep stable provenance" jq -se \
  '.[0].source_sha256 == .[1].source_sha256 and .[0].harness_sha256 == .[1].harness_sha256' \
  "$nested_results/run-01/provenance.json" "$nested_results/run-02/provenance.json"
ln -s "$ROOT/studies/process-behavior/fixtures" "$overlap_results"
if study_record_provenance "$overlap_results" codex gpt-test "$scratch/prompt.txt" "$ROOT/studies/process-behavior" >/dev/null 2>&1; then
  echo "FAIL provenance accepted results inside declared inputs"
  failures=$((failures + 1))
else
  echo "ok   provenance rejects results inside declared inputs"
fi

missing_study="$scratch/missing-study"
missing_run="$scratch/missing-run"
mkdir -p "$missing_study" "$missing_run"
if study_record_provenance "$missing_run" codex gpt-test "$scratch/prompt.txt" "$missing_study" >/dev/null 2>&1; then
  echo "FAIL provenance accepted missing declared inputs"
  failures=$((failures + 1))
else
  echo "ok   provenance rejects missing declared inputs"
fi

hash_tree="$scratch/hash-tree"
mkdir -p "$hash_tree"
printf 'same bytes\n' > "$hash_tree/before.txt"
hash_before="$(study_tree_sha256 "$hash_tree" "$hash_tree")"
mv "$hash_tree/before.txt" "$hash_tree/after.txt"
hash_after="$(study_tree_sha256 "$hash_tree" "$hash_tree")"
check "tree hash binds paths as well as bytes" test "$hash_before" != "$hash_after"

mkdir -p "$scratch/fallback-bin"
cat > "$scratch/fallback-bin/shasum" <<EOF
#!/usr/bin/env bash
touch "$scratch/shasum-used"
exec /usr/bin/shasum "\$@"
EOF
chmod +x "$scratch/fallback-bin/shasum"
STUDY_HASH_FORCE_SHASUM=1 PATH="$scratch/fallback-bin:$PATH" study_hash_file "$scratch/prompt.txt" >/dev/null
check "study hashing can force shasum fallback" test -f "$scratch/shasum-used"

for script in \
  "$ROOT/studies/process-behavior/run-study.sh" \
  "$ROOT/studies/autonomy-run/run-autonomy.sh" \
  "$ROOT/studies/gauntlet/run-gauntlet.sh" \
  "$ROOT/studies/trigger-recall/run-recall.sh"
do
  check "$(basename "$script") records harness_error" grep -q 'run_status.*harness_error' "$script"
  check "$(basename "$script") returns actor status" grep -q 'return "\$rc"' "$script"
done

check "gauntlet oracle mutation selftest runs" bash "$ROOT/studies/gauntlet/oracle.sh" --selftest

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
