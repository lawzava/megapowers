# Manual checks

Use your normal file tools for these checks. Neither edition has a hook launcher;
its absence does not identify the edition or indicate a fault.
Never claim a command ran when it did not.

1. This skill loaded, so discovery works for this skill.
2. Read `version` from `<plugin root>/.claude-plugin/plugin.json`, falling back
   to `<plugin root>/.codex-plugin/plugin.json`. If neither is readable,
   report the version as unknown.
3. Read `<plugin root>/skills/catalog.json` and confirm each listed skill has
   a readable `<plugin root>/skills/<name>/SKILL.md`. The directory edition
   omits the catalog; list sibling skill directories instead. Name any directory
   without a `SKILL.md`.
4. Read the shipped registry template at
   `<plugin root>/skills/orchestrating/assets/agent-capabilities.md`.
5. On Claude Code, check the optional output style with `/config` or ask the
   user which style is selected. It resolves as `megapowers:Megapowers`.
   The directory edition omits this Claude Code component.
6. Ask for a skill explicitly and confirm its body loads.

Report checks as skipped when the required tools are unavailable. Skills with
bundled scripts may require a shell and Go; loading this guidance does not.
