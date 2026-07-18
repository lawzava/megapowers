#!/usr/bin/env bash
# dispatch.sh <claude-target> [<codex-target>] — cross-harness hook selector.
# Codex sets PLUGIN_ROOT: run <codex-target>, or no-op when omitted (the hook
# is Claude Code only). Claude Code: run <claude-target>. Fail open on any
# error. Identical copies ship in every plugin's hooks/ because plugins cannot
# locate each other at runtime; validate.sh pins the twins byte-identical.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="$(cat 2>/dev/null || true)"
if [ -n "${PLUGIN_ROOT:-}" ]; then target="${2:-}"; else target="${1:-}"; fi
[ -n "$target" ] && [ -x "$here/$target" ] || exit 0
printf '%s' "$input" | "$here/$target" 2>/dev/null || true
exit 0
