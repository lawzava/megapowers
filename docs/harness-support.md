# Harness support

Last reviewed: 2026-08-19.

Current stable Claude Code and Codex are the only supported harnesses. Portable
skills may load elsewhere, but this repository does not test or document those
environments.

| Capability | Claude Code | Codex |
|---|---|---|
| Marketplace metadata | `.claude-plugin/marketplace.json` | `.agents/plugins/marketplace.json` |
| Plugin manifest | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` |
| Eleven task skills | Supported | Supported |
| Default communication style | Forced plugin output style | Trusted startup hook |
| Destructive-command guard | High-confidence denies only | High-confidence denies only |
| Native agents and parallel work | Use harness-native features | Use harness-native features |
| Personal capability registry | Advisory, read on demand | Advisory, read on demand |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Claude or Codex CLI, different from author | Claude or Codex CLI, different from author |
| Credentialed installed A/B | Optional diagnostic arm | Optional diagnostic arm |
| Exact-tag install smoke | Post-publish oracle | Post-publish oracle |

## Shared contract

Skills use portable `name` and `description` frontmatter. `SKILL.md` bodies
remain harness-neutral except `independent-review`, whose purpose is to select
one of the two supported provider CLIs explicitly. Channel-specific upgrade
commands live in a directly linked reference loaded only after channel
detection.

Repository instructions, existing code, and configured project tools are
authoritative. megapowers supplies workflow defaults only where the repository
is silent.

## Communication style

Both harness adapters use `output-styles/megapowers.md` as the source of truth.
Claude Code loads it natively with `force-for-plugin: true`. It preserves the
built-in coding instructions, but it overrides a selected Claude Code output
style while megapowers is enabled.

Codex has no output-style component. Its bundled `SessionStart` hook adds the
same body as developer context on startup, resume, clear, and compaction. Codex
requires the user to review and trust non-managed plugin hooks before it runs.
The adapter does not edit `~/.codex/config.toml` or `AGENTS.md`.

Both paths affect the main conversation only. They do not change ordinary
subagent prompts or hide tool results. `humanizing-prose` remains available for
tasks that need more specific editing judgment.

The plugin ships no model or role configuration and does not create model,
agent, context, sandbox, or permission availability. An optional user-owned
registry can describe relative capabilities and native bindings for the lead's
selection. It is advisory input: harness and repository policy still decide
what exists and what is authorized.

## Hook behavior

Both adapters forward high-confidence denials only. Reversible risk remains
with each harness's native permission system.

On either harness the hook is a narrow accident tripwire. It does not replace
the sandbox, OS permissions, review, or `safe-effects` approval for external
mutations.

## Freshness policy

A support claim must be checked against a current CLI, the checked-in native
manifest, a fresh config home, and the exact plugin revision. Structural
validation runs without credentials. Behavioral claims require the
installed-plugin A/B protocol for both harnesses. See
[advanced/evals.md](./advanced/evals.md).
