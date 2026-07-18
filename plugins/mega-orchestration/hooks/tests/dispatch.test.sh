#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatch="$here/dispatch.sh"

echo "== orchestration dispatcher tests =="
out="$(printf '%s' '{"stop_hook_active":false}' | PLUGIN_ROOT="$here/.." "$dispatch" run-loop.sh)"
if [ -n "$out" ]; then
  echo "  FAIL Codex path must skip the Claude autonomous loop"
  exit 1
fi
out="$(printf '%s' '{"stop_hook_active":false}' | PLUGIN_ROOT="$here/.." "$dispatch" delegate-nudge.sh)"
if [ -n "$out" ]; then
  echo "  FAIL Codex path must skip the Claude delegate nudge"
  exit 1
fi
echo "2 passed, 0 failed"
