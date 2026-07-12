#!/usr/bin/env bash
# Auto-format remains Claude Code only. Codex sets PLUGIN_ROOT and gets a no-op.
set -u

if [ -n "${PLUGIN_ROOT:-}" ]; then
  cat >/dev/null 2>&1 || true
  exit 0
fi
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -x "$here/auto-format.sh" ] || exit 0
exec "$here/auto-format.sh"
