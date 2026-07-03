#!/bin/bash
# PostToolUse(Write|Edit): format the touched file in place. Runs synchronously:
# it must finish before the tool result returns, or an async format can race a
# follow-up Edit and silently clobber it (or invalidate the model's old_string).
# It only rewrites the single file just written and is fast, so the cost is small.
f=$(jq -r '.tool_input.file_path // empty')
[ -f "$f" ] || exit 0
case "$f" in
  *.go)
    if command -v goimports >/dev/null 2>&1; then goimports -w "$f"
    else gofmt -w "$f"; fi 2>/dev/null ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css|*.scss|*.md|*.yaml|*.yml)
    # use the project's own prettier if installed; otherwise do nothing
    (cd "$(dirname "$f")" && npx --no-install prettier --write "$f") >/dev/null 2>&1 ;;
esac
exit 0
