#!/usr/bin/env bash
# The autonomous run loop is Claude Code only. Codex sets PLUGIN_ROOT and gets
# a no-op; its portable loop discipline remains in the autonomous-run skill.
set -u

if [ -n "${PLUGIN_ROOT:-}" ]; then
  cat >/dev/null 2>&1 || true
  exit 0
fi
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -x "$here/run-loop.sh" ] || exit 0
exec "$here/run-loop.sh"
