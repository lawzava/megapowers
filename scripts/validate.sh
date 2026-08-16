#!/usr/bin/env bash
# Bounded deterministic validation for the sole shipped plugin and its two harnesses.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2

pass=0
fail=0

if command -v timeout >/dev/null 2>&1; then
  timeout_cmd=timeout
elif command -v gtimeout >/dev/null 2>&1; then
  timeout_cmd=gtimeout
else
  printf 'validate: GNU timeout or gtimeout is required\n' >&2
  exit 2
fi

scratch_root="${TMPDIR:-/tmp}"
log_dir="$(mktemp -d "$scratch_root/megapowers-validate.XXXXXX")" || exit 2
trap 'rm -rf -- "$log_dir"' EXIT

run_check() {
  local name="$1"
  local limit="$2"
  shift 2
  local log="$log_dir/$((pass + fail)).log"
  local status=0

  "$timeout_cmd" --foreground "$limit" "$@" >"$log" 2>&1 || status=$?
  if (( status == 0 )); then
    printf '  PASS %s\n' "$name"
    pass=$((pass + 1))
    return 0
  fi

  printf '  FAIL %s' "$name" >&2
  if (( status == 124 || status == 137 )); then
    printf ' (timed out after %s)' "$limit" >&2
  else
    printf ' (exit %d)' "$status" >&2
  fi
  printf '\n' >&2
  sed -n '1,200p' "$log" >&2
  fail=$((fail + 1))
  return 0
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'validate: required command not found: %s\n' "$1" >&2
    exit 2
  }
}

require_command go
require_command jq

run_check "marketplace and manifest contract" 20s bash -c '
  set -euo pipefail
  claude_marketplace=.claude-plugin/marketplace.json
  codex_marketplace=.agents/plugins/marketplace.json
  claude_manifest=plugins/megapowers/.claude-plugin/plugin.json
  codex_manifest=plugins/megapowers/.codex-plugin/plugin.json

  jq -e '\''
    .name == "megapowers" and
    (.plugins | length) == 1 and
    .plugins[0].name == "megapowers" and
    .plugins[0].source == "./plugins/megapowers"
  '\'' "$claude_marketplace" >/dev/null
  jq -e '\''
    .name == "megapowers" and
    (.plugins | length) == 1 and
    .plugins[0].name == "megapowers" and
    .plugins[0].source.source == "local" and
    .plugins[0].source.path == "./plugins/megapowers"
  '\'' "$codex_marketplace" >/dev/null

  claude_name="$(jq -er .name "$claude_manifest")"
  codex_name="$(jq -er .name "$codex_manifest")"
  claude_version="$(jq -er .version "$claude_manifest")"
  codex_version="$(jq -er .version "$codex_manifest")"
  test "$claude_name" = megapowers
  test "$codex_name" = megapowers
  test "$claude_version" = "$codex_version"
  grep -qF "## $claude_version - " CHANGELOG.md
  test "$(git ls-files "plugins/*" | cut -d/ -f2 | sort -u)" = megapowers
'

run_check "shell syntax" 30s bash -c '
  set -euo pipefail
  mapfile -d "" files < <(find scripts evals plugins/megapowers/hooks -type f \
    \( -name "*.sh" -o -name "*.bash" \) -print0)
  ((${#files[@]} > 0))
  bash -n "${files[@]}"
'

if command -v shellcheck >/dev/null 2>&1; then
  run_check "shell lint" 60s bash -c '
    set -euo pipefail
    mapfile -d "" files < <(find scripts evals plugins/megapowers/hooks -type f \
      \( -name "*.sh" -o -name "*.bash" \) -print0)
    shellcheck --severity=warning "${files[@]}"
  '
fi

run_check "Go formatting" 30s bash -c '
  set -euo pipefail
  mapfile -d "" files < <(find scripts evals plugins/megapowers -type f -name "*.go" -print0)
  ((${#files[@]} > 0))
  output="$(gofmt -l "${files[@]}")"
  test -z "$output" || { printf "%s\n" "$output"; exit 1; }
'

run_check "full-tree security lint" 60s scripts/security-lint.sh
run_check "freshness metadata" 30s scripts/check-freshness.sh --max-age-days 36500

script_tests=(
  scripts/tests/ci-contract.test.sh
  scripts/tests/docs-contract.test.sh
  scripts/tests/freshness.test.sh
  scripts/tests/hook-contract.test.sh
  scripts/tests/megapowers-review.test.sh
  scripts/tests/native-first-contract.test.sh
  scripts/tests/release-preflight.test.sh
  scripts/tests/security-lint.test.sh
  scripts/tests/skill-contracts.test.sh
  scripts/tests/validation-contract.test.sh
)

hook_tests=(
  plugins/megapowers/hooks/tests/codex-deny-destructive.test.sh
  plugins/megapowers/hooks/tests/deny-destructive.test.sh
  plugins/megapowers/hooks/tests/dispatch.test.sh
)

eval_tests=(
  evals/tests/coverage-inventory.test.sh
  evals/tests/portability-boundary.test.sh
  evals/tests/run-all-reporting.test.sh
  evals/tests/run-failclosed.test.sh
  evals/tests/score-failclosed.test.sh
  evals/studies/tests/installed-ab-contract.test.sh
  evals/studies/tests/pr-replay-contract.test.sh
)

run_check "test entrypoints executable" 10s bash -c '
  set -euo pipefail
  for test_path in "$@"; do
    test -x "$test_path" || { printf "not executable: %s\n" "$test_path"; exit 1; }
  done
' validate-entrypoints "${script_tests[@]}" "${hook_tests[@]}" "${eval_tests[@]}"

for test_path in "${script_tests[@]}"; do
  run_check "$test_path" 120s "$test_path"
done

for test_path in "${hook_tests[@]}"; do
  run_check "$test_path" 120s "$test_path"
done

for test_path in "${eval_tests[@]}"; do
  run_check "$test_path" 180s "$test_path"
done

if command -v claude >/dev/null 2>&1; then
  run_check "Claude marketplace strict validation" 90s \
    claude plugin validate --strict .claude-plugin/marketplace.json
  run_check "Claude plugin strict validation" 90s \
    claude plugin validate --strict plugins/megapowers
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
(( fail == 0 ))
