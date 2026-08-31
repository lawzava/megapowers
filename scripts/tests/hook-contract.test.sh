#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
guard="$ROOT/plugins/megapowers/hooks/deny-destructive.sh"
wrapper="$ROOT/plugins/megapowers/hooks/run-hook.cmd"
manifest="$ROOT/plugins/megapowers/hooks/hooks.json"

fail() {
  printf 'hook contract: %s\n' "$*" >&2
  exit 1
}

if rg -ni 'effect-broker|ask[ -]tier|decision=.*ask|permissionDecision[^[:alnum:]]*:[^[:alnum:]]*ask' "$guard"; then
  fail 'destructive guard retains a dormant confirmation tier'
fi

set +e
missing_out="$(bash "$wrapper" missing-hook.sh 2>&1)"
missing_rc=$?
set -e
[[ $missing_rc -ne 0 ]] || fail 'Unix wrapper silently accepted a missing hook target'
[[ $missing_out == *'cannot run'* ]] || fail 'Unix wrapper did not explain missing hook target'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
plugin_root="$tmp/plugin root"
ln -s "$ROOT/plugins/megapowers" "$plugin_root"
session_command="$(jq -er '.hooks.SessionStart[0].hooks[0].command' "$manifest")"
if ! printf '%s' '{"hook_event_name":"SessionStart","source":"startup"}' |
  CLAUDE_PLUGIN_ROOT="$plugin_root" PLUGIN_ROOT="$plugin_root" bash -c "$session_command" >/dev/null; then
  fail 'hook command cannot execute when the plugin root contains spaces'
fi

printf 'hook contract: ok\n'
