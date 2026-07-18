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

# delegate-nudge runs on BOTH harnesses (passed as both targets in
# hooks.json); a Codex session with an unreviewed risky diff must still get
# the nudge through the dispatcher.
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
( cd "$TMP" && git init -q && git config user.email t@t && git config user.name t \
    && git config commit.gpgsign false && git commit -q --allow-empty -m init \
    && printf 'func handler() { billing() }\n' > svc.go )
: > "$TMP/tr.jsonl"
out="$(cd "$TMP" && printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TMP/tr.jsonl" \
  | PLUGIN_ROOT="$here/.." "$dispatch" delegate-nudge.sh delegate-nudge.sh)"
if ! printf '%s' "$out" | grep -q '"decision":"block"'; then
  echo "  FAIL Codex path must still run the delegate nudge"
  exit 1
fi
echo "2 passed, 0 failed"
