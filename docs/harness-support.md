# Harness support

Last reviewed: 2026-08-16.

Current stable Claude Code and Codex are the only supported harnesses. Portable
skills may load elsewhere, but this repository does not test or document those
environments.

| Capability | Claude Code | Codex |
|---|---|---|
| Marketplace metadata | `.claude-plugin/marketplace.json` | `.agents/plugins/marketplace.json` |
| Plugin manifest | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` |
| Ten task skills | Supported | Supported |
| Destructive-command guard | High-confidence denies only | High-confidence denies only |
| Native agents and parallel work | Use harness-native features | Use harness-native features |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Claude or Codex CLI, different from author | Claude or Codex CLI, different from author |
| Credentialed installed A/B | Release evidence arm | Release evidence arm |
| Exact-tag install smoke | Post-publish oracle | Post-publish oracle |

## Shared contract

Skills use portable `name` and `description` frontmatter. Full bodies remain
harness-neutral except `independent-review`, whose purpose is to select one of
the two supported provider CLIs explicitly.

Repository instructions, existing code, and configured project tools are
authoritative. megapowers supplies workflow defaults only where the repository
is silent.

The plugin does not configure models, agent roles, context budgets, sandbox
rules, or permission policy. Those are harness and repository concerns. This
avoids pinning quickly changing native surfaces into plugin guidance.

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
