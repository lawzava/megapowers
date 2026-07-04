#!/usr/bin/env bash
# Stop-hook nudge: if risky logic changed this session WITHOUT an independent
# delegate review, ask for one before finishing. Deterministic, cheap, no model
# call. Fail-open: any error/uncertainty -> allow (exit 0). Self-suppressing via
# stop_hook_active, the delegate-usage check, and once-per-diff-state (below).
# Depends only on jq/git/grep, plus a hashing tool for the once-per-diff-state
# sentinel (sha256sum, shasum, or openssl; falls back to always-block if none
# are present, same as before this sentinel existed).
set -u
input="$(cat)"

command -v jq >/dev/null 2>&1 || exit 0

# Avoid loops: if this stop was already triggered by a stop hook, allow.
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0

# If an independent delegate was actually INVOKED this session, allow. Match the JSON
# structure of a tool call, not a mere mention — otherwise merely reading this repo's
# own docs (which name `codex exec`, `gemini -p`, `codex-delegate`, etc.) would silence
# the nudge for the rest of the session, exactly when delegation was considered and
# skipped. Real invocations show up as: an mcp__codex__codex* tool_use name (the default
# channel — including the codex-reply continuation), a delegate subagent dispatch
# (any subagent_type containing `codex` or `delegate`), or a Bash command field that
# runs a delegate CLI. Matching is anchored on those JSON field names, so a prose or
# code-block mention (not inside a "name"/"subagent_type"/"command" value) does not match.
transcript="$(printf '%s' "$input" | jq -r '.transcript_path // empty')"
if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  # This is a heuristic backstop, not a proof of review: it looks for the JSON
  # signatures of a real delegate call. The subagent_type value must START with
  # "codex" or END in "-delegate" (so "notdelegate" does NOT count); the command
  # must run a review-capable delegate CLI ("codex apply" is excluded — applying a
  # patch is not an independent review). It fails open on any doubt.
  grep -qE '"name"[[:space:]]*:[[:space:]]*"mcp__codex__codex|"subagent_type"[[:space:]]*:[[:space:]]*"(codex|[a-z0-9_-]+-delegate)|"command"[[:space:]]*:[[:space:]]*"[^"]*(codex +exec|agy +(exec|run))' "$transcript" 2>/dev/null && exit 0
fi

# Hash stdin to a hex digest with whatever tool is on PATH; normalizes each
# tool's output to just the digest (openssl prints "SHA2-256(stdin)= <hex>").
# No tool found -> drain stdin and print nothing, so the caller's key is empty
# and the once-per-diff-state check below is skipped (fail open: always block,
# the behavior before this sentinel existed).
hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | cut -d' ' -f1
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 | sed 's/^.*= *//'
  else
    cat >/dev/null
  fi
}

# Remove the once-per-diff-state sentinel. Called on every non-blocking path
# (clean tree, or a diff with no risky content) so the sentinel only persists
# while risky state is continuously present: any disappearance re-arms the
# nudge, so a later reintroduction of the same hunk (revert, cherry-pick,
# unstash, or a fresh unrelated risky diff) is never mistaken for the one
# already reviewed.
clear_sentinel() {
  local s
  s="$(git rev-parse --git-path megapowers-delegate-nudge-seen 2>/dev/null)"
  [ -n "$s" ] && rm -f "$s" 2>/dev/null
}

# Only relevant inside a git repo with pending changes.
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
diff="$(git diff HEAD 2>/dev/null)"
# git diff HEAD misses untracked new files — a new auth/billing file would escape.
# Fold in untracked filenames (respecting .gitignore) and their contents.
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
  # Bound the content scan: an unbounded per-file grep loop over every untracked
  # file is a per-turn cost that scales with repo scaffolding, not with risk. Cap
  # to the first 50 untracked files and read them in one batched grep instead of
  # a process per file.
  # A risky-by-content-only untracked file past this cap is not content-scanned;
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

# Risky-logic signal (paths or content). Bias to allow: high-signal terms only.
if [ "$hit" -eq 1 ]; then
  # Once per diff-state: a risky diff that hasn't changed since the last block
  # has already been nudged, so re-blocking on every following Stop just burns a
  # model turn for no new information. Hash the risky-matching lines (diff plus
  # untracked names/content) and compare to the last-blocked state. Store it via
  # `git rev-parse --git-path`, never a literal .git/ path: in a linked worktree
  # .git is a file, not a directory, and a hardcoded path would either miss the
  # per-worktree gitdir or clobber that file outright.
  sentinel="$(git rev-parse --git-path megapowers-delegate-nudge-seen 2>/dev/null)"
  # By design the key hashes only the risky-matching lines, so surrounding
  # non-risky edits in the same diff do not re-arm the nudge.
  key="$(
    { printf '%s\n' "$diff_hits"; printf '%s\n' "$untracked_name_hits"; printf '%s\n' "$untracked_content_hits"; } \
      | sort | hash_stdin 2>/dev/null
  )"
  if [ -n "$sentinel" ] && [ -n "$key" ]; then
    prev="$(cat "$sentinel" 2>/dev/null)"
    if [ "$prev" = "$key" ]; then
      exit 0   # same risky diff-state already nudged; nothing new to review
    fi
    printf '%s' "$key" > "$sentinel" 2>/dev/null
  fi
  printf '%s\n' '{"decision":"block","reason":"Risky logic (auth/billing/concurrency) changed without an independent delegate review. Run an independent pass on the diff with a different model (e.g. Codex via mcp__codex__codex or codex exec, or a verified Antigravity path), then finish. If you already reviewed it with a delegate this session, say so and stop."}'
else
  clear_sentinel
fi
exit 0
