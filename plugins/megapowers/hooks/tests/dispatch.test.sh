#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatch="$here/dispatch.sh"
pass=0
fail=0

echo "== guardrail dispatcher tests =="
input='{"tool_name":"Bash","tool_input":{"command":"git reset --hard HEAD~1"}}'
claude_out="$(printf '%s' "$input" | env -u PLUGIN_ROOT "$dispatch" deny-destructive.sh codex-deny-destructive.sh)"
if [ -z "$claude_out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL Claude path must emit no reversible-risk decision"; fi

codex_out="$(printf '%s' "$input" | PLUGIN_ROOT="$here/.." "$dispatch" deny-destructive.sh codex-deny-destructive.sh)"
if [ -z "$codex_out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL Codex path must emit no reversible-risk decision"; fi

set +e
missing_err="$(printf '%s' "$input" | "$dispatch" missing-hook.sh codex-deny-destructive.sh 2>&1 >/dev/null)"
missing_rc=$?
set -e
if [ "$missing_rc" -ne 0 ] && printf '%s' "$missing_err" | grep -q 'cannot evaluate'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL missing hook must report a visible evaluation error"
fi

set +e
invalid_err="$(printf '{}' | "$dispatch" deny-destructive.sh codex-deny-destructive.sh 2>&1 >/dev/null)"
invalid_rc=$?
set -e
if [ "$invalid_rc" -ne 0 ] && printf '%s' "$invalid_err" | grep -q 'cannot evaluate'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL invalid hook input must report a visible evaluation error"
fi

# One matcher, one command, two harness adapters: Bash and PowerShell tool calls
# hand the same tool_input.command field to the same dispatch, which routes to
# the native guard on Claude Code and through the Codex adapter on Codex.
hooks_json="$here/hooks.json"
matcher="$(jq -r '.hooks.PreToolUse[0].matcher' "$hooks_json")"
if [ "$matcher" = "Bash|PowerShell" ]; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL hooks.json matcher must cover Bash and PowerShell, got: $matcher"
fi
command_line="$(jq -r '.hooks.PreToolUse[0].hooks[0].command' "$hooks_json")"
case "$command_line" in
  *"dispatch.sh deny-destructive.sh codex-deny-destructive.sh"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); echo "  FAIL hooks.json must keep one shared command routing both adapters" ;;
esac

printf '%s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
