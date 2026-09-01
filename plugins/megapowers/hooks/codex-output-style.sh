#!/usr/bin/env bash
# Codex adapter: add the shared output-style body as startup developer context.
# Claude Code uses the native output-styles component and does not set
# PLUGIN_ROOT, so the shared hooks file remains silent there.
set -u

case "${MEGAPOWERS_HARNESS:-}" in
  codex) ;;
  claude) exit 0 ;;
  *) [ -n "${PLUGIN_ROOT:-}" ] || exit 0 ;;
esac

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

# Codex lists skill trigger descriptions but nothing makes it load a body;
# Claude Code needs no equivalent because its native Skill tool carries this
# contract. Real Codex sessions read SKILL.md in 27/32 sessions (2026-09-01
# audit); this reminder keeps that behavior explicit.
cat <<'SKILLS'

# Skills

The available-skills catalog lists trigger descriptions, not skill content.
When a task matches a skill's description, load that skill's SKILL.md with
the skills tool or a direct file read before you act on the task, and follow
what it says. Do not claim a skill without loading it.
SKILLS
