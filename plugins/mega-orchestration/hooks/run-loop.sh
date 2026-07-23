#!/usr/bin/env bash
# Stop-hook accelerator for explicitly owned autonomous runs.
# Fail open on missing context, malformed ownership, or any uncertainty.
set -u
input="$(cat)"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=stop-context.sh
. "$here/stop-context.sh"

command -v jq >/dev/null 2>&1 || exit 0
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0
stop_context_is_exempt "$input" && exit 0

session_id="$(printf '%s' "$input" | jq -r '.session_id // empty')"
[ -n "$session_id" ] || exit 0
base="${MEGAPOWERS_RUN_DIR:-.megapowers/run}"
[ -d "$base" ] || exit 0
transcript="$(printf '%s' "$input" | jq -r '.transcript_path // empty')"

owner_matches() {
  local file="$1"
  [ -f "$file" ] || return 1
  jq -e --arg sid "$session_id" '
    .schema == 1 and .session_id == $sid and .role == "autonomous-run"
  ' "$file" >/dev/null 2>&1
}

claim_invoked() {
  local id="$1"
  [ -n "$transcript" ] && [ -f "$transcript" ] || return 1
  jq -e --arg id "$id" '
    def tool_uses:
      if .type == "tool_use" then .
      elif .type == "assistant" then
        .message.content[]? | select(.type == "tool_use")
      else empty end;
    tool_uses |
    select(.name == "Bash") |
    select((.input.command // "") |
      test("(^|[;&|[:space:]])([^[:space:]]*/)?run-claim[[:space:]]+" + $id + "([;&|[:space:]]|$)"))
  ' "$transcript" >/dev/null 2>&1
}

for dir in "$base"/*/; do
  [ -d "$dir" ] || continue
  [ -f "$dir/status" ] || continue
  id="$(basename "$dir")"
  case "$id" in ''|*[!A-Za-z0-9_-]*) continue ;; esac
  owner_file="$dir/owner.json"
  if ! owner_matches "$owner_file"; then
    claim_invoked "$id" || continue
    tmp_owner="$owner_file.tmp.$$"
    jq -cn --arg sid "$session_id" \
      '{schema:1,session_id:$sid,role:"autonomous-run"}' > "$tmp_owner" 2>/dev/null || {
      rm -f "$tmp_owner" 2>/dev/null
      continue
    }
    mv "$tmp_owner" "$owner_file" 2>/dev/null || {
      rm -f "$tmp_owner" 2>/dev/null
      continue
    }
  fi
  state="$(sed -n 's/^STATE=//p' "$dir/status" | head -1)"
  case "$state" in
    initialized|working|running|in-progress|in_progress) ;;
    *) continue ;;
  esac
  level="$(sed -n 's/^LEVEL=//p' "$dir/status" | head -1)"
  [ "$level" = "in-the-loop" ] && continue
  cursor="$(sed -n 's/^CURSOR=//p' "$dir/status" | head -1)"
  scripts="$here/../skills/autonomous-run/scripts"
  jq -nc --arg id "$id" --arg state "${state:-unset}" --arg cursor "${cursor:-unset}" \
    --arg journal "$scripts/run-journal" --arg derive "$scripts/run-derive-status" \
    --arg verify "$scripts/run-verify-status" \
    '{decision:"block", reason:("Autonomous run " + $id + " is owned by this session and active (STATE=" + $state + ", CURSOR=" + $cursor + "). Continue the next unmet milestone and its declared acceptance check. Journal the result with " + $journal + ", derive status with " + $derive + ", and verify closure with " + $verify + ". To stop deliberately, journal paused or blocked and re-derive status.")}'
  exit 0
done
exit 0
