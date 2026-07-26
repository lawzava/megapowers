#!/usr/bin/env bash
# Stop-hook backstop for risky pending changes. Only a current, independent,
# launcher-generated approval receipt suppresses the nudge.
set -u
input="$(cat)"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=stop-context.sh
. "$here/stop-context.sh"

command -v jq >/dev/null 2>&1 || exit 0
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0
stop_context_is_exempt "$input" && exit 0

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
# The gate is about changed logic, so it reads code paths only. Prose names these
# categories constantly: a checklist that lists them, or a template telling an
# agent to route such work for review, is not a change to that work. Scanning
# documentation fired the gate on doc-only edits.
#
# This file and its tests are excluded for the same reason, one step further in:
# they must contain the risky keyword list verbatim (the pattern below, the block
# message that names the categories, and fixtures like `billing()` that prove the
# gate fires). Without this, editing the guard always trips the guard, and the
# resulting review request cites its own warning text as the risky change. The
# exclusion is deliberately narrow: sibling hooks stay scanned, so a real logic
# change to deny-destructive.sh is still gated, and it only ever matches inside a
# checkout of this repository.
prose=(':(top,exclude,glob)**/*.md' ':(top,exclude,glob)**/*.mdx'
       ':(top,exclude,glob)**/*.markdown' ':(top,exclude,glob)**/*.rst'
       ':(top,exclude,glob)**/mega-orchestration/hooks/delegate-nudge.sh'
       ':(top,exclude,glob)**/mega-orchestration/hooks/tests/**')
diff="$(git diff HEAD --binary --no-ext-diff -- ':/' "${prose[@]}" 2>/dev/null)"
untracked="$(git ls-files --others --exclude-standard -- ':/' "${prose[@]}" 2>/dev/null)"
[ -n "$diff" ] || [ -n "$untracked" ] || exit 0

risky='authn|authz|authenticat|authoriz|oauth|jwt|saml|passwd|password|billing|payment|invoice|subscription|stripe|webhook|mutex|goroutine|semaphore|deadlock|concurren'
hit=0
printf '%s' "$diff" | grep -qiE "$risky" && hit=1
printf '%s' "$untracked" | grep -qiE "$risky" && hit=1
if [ -n "$untracked" ]; then
  scan_files=()
  while IFS= read -r f; do [ -f "$f" ] && scan_files+=("$f"); done < <(printf '%s\n' "$untracked" | head -50)
  if [ "${#scan_files[@]}" -gt 0 ] && grep -qiE "$risky" -- "${scan_files[@]}" 2>/dev/null; then hit=1; fi
fi
[ "$hit" -eq 1 ] || exit 0

diff_id_tool="$here/../skills/multi-agent-delegation/scripts/review-diff-id"
receipt="$(git rev-parse --git-path megapowers-review-receipt.json 2>/dev/null)"
if [ -x "$diff_id_tool" ] && [ -f "$receipt" ]; then
  current_id="$("$diff_id_tool" . 2>/dev/null)"
  if [ -n "$current_id" ] && jq -e --arg id "$current_id" '
    . as $receipt |
    .schema == "megapowers.review-receipt.v1" and
    (.role == "verify" or .role == "code_review" or .role == "visual_verify") and
    .subject.kind == "worktree-diff" and .subject.id == $id and
    .independent == true and .result.verdict == "approve" and
    (.author_vendors | type == "array" and length > 0) and
    (.reviewer.vendor | type == "string" and length > 0) and
    (all(.author_vendors[]; . != $receipt.reviewer.vendor))
  ' "$receipt" >/dev/null 2>&1; then
    exit 0
  fi
fi

launcher="$here/../skills/multi-agent-delegation/scripts/delegate-run"
resolver="$here/../skills/multi-agent-delegation/scripts/delegate-resolve"

# Cross-vendor review needs at least two reachable vendors. With one (or none),
# no --author-vendor choice can route away from the author, so instructing the
# agent to run the launcher would prescribe a command that cannot succeed: it
# would exit 3 no matter what. The risk is real either way, so the gate still
# fires; only the remedy changes to one the agent can actually carry out.
# Probe the verify ROLE, not the machine. A globally installed vendor that the
# verify chain does not route to, or that fails its capability/tier/effort/floor
# requirements, cannot serve the review; counting it would claim an independent
# pass is possible when resolution would exit 3.
#
# Default 2 (the stricter path) so a missing or failing resolver never downgrades
# the remedy. Count only when the resolver itself succeeds, and count with awk
# rather than `grep -c .`: grep exits 1 on zero matches, which would discard a
# legitimate zero and leave the default in place, sending a machine with NO
# reachable vendor down the launcher path it cannot follow.
reachable=2
if [ -x "$resolver" ]; then
  if vendor_list="$("$resolver" verify --vendors 2>/dev/null)"; then
    reachable="$(printf '%s\n' "$vendor_list" | awk 'NF{n++} END{print n+0}')"
  fi
fi

if [ "$reachable" -lt 2 ]; then
  jq -nc '{decision:"block", reason:("Risky auth, billing, payment, or concurrency logic changed, and no independent reviewer is reachable: fewer than two delegate vendors have an installed CLI, so a different-vendor review cannot be resolved on this machine. Do not silently ship it. Summarize the risky change and its blast radius for the human and get an explicit go-ahead, or install a second vendor CLI and re-run the independent pass. Say plainly that the automated cross-vendor check did not run.")}'
  exit 0
fi

jq -nc --arg launcher "$launcher" \
  '{decision:"block", reason:("Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run " + $launcher + " --role verify --author-vendor <artifact-author-vendor> --artifact worktree --claim <claim>. The launcher resolves a different-vendor reviewer and binds its verdict to the complete pending tree. Unrelated delegate calls and stale receipts do not satisfy this gate.")}'
exit 0
