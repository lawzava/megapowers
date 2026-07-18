#!/usr/bin/env bash
# Stop-hook nudge: if risky logic changed this session WITHOUT an independent
# delegate review, ask for one before finishing. Deterministic, cheap, no model
# call, and a heuristic backstop rather than a proof of review. Fail-open: any
# error/uncertainty -> allow (exit 0). Self-suppressing via stop_hook_active,
# the delegate-usage check, and once-per-diff-state (below). Runs on both
# Claude Code and Codex (hooks.json passes it as both dispatch.sh targets).
# Depends only on jq/git/grep (sha256sum is optional; without it the
# once-per-diff-state suppression is skipped).
set -u
input="$(cat)"

command -v jq >/dev/null 2>&1 || exit 0

# Avoid loops: if this stop was already triggered by a stop hook, allow.
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0

# If an independent delegate was actually INVOKED this session, allow. Match
# the JSON structure of a tool call, not a mere mention — otherwise merely
# reading this repo's own docs (which name `codex exec`, `model-delegate`,
# etc.) would silence the nudge for the rest of the session. Real invocations
# show up as: an mcp__codex__codex* tool_use name, a delegate subagent
# dispatch (subagent_type starting "codex" or ending "-delegate"), a Bash
# command field that runs a delegate CLI, or a Codex rollout cmd key. The
# marker list mirrors the shipped `detect` arrays in delegates.toml and
# models.toml; a new provider means adding its marker here.
#
# The CLI-marker prefix is [^"\\]* (no escaped characters), so a marker QUOTED
# inside a command (`rg -n \"claude -p\" docs`) does not count as a review:
# the escaped quote before the marker breaks the match. The codex-companion
# alternative keeps an escape-tolerant prefix because its own path is quoted.
# Residual limitation: an unquoted prose mention in a command value
# (`echo claude -p`) still suppresses; the pre-rewrite hook shared it.
delegate_re='"name"[[:space:]]*:[[:space:]]*"mcp__codex__codex|"subagent_type"[[:space:]]*:[[:space:]]*"(codex|[a-z0-9_-]+-delegate)|"command"[[:space:]]*:[[:space:]]*"[^"\\]*(codex +exec|agy +(exec|run)|claude +-p)|"command"[[:space:]]*:[[:space:]]*"([^"\\]|\\.)*codex-companion\.mjs(\\")? +(adversarial-)?review|(\\")?cmd(\\")?:[[:space:]]*\\?"[^"\\]*(codex +exec|claude +-p)'
transcript="$(printf '%s' "$input" | jq -r '.transcript_path // empty')"
if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  grep -qE "$delegate_re" "$transcript" 2>/dev/null && exit 0
fi

# Remove the once-per-diff-state sentinel. Called on every non-blocking path
# (clean tree, or a diff with no risky content) so the sentinel only persists
# while risky state is continuously present: any disappearance re-arms the
# nudge, so a later reintroduction of the same hunk is never mistaken for the
# one already reviewed.
clear_sentinel() {
  local s
  s="$(git rev-parse --git-path megapowers-delegate-nudge-seen 2>/dev/null)"
  [ -n "$s" ] && rm -f "$s" 2>/dev/null
}

# Only relevant inside a git repo with pending changes.
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
diff="$(git diff HEAD 2>/dev/null)"
# git diff HEAD misses untracked new files — a new auth/billing file would
# escape. Fold in untracked filenames (respecting .gitignore) and contents.
untracked="$(git ls-files --others --exclude-standard 2>/dev/null)"
if [ -z "$diff" ] && [ -z "$untracked" ]; then
  clear_sentinel
  exit 0
fi

# 'authn|authz|authenticat|authoriz' instead of bare 'auth' so it fires on real
# auth code but not on the word 'author'/'authored' (commit trailers, docs).
risky='authn|authz|authenticat|authoriz|oauth|jwt|saml|passwd|password|billing|payment|invoice|subscription|stripe|webhook|mutex|goroutine|semaphore|deadlock|concurren'
hit=0
diff_hits="$(printf '%s' "$diff" | grep -iE "$risky")"
[ -n "$diff_hits" ] && hit=1

untracked_name_hits=""
untracked_content_hits=""
if [ -n "$untracked" ]; then
  untracked_name_hits="$(printf '%s' "$untracked" | grep -iE "$risky")"   # filename signal
  [ -n "$untracked_name_hits" ] && hit=1
  # Bound the content scan: cap to the first 50 untracked files, one batched
  # grep. A risky-by-content-only file past the cap is not content-scanned;
  # the uncapped filename scan above still catches risky-named files.
  scan_files=()
  while IFS= read -r f; do
    [ -f "$f" ] && scan_files+=("$f")
  done < <(printf '%s\n' "$untracked" | head -50)
  if [ "${#scan_files[@]}" -gt 0 ]; then
    untracked_content_hits="$(grep -ihE "$risky" -- "${scan_files[@]}" 2>/dev/null)"
    [ -n "$untracked_content_hits" ] && hit=1
  fi
fi

if [ "$hit" -eq 0 ]; then
  clear_sentinel
  exit 0
fi

# Once per diff-state: a risky diff that hasn't changed since the last block
# has already been nudged. Hash the risky-matching lines and compare to the
# last-blocked state. Store via `git rev-parse --git-path`, never a literal
# .git/ path: in a linked worktree .git is a file, and a hardcoded path would
# miss the per-worktree gitdir. Without sha256sum the key stays empty and the
# suppression is skipped (always nudge, the pre-sentinel behavior).
sentinel="$(git rev-parse --git-path megapowers-delegate-nudge-seen 2>/dev/null)"
key=""
if command -v sha256sum >/dev/null 2>&1; then
  # The key hashes only the risky-matching lines, so surrounding non-risky
  # edits in the same diff do not re-arm the nudge.
  key="$(
    { printf '%s\n' "$diff_hits"; printf '%s\n' "$untracked_name_hits"; printf '%s\n' "$untracked_content_hits"; } \
      | sort | sha256sum | cut -d' ' -f1
  )"
fi
if [ -n "$sentinel" ] && [ -n "$key" ]; then
  prev="$(cat "$sentinel" 2>/dev/null)"
  if [ "$prev" = "$key" ]; then
    exit 0   # same risky diff-state already nudged; nothing new to review
  fi
  printf '%s' "$key" > "$sentinel" 2>/dev/null
fi
printf '%s\n' '{"decision":"block","reason":"Risky logic (auth/billing/concurrency) changed without an independent delegate review. Resolve a reviewer with the multi-agent-delegation skill (scripts/delegate-resolve verify --exclude-lead) and run an independent pass on the diff with that model, then finish. If you already reviewed it with a delegate this session, say so and stop."}'
exit 0
