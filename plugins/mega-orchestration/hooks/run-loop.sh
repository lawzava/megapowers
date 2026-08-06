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

# This gate's state is declared in ../enforcement.toml, not decided here, so a
# repository can turn it off with `state = "advisory"` in a project layer instead
# of patching a hook. Layers are read project, then user, then shipped, first hit
# wins, matching how models.toml and delegates.toml already layer.
#
# THE PROJECT LAYER COMES FROM THE BASE COMMIT, NEVER FROM THE WORKTREE, exactly as
# hooks/delegate-nudge.sh reads it. This hook exists to keep a long autonomous run
# going, and that run is the thing writing the pending tree: reading the layer from
# disk let a run drop `state = "off"` into .megapowers/enforcement.toml and release
# itself, with nothing said and no commit needed. Committed content is different in
# kind, because a human already accepted it. A repository with no commits, or no
# repository at all, has no reviewed content and therefore no project layer, so the
# shipped defaults apply. The user layer stays worktree-readable: it lives outside
# the repository, reaches no diff, and is the machine owner speaking rather than the
# run speaking.
#
# Fails CLOSED. An unreadable parser or a missing rules file leaves the gate on,
# because the alternative is that deleting a file silently disables a gate.
project_layer=".megapowers/enforcement.toml"
project_committed=""
project_kind=""
project_changed=0
# Read by the exits below whether or not the parser loaded, so `set -u` cannot
# abort the hook on a machine missing lib-toml.sh. `enforced` is the value a
# missing parser leaves in place, which is the fail-closed direction.
policy_note=""
policy_state="enforced"
root="$(git rev-parse --show-toplevel 2>/dev/null)" || root=""
# shellcheck source=../skills/multi-agent-delegation/scripts/lib-toml.sh
. "$here/../skills/multi-agent-delegation/scripts/lib-toml.sh" 2>/dev/null || true
if declare -F toml_scalar_in >/dev/null 2>&1; then
  # Removed on EXIT rather than after the last read, because every path out of the
  # state check below is an `exit`.
  trap 'rm -f "$project_committed" 2>/dev/null' EXIT
  if [ -n "$root" ] && git rev-parse --verify -q HEAD >/dev/null 2>&1; then
    # Materialized to a temp file because lib-toml.sh reads files rather than
    # strings. `HEAD:<path>` resolves against the repository ROOT, so a stop from a
    # subdirectory reads the same layer as a stop from the top.
    project_committed="$(mktemp 2>/dev/null)" || project_committed=""
    if [ -z "$project_committed" ] ||
       ! git show "HEAD:$project_layer" > "$project_committed" 2>/dev/null; then
      rm -f "$project_committed" 2>/dev/null
      project_committed=""
    fi
  fi
  # A PENDING ADD, EDIT OR DELETE IS ANNOUNCED, NEVER HONORED, in both directions.
  # BOTH SIDES are compared, because `git diff HEAD` says nothing about the index:
  # staging the edit and restoring the worktree copy to its committed bytes leaves
  # the worktree clean while the change sits one plain `git commit` from governing.
  if [ -n "$root" ]; then
    if [ -n "$project_committed" ]; then
      git diff --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null || project_changed=1
      git diff --cached --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null || project_changed=1
      project_kind=edit
      [ -e "$root/$project_layer" ] || project_kind=deletion
    elif [ -e "$root/$project_layer" ] ||
         [ -n "$(git ls-files --cached -- ":(top,literal)$project_layer" 2>/dev/null)" ]; then
      # `ls-files --cached` and not `git diff --cached HEAD`, because this branch is
      # also the one a repository with no commits takes, where naming HEAD is a
      # fatal error rather than a clean tree.
      project_changed=1
      project_kind=addition
    fi
  fi
  # One resolver, called twice: once with the committed project layer, which
  # governs, and once with the pending one, which only ever gets announced. Passing
  # the layer in is what keeps the two answers comparable.
  resolve_state() {
    local f v
    for f in ${1:+"$1"} \
             "${XDG_CONFIG_HOME:-$HOME/.config}/megapowers/enforcement.toml" \
             "$here/../enforcement.toml"; do
      [ -f "$f" ] || continue
      v="$(toml_scalar_in "$f" "rules.autonomous-run-continuation" state 2>/dev/null)"
      [ -n "$v" ] && { printf '%s' "$v"; return 0; }
    done
    return 1
  }
  # An ABSENT or unreadable value stays enforced, because a gate that disables
  # itself when its own rules go missing is a gate anyone disables by deleting a
  # file. Anything other than "enforced" is off or advisory, and an advisory rule
  # says nothing from a hook.
  policy_state="$(resolve_state "$project_committed")" || policy_state=""
  [ -n "$policy_state" ] || policy_state="enforced"
  pending_state="$policy_state"
  if [ "$project_changed" -eq 1 ]; then
    # WHICH COPY THE PENDING STATE IS READ FROM. The worktree copy almost always,
    # because that is the file the author is editing. The exception is the masked
    # case detected above: the worktree copy reads exactly as the committed one
    # while the INDEX holds something else. Reading the worktree there would report
    # the committed state straight back and deny the very edit it announces. A
    # deletion has no pending file at all, and resolving with none is exactly the
    # policy that would apply once the layer is gone.
    pending_file=""
    pending_staged=""
    if git diff --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null &&
       ! git diff --cached --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null; then
      pending_staged="$(mktemp 2>/dev/null)" || pending_staged=""
      [ -n "$pending_staged" ] && git show ":$project_layer" > "$pending_staged" 2>/dev/null &&
        pending_file="$pending_staged"
    fi
    [ -n "$pending_file" ] || { [ -r "$root/$project_layer" ] && pending_file="$root/$project_layer"; }
    pending_state="$(resolve_state "$pending_file")" || pending_state=""
    [ -n "$pending_state" ] || pending_state="enforced"
    [ -z "$pending_staged" ] || rm -f "$pending_staged"
  fi
  # THE EDIT IS ANNOUNCED WHEN IT WOULD CHANGE THIS RULE'S ANSWER, and only then.
  # A pending layer that leaves this rule where it stood says nothing this hook
  # knows, and hooks/delegate-nudge.sh already announces the file itself at the same
  # stop, so speaking here too would put two messages on one fact. Off and advisory
  # differ from each other in what they mean elsewhere and not in what this hook
  # does, so the comparison is on the one bit this gate acts on.
  now_on=0; [ "$policy_state" = "enforced" ] && now_on=1
  pending_on=0; [ "$pending_state" = "enforced" ] && pending_on=1
  if [ "$project_changed" -eq 1 ] && [ "$now_on" != "$pending_on" ]; then
    policy_note="A pending $project_kind of $project_layer is not honored by this gate, which reads that layer as it stands in the base commit: a run must not be able to release itself. This stop ran with state = $policy_state, and the pending file would set state = $pending_state. Read that edit before it is committed, because from the commit onward it governs every stop. "
  fi
