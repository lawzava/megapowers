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
diff="$(git diff HEAD --binary --no-ext-diff 2>/dev/null)"
untracked="$(git ls-files --others --exclude-standard 2>/dev/null)"
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
jq -nc --arg launcher "$launcher" \
  '{decision:"block", reason:("Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run " + $launcher + " --role verify --author-vendor <artifact-author-vendor> --artifact worktree --claim <claim>. The launcher resolves a different-vendor reviewer and binds its verdict to the complete pending tree. Unrelated delegate calls and stale receipts do not satisfy this gate.")}'
exit 0
