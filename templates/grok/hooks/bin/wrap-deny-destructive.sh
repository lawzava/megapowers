#!/usr/bin/env bash
# Grok PreToolUse adapter for mega-guardrails deny-destructive.
set -u
command -v jq >/dev/null 2>&1 || exit 0
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$here/lib.sh"
target="$(plugin_hook "mega-guardrails/hooks/deny-destructive.sh")" || exit 0
input="$(cat 2>/dev/null || true)"
[ -n "$input" ] || exit 0
normalized="$(printf '%s' "$input" | normalize_hook_input 2>/dev/null)" || exit 0
out="$(printf '%s' "$normalized" | "$target" 2>/dev/null)" || true
[ -n "$out" ] || exit 0
printf '%s' "$out" | translate_pretool_output 2>/dev/null || true
exit 0
