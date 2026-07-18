#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatch="$here/dispatch.sh"
pass=0
fail=0

echo "== guardrail dispatcher tests =="
input='{"tool_name":"Bash","tool_input":{"command":"git reset --hard HEAD~1"}}'
claude_out="$(printf '%s' "$input" | env -u PLUGIN_ROOT "$dispatch" deny-destructive.sh codex-deny-destructive.sh)"
if printf '%s' "$claude_out" | grep -q '"permissionDecision": "ask"'; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL Claude path keeps ask decision"; fi

codex_out="$(printf '%s' "$input" | PLUGIN_ROOT="$here/.." "$dispatch" deny-destructive.sh codex-deny-destructive.sh)"
if [ -z "$codex_out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL Codex path drops unsupported ask decision"; fi

format_out="$(printf '%s' '{"tool_name":"Edit","tool_input":{}}' | PLUGIN_ROOT="$here/.." "$dispatch" auto-format.sh)"
if [ -z "$format_out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL Codex path no-ops a Claude-only hook"; fi

printf '%s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
