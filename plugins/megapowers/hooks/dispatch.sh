#!/usr/bin/env bash
# dispatch.sh <claude-target> <codex-target> - native harness hook selector.
# Codex sets PLUGIN_ROOT. Any dispatch failure is visible and nonzero so a
# broken safety hook cannot silently look healthy.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="$(cat)" || { printf 'megapowers hook: cannot evaluate input\n' >&2; exit 1; }
if [ -n "${PLUGIN_ROOT:-}" ]; then target="${2:-}"; else target="${1:-}"; fi
if [ -z "$target" ] || [ ! -x "$here/$target" ]; then
  printf 'megapowers hook: cannot evaluate %s\n' "${target:-missing target}" >&2
  exit 1
fi
if ! printf '%s' "$input" | "$here/$target"; then
  printf 'megapowers hook: cannot evaluate %s\n' "$target" >&2
  exit 1
fi
