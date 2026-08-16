# Shared helpers for Grok wrappers around Claude-shaped megapowers hooks.
# Fail open: missing jq, missing target, or empty input means no stdout, exit 0.

plugin_hook() {
  local rel="$1" p
  for p in \
    ${MEGAPOWERS_PLUGIN_DIR:+"${MEGAPOWERS_PLUGIN_DIR}/${rel}"} \
    "${HOME}/.claude/plugins/marketplaces/megapowers/plugins/${rel}" \
    "${HOME}/Code/lawzava/megapowers/plugins/${rel}"
  do
    [ -x "$p" ] && { printf '%s' "$p"; return 0; }
  done
  return 1
}

normalize_hook_input() {
  jq -c '
    def pick($a; $b):
      if $a != null then $a elif $b != null then $b else null end;
    . as $in
    | .tool_input = ($in.tool_input // $in.toolInput // {})
    | .tool_name = (pick($in.tool_name; $in.toolName) // "")
    | .stop_hook_active = (pick($in.stop_hook_active; $in.stopHookActive) // false)
    | .permission_mode = (pick($in.permission_mode; $in.permissionMode) // "")
    | .agent_type = (pick($in.agent_type; $in.agentType) // "")
    | .transcript_path = (pick($in.transcript_path; $in.transcriptPath) // "")
    | .reason = ($in.reason // "")
    | .last_assistant_message = (pick($in.last_assistant_message; $in.lastAssistantMessage) // "")
  '
}

translate_pretool_output() {
  # Grok PreToolUse understands allow/deny only. Claude ASK is not a Grok
  # decision: pass those through (empty stdout) so Grok's permission prompt
  # is the human gate. DENY stays DENY.
  jq -c '
    if (.hookSpecificOutput.permissionDecision // "") == "deny" then
      {decision:"deny", reason:(.hookSpecificOutput.permissionDecisionReason // "")}
    elif .decision == "deny" then
      {decision:"deny", reason:(.reason // "")}
    else
      empty
    end
  '
}
