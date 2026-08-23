# megapowers

[![CI](https://github.com/lawzava/megapowers/actions/workflows/ci.yml/badge.svg)](https://github.com/lawzava/megapowers/actions/workflows/ci.yml)
[![Latest tag](https://img.shields.io/github/v/tag/lawzava/megapowers?label=release)](https://github.com/lawzava/megapowers/tags)
[![License](https://img.shields.io/github/license/lawzava/megapowers)](./LICENSE)

megapowers is exactly one plugin for current Claude Code and Codex. It adds
thirteen task-level skills, one shared default communication style, and a small
destructive-command tripwire. It does not replace native agents, plans, goals,
permissions, worktrees, memory, or browser tools.

The plugin is deliberately small. There is no model router, session catalog,
formatter, status line, custom scheduler, hosted service, or compatibility
layer for other harnesses.

## Install

Claude Code:

```bash
claude plugin marketplace add lawzava/megapowers
claude plugin install megapowers@megapowers
```

Codex:

```bash
codex plugin marketplace add lawzava/megapowers
codex plugin add megapowers@megapowers
```

Start a fresh session after installation. Full update, verification, pinning,
and uninstall instructions are in [docs/install.md](./docs/install.md).

## The workflow

Use the smallest skill that matches the task. Skill descriptions stay visible;
the full body loads only when selected.

| Need | Skill |
|---|---|
| Choose between inline work, native agents, a durable run, or review | `orchestrating` |
| Resolve requirements and write an executable plan | `design-and-plan` |
| Change behavior with a verified red, green, refactor loop | `test-first-implementation` |
| Find a root cause before fixing a bug or flaky test | `systematic-debugging` |
| Prove completion, handoff, commit, merge, or release claims | `verify-and-finish` |
| Preview and authorize a real-world side effect | `safe-effects` |
| Preserve honest state across long or interrupted work | `autonomous-run` |
| Send an explicit artifact to a different provider for adversarial review | `independent-review` |
| Write direct human-facing prose without dropping or inventing facts | `humanizing-prose` |
| Make code-quality judgments from repository context and stable language guidance | `code-quality` |
| Upgrade Megapowers without changing its source, scope, pins, or local edits | `upgrading-megapowers` |
| Research current facts, historical rationale, or contested evidence | `evidence-research` |
| Configure, debug, or verify an MCP server connection | `mcp-setup` |

Repository instructions, existing code, and configured project tools remain
authoritative. See [docs/orchestration.md](./docs/orchestration.md) for the
native-first task shapes.

For non-trivial delegation, `orchestrating` can read one optional personal
capability registry from `~/.config/megapowers/agent-capabilities.md`. The file
stays outside the plugin and supplies advisory choices only; it creates no
agent access or authority.

## What installs

- Thirteen portable `SKILL.md` directories. `skills/catalog.json` marks
  maturity; `evidence-research` and `mcp-setup` start as experimental.
- One shared style for direct, concise technical replies: a forced Claude Code
  output style and a trusted Codex startup hook.
- One `PreToolUse` shell hook for obvious catastrophic commands.
- One Go standard-library independent-review tool, loaded only with that skill.

Claude Code applies the style while the plugin is enabled. It preserves Claude
Code's built-in software-engineering instructions, but the plugin default
overrides a manually selected output style. Codex adds the same style as
developer context at session startup and after compaction once the user trusts
the bundled hooks. Neither adapter changes global user configuration.

Claude Code and Codex receive the same high-confidence denials. Reversible risk
remains with each harness's own permission system. The hook is an accident
tripwire, not a sandbox. Read
[SECURITY.md](./SECURITY.md) before trusting the plugin.

## Evidence

Three evidence classes are kept separate:

1. `scripts/validate.sh` and `evals/run-all.sh` run bounded deterministic
   regressions and runner selftests. They prove repository mechanics, not agent
   quality.
2. The optional installed-plugin A/B study compares this checkout with an empty
   control under Claude Code and Codex through a hash-pinned isolation broker.
   It reports treatment reliability and paired control outcomes; it does not
   gate releases or claim that the plugin improves general model capability.
3. PR replay uses hidden correctness tests against pinned historical changes.
   It is report-only until repeated real runs support a release threshold.

No current thirteen-skill behavioral result is claimed from selftests. Historical
measurements, including null results and their limitations, remain in
[evals/RESULTS.md](./evals/RESULTS.md). Current protocols and gates are in
[evals/README.md](./evals/README.md) and
[docs/advanced/evals.md](./docs/advanced/evals.md).

## Limits

- Skills are instructions, not enforcement. Model and harness behavior can
  change.
- The communication style applies to the main conversation, not ordinary
  subagents, and it cannot suppress tool-result rendering. Codex skips the
  adapter until the user trusts the plugin hooks.
- The destructive guard matches a narrow set of command strings. OS sandboxing
  and least privilege remain the real boundary.
- Independent review discloses approved source content to the selected provider.
  It rejects common secret patterns, not every possible secret.
- Real-agent studies require credentials, spend, and a reviewed broker that
  keeps credentials and hidden state outside the actor's OS boundary. Their
  selftests do not substitute for a credentialed run.
- Only current Claude Code and Codex are supported. Exact structural and
  behavioral evidence boundaries are in
  [docs/harness-support.md](./docs/harness-support.md).

## Develop

```bash
scripts/validate.sh
bash evals/run-all.sh --json results.jsonl
```

The deterministic suite fails on malformed, incomplete, indeterminate,
timed-out, or harness-error results. Contributions that change behavioral
guidance need deterministic regression coverage. Credentialed installed-plugin
A/B remains optional diagnostic evidence. See [CONTRIBUTING.md](./CONTRIBUTING.md).
The repository-local verification-map pilot is documented in
[docs/advanced/verification-maps.md](./docs/advanced/verification-maps.md).

## License and origin

megapowers is MIT-licensed. It began from Superpowers by Jesse Vincent and
retains other source credits in [ATTRIBUTION.md](./ATTRIBUTION.md). It is not an
Anthropic or OpenAI product.
