#!/usr/bin/env bash
# Stop-hook nudge: if risky logic changed this session WITHOUT an independent
# delegate review, ask for one before finishing. Deterministic, cheap, no model
# call. Fail-open: any error/uncertainty -> allow (exit 0). Self-suppressing via
# stop_hook_active + the delegate-usage check. Depends only on jq/git/grep.
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

# Only relevant inside a git repo with pending changes.
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
diff="$(git diff HEAD 2>/dev/null)"
# git diff HEAD misses untracked new files — a new auth/billing file would escape.
# Fold in untracked filenames (respecting .gitignore) and their contents.
untracked="$(git ls-files --others --exclude-standard 2>/dev/null)"
[ -n "$diff" ] || [ -n "$untracked" ] || exit 0

# 'authn|authz|authenticat|authoriz' instead of bare 'auth' so it fires on real
# auth code but not on the word 'author'/'authored' (commit trailers, docs).
risky='authn|authz|authenticat|authoriz|oauth|jwt|saml|passwd|password|billing|payment|invoice|subscription|stripe|webhook|mutex|goroutine|semaphore|deadlock|concurren'
hit=0
printf '%s' "$diff" | grep -qiE "$risky" && hit=1
if [ "$hit" -eq 0 ] && [ -n "$untracked" ]; then
  printf '%s' "$untracked" | grep -qiE "$risky" && hit=1   # filename signal
  if [ "$hit" -eq 0 ]; then
    while IFS= read -r f; do
      [ -f "$f" ] || continue
      grep -qiE "$risky" -- "$f" 2>/dev/null && { hit=1; break; }
    done <<< "$untracked"
  fi
fi

# Risky-logic signal (paths or content). Bias to allow: high-signal terms only.
if [ "$hit" -eq 1 ]; then
  printf '%s\n' '{"decision":"block","reason":"Risky logic (auth/billing/concurrency) changed without an independent delegate review. Run an independent pass on the diff with a different model (e.g. Codex via mcp__codex__codex or codex exec, or a verified Antigravity path), then finish. If you already reviewed it with a delegate this session, say so and stop."}'
fi
exit 0
