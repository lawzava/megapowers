# OpenCode Tool Mapping

OpenCode reads project instructions from `AGENTS.md` and can load portable
skills from `.opencode/skills/<name>/SKILL.md`, `.agents/skills/<name>/SKILL.md`,
or Claude-compatible skill paths.

## Skills

Use one canonical skill directory per skill. Do not paste every skill body into
`opencode.json` instructions; that bypasses progressive disclosure and wastes
context. Copy or symlink only the skills needed by the project.

OpenCode recognizes `name` and `description` frontmatter and ignores unknown
fields. Keep frontmatter simple for portability across Codex, Claude Code, and
OpenCode.

## Agents

OpenCode has primary agents and subagents. Use the built-in planning or
read-only agents for analysis, and use subagents for independent research or
implementation tracks. Keep single-writer discipline when subagents can edit.

## Plugins

OpenCode plugins are JavaScript or TypeScript modules loaded from
`.opencode/plugins/` or package configuration. The Claude Code shell hooks in
this repository are not OpenCode plugins; port them explicitly before enabling
equivalent behavior.
