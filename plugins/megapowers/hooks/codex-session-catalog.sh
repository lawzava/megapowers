#!/usr/bin/env bash
# Codex SessionStart adapter: inject the rendered model catalog into session
# context, the same block the Claude Code session-start hook injects. Codex's
# SessionStart mirrors Claude Code's schema and consumes
# hookSpecificOutput.additionalContext, so only the payload differs: Codex
# sessions get the catalog alone, not the using-megapowers nudge (that skill's
# platform guidance is Claude Code specific; Codex loads its charter from
# AGENTS.md instead).
#
# Wiring is manual today (mirror of the mega-guardrails deny-destructive pilot):
# reference this script from a Codex hooks manifest (hooks/codex-hooks.json here,
# or ~/.codex/hooks.json / <repo>/.codex/hooks.json) on SessionStart, then trust
# it via /hooks inside Codex. Fails OPEN: no renderable catalog -> no output,
# exit 0, session start proceeds untouched.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

catalog="$("$here/render-model-catalog" 2>/dev/null || true)"
[ -n "$catalog" ] || exit 0

escape_for_json() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\t'/\\t}"
    printf '%s' "$s"
}

printf '{\n  "hookSpecificOutput": {\n    "hookEventName": "SessionStart",\n    "additionalContext": "%s"\n  }\n}\n' "$(escape_for_json "$catalog")"
exit 0
