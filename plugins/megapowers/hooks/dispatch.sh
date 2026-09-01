#!/usr/bin/env bash
# dispatch.sh <claude-target> <codex-target> - native harness hook selector.
# Codex sets PLUGIN_ROOT; MEGAPOWERS_HARNESS=claude|codex overrides that
# heuristic when a harness stops exporting it. Any dispatch failure is visible
# and nonzero so a broken safety hook cannot silently look healthy.
set -u
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="$(cat)" || { printf 'megapowers hook: cannot evaluate input\n' >&2; exit 1; }
case "${MEGAPOWERS_HARNESS:-}" in
  claude) target="${1:-}" ;;
  codex) target="${2:-}" ;;
  "") if [ -n "${PLUGIN_ROOT:-}" ]; then target="${2:-}"; else target="${1:-}"; fi ;;
  *) printf 'megapowers hook: cannot evaluate unknown MEGAPOWERS_HARNESS=%s\n' "$MEGAPOWERS_HARNESS" >&2; exit 1 ;;
esac
if [ -z "$target" ] || [ ! -x "$here/$target" ]; then
  printf 'megapowers hook: cannot evaluate %s\n' "${target:-missing target}" >&2
  exit 1
fi
if ! printf '%s' "$input" | "$here/$target"; then
  printf 'megapowers hook: cannot evaluate %s\n' "$target" >&2
  exit 1
fi
