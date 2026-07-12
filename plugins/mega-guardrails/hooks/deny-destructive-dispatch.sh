#!/usr/bin/env bash
# Cross-harness destructive-command dispatcher. Codex needs the adapter that
# drops its unsupported ask decision; Claude Code consumes the full guard.
set -u

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="$(cat 2>/dev/null || true)"
if [ -n "${PLUGIN_ROOT:-}" ]; then
  target="$here/codex-deny-destructive.sh"
else
  target="$here/deny-destructive.sh"
fi
[ -x "$target" ] || exit 0
printf '%s' "$input" | "$target" 2>/dev/null || true
exit 0
