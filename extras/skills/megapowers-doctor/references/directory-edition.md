# Manual checks

Use these checks when the local shell or Go is unavailable. The directory
edition bundles the same doctor script. Neither edition has a hook launcher;
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

Report shell and Go checks as skipped when unavailable. Skills with bundled
scripts may require them; loading guidance does not.
