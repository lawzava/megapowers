#!/usr/bin/env bash
# Tests for the Codex SessionStart adapter: it wraps render-model-catalog output
# in the hookSpecificOutput.additionalContext envelope (the same shape Claude
# Code consumes; Codex's SessionStart mirrors it). Fail-open: when no catalog
# renders, the adapter emits nothing and exits 0 so a broken catalog never
# breaks session start.
# Run: plugins/megapowers/hooks/tests/codex-session-catalog.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../codex-session-catalog.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

pass=0; fail=0
ok()  { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }

echo "== codex-session-catalog tests =="

# 1. With the shipped catalog: valid JSON envelope carrying the catalog block.
out="$(bash "$HOOK" 2>/dev/null)"; rc=$?
[ "$rc" -eq 0 ] && ok || bad "exit 0 with shipped catalog (got $rc)"
printf '%s' "$out" | jq -e . >/dev/null 2>&1 && ok || bad "output is valid JSON"
ev="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.hookEventName' 2>/dev/null)"
[ "$ev" = "SessionStart" ] && ok || bad "hookEventName is SessionStart (got '$ev')"
ctx="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.additionalContext' 2>/dev/null)"
printf '%s' "$ctx" | grep -q 'Model catalog' && ok || bad "additionalContext carries the catalog block"
printf '%s' "$ctx" | grep -q 'lead:' && ok || bad "additionalContext names the lead"

# 2. Fail-open: no resolvable catalog -> no output, exit 0.
out="$(MODELS_TOML="/nonexistent/models.toml" bash "$HOOK" 2>/dev/null)"; rc=$?
[ "$rc" -eq 0 ] && ok || bad "exit 0 when the catalog cannot render (got $rc)"
[ -z "$out" ] && ok || bad "no output when the catalog cannot render"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
