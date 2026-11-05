# Manual checks

Use these checks with normal file and session tools. Both editions contain
guidance only. Neither edition has a hook launcher;
its absence does not identify the edition or indicate a fault.
Never claim a command ran when it did not.

1. This skill loaded, so discovery works for this skill.
2. Read `version` from `<plugin root>/.codex-plugin/plugin.json`, or from
   `<plugin root>/.claude-plugin/plugin.json` when it exists. If neither exists,
   report the version as unknown.
3. List sibling directories under `<plugin root>/skills/` that contain a
   `SKILL.md`. Name any directory without one.
4. On Claude Code, inspect the effective `outputStyle` setting. The optional
   plugin style resolves as `megapowers:Megapowers`. The directory edition
   omits this Claude Code component.
5. Inspect successful body-load records in available session traces. Say when
   traces or counts are unavailable; do not infer them from skill listings.

Report any inaccessible input as unchecked. Never infer a successful check
from the presence of guidance.

Compare the installed manifest with the active marketplace registration's
copy. Discover its configured root first. Default Claude copies are beneath
`~/.claude/plugins/marketplaces/megapowers/plugins/megapowers/`; default Codex
copies are beneath `~/.codex/.tmp/marketplaces/megapowers/plugins/megapowers/`.
Respect configured roots instead of assuming defaults. A missing copy is an
unchecked comparison. Read the optional agent registry's `expires_at` and
compare it with today's date. An expired or unreadable registry supplies no
current binding; use the harness defaults and report the limit.
