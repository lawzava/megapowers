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

# Provider only, no model id. This adapter is Codex by construction, so `codex`
# cannot be wrong; the running model can be. ~/.codex/config.toml holds a
# default that a --model flag, a profile, or an in-session /model switch all
# override, and delegate-resolve exits 2 on a --caller-model that matches no
# tier map, so a stale id read from disk would break route resolution outright
# rather than sharpen it. The provider is what makes a route native and what
# keeps `self` at home; that is the whole job here.
#
# --caller puts Codex on the block's lead line. The earlier shape left that line
# reading `lead: claude` and appended a correction underneath the whole catalog;
# sessions acted on the lead line and treated Claude as in charge anyway, which
# is what the flag exists to stop.
catalog="$("$here/render-model-catalog" --caller codex 2>/dev/null || true)"
[ -n "$catalog" ] || exit 0

# The flag that carries the same fact into route resolution. Without it
# delegate-resolve falls back to the catalog [lead] with CALLER=assumed-lead,
# misrouting `self` roles to another vendor and printing "native" for a provider
# this session is not.
identity="Route resolution here takes --caller-provider codex."
catalog="$catalog
$identity"

# shellcheck source=lib-json.sh
. "$here/lib-json.sh"

printf '{\n  "hookSpecificOutput": {\n    "hookEventName": "SessionStart",\n    "additionalContext": "%s"\n  }\n}\n' "$(escape_for_json "$catalog")"
exit 0
