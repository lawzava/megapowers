#!/usr/bin/env bash
# Codex adapter: add the shared output-style body as startup developer context.
# Claude Code uses the native output-styles component and does not set
# PLUGIN_ROOT, so the shared hooks file remains silent there.
set -u

[ -n "${PLUGIN_ROOT:-}" ] || exit 0

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
style="$here/../output-styles/megapowers.md"

cat >/dev/null || {
  printf 'megapowers Codex output style: cannot read hook input\n' >&2
  exit 1
}
[ -f "$style" ] || {
  printf 'megapowers Codex output style: shared style is missing\n' >&2
  exit 1
}

awk '
  NR == 1 && $0 == "---" { frontmatter = 1; next }
  frontmatter && $0 == "---" { frontmatter = 0; next }
  !frontmatter { print }
' "$style"
