#!/usr/bin/env bash
# Stop-hook loop driver for autonomous-run (Claude Code only): if this session
# is working an active run (.megapowers/run/<id>/ with a live STATE) and the
# session tries to stop, block once and point at the next unmet milestone.
# The discipline lives in the autonomous-run skill; this is an accelerator.
# Fail-open: any error/uncertainty -> allow (exit 0). Self-suppressing via
# stop_hook_active. A run only stops reading "active" through honest state:
# journal a blocked/paused/result entry and re-derive the status file.
# Depends only on jq/grep/sed.
set -u
input="$(cat)"

command -v jq >/dev/null 2>&1 || exit 0

# Avoid loops: if this stop was already triggered by a stop hook, allow.
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0

base="${MEGAPOWERS_RUN_DIR:-.megapowers/run}"
[ -d "$base" ] || exit 0

# Only drive runs this session actually touched: a stale run from another day
# must not haunt every later session. The transcript names the run dir when the
# session read/wrote it (paths land in tool_use inputs and results).
transcript="$(printf '%s' "$input" | jq -r '.transcript_path // empty')"
[ -n "$transcript" ] && [ -f "$transcript" ] || exit 0

for dir in "$base"/*/; do
  [ -d "$dir" ] || continue
  [ -f "$dir/status" ] || continue
  id="$(basename "$dir")"
  # Match the path form with a trailing slash ("<base>/<id>/"), not the bare
  # id: a run named "fix" must not match prose, and run "r1" must not match a
  # touch of "r1-old/". Real touches reference files under the run dir.
  grep -qF "$base/$id/" "$transcript" 2>/dev/null || continue
  state="$(sed -n 's/^STATE=//p' "$dir/status" | head -1)"
  case "$state" in
    initialized|working|running|in-progress|in_progress) ;;
    *) continue ;;   # blocked/paused/done/anything-else: the run says stop is fine
  esac
  # in-the-loop means the human approves at milestone boundaries; the hook must
  # not bulldoze that checkpoint. The loop discipline rides on the skill there.
  level="$(sed -n 's/^LEVEL=//p' "$dir/status" | head -1)"
  [ "$level" = "in-the-loop" ] && continue
  cursor="$(sed -n 's/^CURSOR=//p' "$dir/status" | head -1)"
  # CURSOR is agent-written free text: build the JSON with jq so no value can
  # break the hook's output.
  jq -nc --arg id "$id" --arg state "${state:-unset}" --arg cursor "${cursor:-unset}" \
    '{decision:"block", reason:("Autonomous run " + $id + " is active (STATE=" + $state + ", CURSOR=" + $cursor + ") and its done-when criteria are not recorded as met. Continue the loop per its runbook: do the next unmet milestone, run its declared acceptance check, journal the result (scripts/run-journal), and re-derive status (scripts/run-derive-status — it derives done when every milestone is done). If you are pausing deliberately, or you are blocked, or a charter cap is reached, journal a paused or blocked entry and re-derive status so STATE says so — then stopping is correct. Verify a finished run with scripts/run-verify-status.")}'
  exit 0
done
exit 0
