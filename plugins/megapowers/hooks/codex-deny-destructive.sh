#!/usr/bin/env bash
# Codex adapter: pass through a destructive deny; empty guard output means allow.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
guard="$here/deny-destructive.sh"
[ -x "$guard" ] || {
  printf 'megapowers Codex guard: cannot evaluate missing destructive guard\n' >&2
  exit 1
}

input="$(cat)" || {
  printf 'megapowers Codex guard: cannot evaluate input\n' >&2
  exit 1
}
if ! out="$(printf '%s' "$input" | "$guard")"; then
  printf 'megapowers Codex guard: cannot evaluate destructive command\n' >&2
  exit 1
fi
[ -n "$out" ] || exit 0

command -v jq >/dev/null 2>&1 || {
  printf 'megapowers Codex guard: cannot evaluate decision without jq\n' >&2
  exit 1
}
if ! decision="$(printf '%s' "$out" | jq -er '.hookSpecificOutput.permissionDecision' 2>/dev/null)"; then
  printf 'megapowers Codex guard: cannot evaluate destructive decision\n' >&2
  exit 1
fi
if [ "$decision" != "deny" ]; then
  printf 'megapowers Codex guard: unsupported decision: %s\n' "$decision" >&2
  exit 1
fi
printf '%s' "$out"
