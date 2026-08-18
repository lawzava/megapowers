#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADAPTER="$HERE/../codex-output-style.sh"
PLUGIN_ROOT_PATH="$HERE/../.."

fail() {
  printf 'codex output style: %s\n' "$*" >&2
  exit 1
}

[[ -x $ADAPTER ]] || fail 'adapter is missing or not executable'

event='{"hook_event_name":"SessionStart","source":"startup"}'
codex_out="$(printf '%s' "$event" | PLUGIN_ROOT="$PLUGIN_ROOT_PATH" "$ADAPTER")"

grep -qF 'ASD-STE100-inspired principles' <<< "$codex_out" ||
  fail 'Codex context omits the shared clarity baseline'
grep -qF 'Default to 100 prose words or fewer.' <<< "$codex_out" ||
  fail 'Codex context omits the default prose budget'
grep -qF 'Do not exceed 250 prose words' <<< "$codex_out" ||
  fail 'Codex context omits the prose ceiling'
grep -qF 'Preserve exact identifiers, commands, numbers, caveats, decisions, and material uncertainty.' <<< "$codex_out" ||
  fail 'Codex context can discard load-bearing facts'
if grep -qF 'force-for-plugin:' <<< "$codex_out"; then
  fail 'Codex context exposes Claude frontmatter'
fi

claude_out="$(printf '%s' "$event" | env -u PLUGIN_ROOT "$ADAPTER")"
[[ -z $claude_out ]] || fail 'Claude must keep using its native output-style component'

printf 'codex output style: ok\n'
