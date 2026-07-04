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
    # Use a real prettier if one is resolvable, otherwise do nothing. Resolve it
    # ourselves and invoke it directly: never `npx`, whose node startup is ~0.6-2s even
    # when it resolves nothing, paid on every md/json/yaml edit in a non-JS project.
    prettier_bin=""
    d="$(cd "$(dirname "$f")" 2>/dev/null && pwd)"
    while [ -n "$d" ]; do
      if [ -x "$d/node_modules/.bin/prettier" ]; then prettier_bin="$d/node_modules/.bin/prettier"; break; fi
      [ "$d" = "/" ] && break
      d="$(dirname "$d")"
    done
    [ -z "$prettier_bin" ] && command -v prettier >/dev/null 2>&1 && prettier_bin="prettier"
    [ -n "$prettier_bin" ] && "$prettier_bin" --write "$f" >/dev/null 2>&1 ;;
esac
exit 0