fi
# A NOTICE, NOT A BLOCK, on every path that would otherwise end in silence. The
# pending layer already buys nothing, so there is no run to drive and nothing to
# stop; what was missing is that a human sees the moment policy moves. A stop that
# blocks for a real finding names the edit in its reason instead, so the fact is
# stated exactly once.
policy_notice_exit() {
  [ -z "$policy_note" ] || jq -nc --arg m "Policy notice: $policy_note" '{systemMessage:$m}'
  exit 0
}
[ "$policy_state" = "enforced" ] || policy_notice_exit

session_id="$(printf '%s' "$input" | jq -r '.session_id // empty')"
[ -n "$session_id" ] || policy_notice_exit
base="${MEGAPOWERS_RUN_DIR:-.megapowers/run}"
[ -d "$base" ] || policy_notice_exit
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
    --arg verify "$scripts/run-verify-status" --arg note "$policy_note" \
    '{decision:"block", reason:($note + "Autonomous run " + $id + " is owned by this session and active (STATE=" + $state + ", CURSOR=" + $cursor + "). Continue the next unmet milestone and its declared acceptance check. Journal the result with " + $journal + ", derive status with " + $derive + ", and verify closure with " + $verify + ". To stop deliberately, journal paused or blocked and re-derive status.")}'
  exit 0
done
policy_notice_exit
