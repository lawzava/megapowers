#!/usr/bin/env bash
# Codex SessionStart adapter: inject the rendered model catalog into session
# context, the same block the Claude Code session-start hook injects. Codex's
# SessionStart mirrors Claude Code's schema and consumes
# hookSpecificOutput.additionalContext, so only the payload differs: Codex
# sessions get the catalog alone, not the using-megapowers nudge (that skill's
# platform guidance is Claude Code specific; Codex loads its charter from
# AGENTS.md instead).
#
# The cross-harness session-start dispatcher selects this adapter when Codex
# loads the plugin's hooks/hooks.json. Trust the installed definition via /hooks.
# Fails OPEN: no renderable catalog -> no output, exit 0, session start proceeds
# untouched.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

catalog="$("$here/render-model-catalog" 2>/dev/null || true)"
[ -n "$catalog" ] || exit 0

# shellcheck source=lib-json.sh
. "$here/lib-json.sh"

printf '{\n  "hookSpecificOutput": {\n    "hookEventName": "SessionStart",\n    "additionalContext": "%s"\n  }\n}\n' "$(escape_for_json "$catalog")"
exit 0
