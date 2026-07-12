#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatch="$here/run-loop-dispatch.sh"

echo "== run-loop dispatcher tests =="
out="$(printf '%s' '{"stop_hook_active":false}' | PLUGIN_ROOT="$here/.." "$dispatch")"
if [ -n "$out" ]; then
  echo "  FAIL Codex path must skip the Claude autonomous loop"
  exit 1
fi
echo "1 passed, 0 failed"
