# Directory edition

A plugin root with no `hooks/run-hook.cmd` is the directory edition: skills
only, with no hooks, no output style, no destructive-command guard, no startup
reminder, and no deterministic doctor runner. Report that as the diagnosis of
"hooks missing", not as a fault. There is nothing to fix and no `WARN` line to
quote.

Check only what this skill can, and say which checks ran. Never claim a check
ran that did not.

1. This skill loaded. It is running, so skill discovery works.
2. Plugin version: read `version` from `<plugin root>/.codex-plugin/plugin.json`,
   or from `<plugin root>/.claude-plugin/plugin.json` when that file exists.
   If neither file exists, say the version is unknown.
3. Sibling skills: list the directories under `<plugin root>/skills/` that
   each contain a `SKILL.md`. Name any directory without one.
4. Local shell and Go: only if the harness has a shell, run `go version` and
   report the output. Otherwise write one line: "No local shell; shell and Go
   checks skipped."
5. Skill loads: the runner's `skill loads` line does not exist here. Say that
   no count is available; do not infer one.

For the destructive-command guard, the output style, the startup reminder, and
the hook doctor, tell the user to install the full plugin for Claude Code or
Codex. The marketplace commands are in the
[install guide](https://github.com/lawzava/megapowers/blob/main/docs/install.md)
(`docs/install.md` in https://github.com/lawzava/megapowers), under the Claude
Code and Codex headings. The skills work without them.
