#!/usr/bin/env bash
# Codex PreToolUse adapter for deny-destructive.sh.
#
# Codex's hook event schema deliberately mirrors Claude Code's: the PreToolUse
# input carries the shell command at .tool_input.command, and a deny is returned
# as {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":
# "deny",...}}. Codex even exposes CLAUDE_PLUGIN_ROOT as a compatibility alias.
# So the INPUT needs no field remapping; this adapter pipes stdin straight into
# the existing guard.
#
# The one real gap is on OUTPUT. Codex's PreToolUse supports permissionDecision
# "deny" and "allow"; "ask" is "parsed but not supported yet". Returning an
# unsupported value makes Codex mark the hook run failed, report the error, and
# then run the tool anyway. deny-destructive.sh emits "ask" for the reversible
# -but-risky tier (git reset --hard, aws s3 rm --recursive, docker prune -f,
# terraform destroy -auto-approve, kubectl delete --all, curl | bash). A Codex
# command hook cannot surface an interactive confirmation, so this adapter maps:
#
#   guard "deny"        -> pass the deny JSON through (Codex blocks the command)
#   guard "ask"         -> emit nothing, exit 0 (fall back to Codex's own
#                          approval flow; do NOT force-allow a risky command and
#                          do NOT return the unsupported "ask")
#   guard allow/no hit  -> emit nothing, exit 0
#
# Wiring: deny-destructive-dispatch.sh selects this adapter from the plugin's
# hooks.json when Codex sets PLUGIN_ROOT. Locating the sibling guard via
# BASH_SOURCE keeps it directly testable. Fails OPEN: any error exits 0 (allow).
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
guard="$here/deny-destructive.sh"
[ -x "$guard" ] || exit 0

input="$(cat 2>/dev/null || true)"
out="$(printf '%s' "$input" | "$guard" 2>/dev/null || true)"
[ -n "$out" ] || exit 0

command -v jq >/dev/null 2>&1 || exit 0
decision="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.permissionDecision // empty' 2>/dev/null || true)"
[ "$decision" = "deny" ] && printf '%s' "$out"
exit 0
