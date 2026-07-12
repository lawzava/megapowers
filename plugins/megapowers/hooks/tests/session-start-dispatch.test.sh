#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatch="$here/session-start-dispatch"
pass=0
fail=0

check_contains() {
  local label="$1" output="$2" pattern="$3"
  if printf '%s' "$output" | grep -qF "$pattern"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    printf '  FAIL %s\n' "$label"
  fi
}

echo "== session-start dispatcher tests =="
claude_out="$(printf '%s' '{"source":"startup"}' | env -u PLUGIN_ROOT "$dispatch")"
check_contains "Claude path injects workflow guidance" "$claude_out" "The Core Rule"

codex_out="$(printf '%s' '{"source":"startup"}' | PLUGIN_ROOT="$here/.." "$dispatch")"
check_contains "Codex path injects model catalog" "$codex_out" "Model catalog"

printf '%s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
