# megapowers plugin

One native-first workflow plugin for current Claude Code and Codex.

## Contents

- `skills/`: task-level skills for orchestration, structured interviews,
  research, design, implementation with language-specific code judgment,
  debugging, verification, effects, durable runs, independent review, memory
  hygiene, MCP setup, prose, safe upgrades, and writing agent instructions.
  `skills/catalog.json` records stable or experimental maturity
  outside portable skill frontmatter.
- `output-styles/`: the shared source for direct, concise technical replies.
- `hooks/`: a Codex startup hook for the shared style and one
  destructive-command tripwire with Claude Code and Codex adapters.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: native plugin
  metadata.

The plugin uses native harness planning, agents, goals, permissions, memory,
worktrees, and browser tools. It ships no model router or orchestration daemon.

Select `Megapowers` in Claude Code's `/config` output-style picker. It preserves
the built-in coding instructions and respects another selected style.
The trusted Codex startup hook adds the same style as developer
context without changing user config. Codex requires review and trust before it
runs plugin hooks. Set `MEGAPOWERS_OUTPUT_STYLE=off` before launching Codex to
omit the style. Hooks use Go with a locally cached executable. Both style
adapters apply only to the main conversation; ordinary
subagents do not inherit them.

## Install

Add the repository marketplace, then install `megapowers@megapowers`. The
complete commands and fresh-session checks are in
[the installation guide](https://github.com/lawzava/megapowers/blob/main/docs/install.md).

## Security

The hook catches a narrow set of obvious catastrophic shell commands. It is not
a sandbox. The independent-review skill sends only an explicit file or immutable
commit range after a disclosure and approval step. Read the
[security policy](https://github.com/lawzava/megapowers/blob/main/SECURITY.md)
before enabling either path.

## Attribution

The workflow descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License. See the
[full attribution](https://github.com/lawzava/megapowers/blob/main/ATTRIBUTION.md).
