# megapowers plugin

One native-first workflow plugin for current Claude Code and Codex.

## Contents

- `skills/`: eleven task-level skills for orchestration, design, implementation,
  debugging, verification, effects, durable runs, independent review, prose,
  code quality, and safe upgrades.
- `hooks/`: one destructive-command tripwire, with Claude Code and Codex
  adapters.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: native plugin
  metadata.

The plugin uses native harness planning, agents, goals, permissions, memory,
worktrees, and browser tools. It ships no model router or orchestration daemon.

## Install

Add the repository marketplace, then install `megapowers@megapowers`. The
complete commands and fresh-session checks are in
[docs/install.md](../../docs/install.md).

## Security

The hook catches a narrow set of obvious catastrophic shell commands. It is not
a sandbox. The independent-review skill sends only an explicit file or immutable
commit range after a disclosure and approval step. Read
[SECURITY.md](../../SECURITY.md) before enabling either path.

## Attribution

The workflow descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License. Full notices are in [ATTRIBUTION.md](../../ATTRIBUTION.md).
