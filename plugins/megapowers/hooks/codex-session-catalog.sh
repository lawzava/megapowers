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

# Who is running, appended to the block the way the OpenCode plugin appends its
# own identity. The catalog renders `lead: <[lead] provider>`, and the shipped
# [lead] is Claude; a Codex session reading that alone concludes Claude is in
# charge, and delegate-resolve agrees, falling back to the catalog [lead] with
# CALLER=assumed-lead for every route it is not told about. That misroutes
# `self` roles to another vendor and prints "native" for a provider this session
# is not.
#
# Provider only, no model id. This adapter is Codex by construction, so `codex`
# cannot be wrong; the running model can be. ~/.codex/config.toml holds a
# default that a --model flag, a profile, or an in-session /model switch all
# override, and delegate-resolve exits 2 on a --caller-model that matches no
# tier map, so a stale id read from disk would break route resolution outright
# rather than sharpen it. The provider is what makes a route native and what
# keeps `self` at home; that is the whole job here.
identity="This session runs Codex, so Codex leads it. The lead line above is the catalog default for a session that does not declare itself; route resolution here takes --caller-provider codex."
catalog="$catalog
$identity"

# shellcheck source=lib-json.sh
. "$here/lib-json.sh"

printf '{\n  "hookSpecificOutput": {\n    "hookEventName": "SessionStart",\n    "additionalContext": "%s"\n  }\n}\n' "$(escape_for_json "$catalog")"
exit 0
