# megapowers plugin

One native-first workflow plugin for current Claude Code and Codex: 16 task
skills, one concise output style, a destructive-command tripwire, and a doctor.

## Contents

- `skills/`: task-level skills for orchestration, structured interviews,
  research, design, implementation with language-specific code judgment,
  debugging, verification, effects, durable runs, independent review, memory
  hygiene, MCP setup, prose, safe upgrades, writing agent instructions, and
  self-diagnosis (`megapowers-doctor`). `skills/catalog.json` records stable
  or experimental maturity outside portable skill frontmatter.
- `output-styles/`: the shared source for direct, concise technical replies.
- `hooks/`: Go hooks with one entrypoint, `run-hook.cmd`. `SessionStart` adds
  a skill-loading reminder and, on Codex, the style. `SubagentStart` adds the
  same reminder and a compact report contract for subagents. `PreToolUse`
  denies a narrow set of catastrophic shell commands and returns a
  non-blocking reminder to load `verify-and-finish` before commit, push, and
  PR commands or `safe-effects` before publish and deploy commands. The same
  runner's `doctor` command backs `megapowers-doctor`.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: native plugin
  metadata.

The plugin uses native harness planning, agents, goals, permissions, memory,
worktrees, and browser tools. It ships no model router, orchestration daemon,
or vendor choice.

Claude Code: the style is off until selected. Choose `megapowers:Megapowers`
in the `/config` output-style picker or run
`/output-style megapowers:Megapowers`; the bare value `Megapowers` does not
resolve. The style preserves the built-in coding instructions and respects
another selected style. Codex: the trusted startup hook adds the same style as
developer context without changing user config; Codex requires review and
trust before it runs plugin hooks, and asks again when a release changes them.
Set `MEGAPOWERS_OUTPUT_STYLE=off` before launching Codex to omit the style.
Hooks compile once with the local Go toolchain (1.25 or newer) into a cached
executable; the plugin contains no prebuilt binaries.

## Install

Add the repository marketplace at the `release` branch, then install
`megapowers@megapowers`. The complete commands, fresh-session checks, and
diagnosis steps are in
[the installation guide](https://github.com/lawzava/megapowers/blob/main/docs/install.md).

## Security

The guard catches a narrow set of obvious catastrophic shell commands, including
compound forms and absolute-path wrappers. It is not a sandbox. The
independent-review skill sends only an explicit file or immutable commit range
after a disclosure and approval step. No hook, tool, or skill opens a network
connection on its own. Read the
[security policy](https://github.com/lawzava/megapowers/blob/main/SECURITY.md)
before enabling either path.

## Attribution

The workflow descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License. See the
[full attribution](https://github.com/lawzava/megapowers/blob/main/ATTRIBUTION.md).
