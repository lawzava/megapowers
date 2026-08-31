# megapowers plugin

One native-first workflow plugin for current Claude Code and Codex.

## Contents

- `skills/`: fifteen task-level skills for orchestration, structured interviews,
  research, design, implementation, debugging, verification, effects, durable
  runs, independent review, memory hygiene, MCP setup, prose, code quality, and
  safe upgrades. `skills/catalog.json` records stable or experimental maturity
  outside portable skill frontmatter.
- `output-styles/`: the shared source for direct, concise technical replies.
- `hooks/`: a Codex startup hook for the shared style and one
  destructive-command tripwire with Claude Code and Codex adapters.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: native plugin
  metadata.

The plugin uses native harness planning, agents, goals, permissions, memory,
worktrees, and browser tools. It ships no model router or orchestration daemon.

Claude Code applies the output style automatically while the plugin is enabled.
It preserves the built-in coding instructions and overrides a manually selected
output style. The trusted Codex startup hook adds the same style as developer
context without changing user config. Codex requires review and trust before it
runs plugin hooks. Both adapters apply only to the main conversation; ordinary
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
