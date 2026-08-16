# Contributing

megapowers accepts small, measured changes to one plugin for current Claude Code
and Codex.

## Before a pull request

Run the deterministic gate:

```bash
scripts/validate.sh
bash evals/run-all.sh --json results.jsonl
```

Then match evidence to the change:

| Change | Required evidence |
|---|---|
| Hook, tool, manifest, or runner behavior | A failing regression first, then the focused test and full deterministic gate |
| New or changed agent guidance | Baseline the target behavior, then run source-bound installed-plugin A/B under Claude Code and Codex |
| Removed or compressed guidance | Deterministic gate, plus evidence for any published behavior the removed text carried |
| Editorial text | Link, reference, and deterministic checks only |
| Eval oracle | Mutation proof that the oracle rejects a deliberately wrong artifact |

A runner selftest proves mechanics only. It is not behavioral evidence.

## Scope

- Keep the marketplace at exactly one plugin and the inventory at ten skills.
- Keep semantic skills portable. Harness-specific mechanics belong at a narrow
  adapter or documented provider boundary.
- Prefer native agents, goals, permissions, worktrees, memory, and browser tools.
- Do not add a model router, session prompt injection, scheduler, formatter,
  status line, or another harness without an explicit scope decision and fresh
  evidence.
- Add no unsourced model, benchmark, cost, context, or performance claims.

## Code and prose

- Follow repository instructions and neighboring conventions.
- New glue is Go standard library. Harness-required hook entrypoints remain
  shell.
- Add or change behavior test-first. Keep hooks bounded and directly tested.
- Human-facing prose leads with the outcome, preserves source facts, and omits
  unsupported claims and session history.
- Keep commits conventional and focused. Do not add attribution or session
  trailers.

## Evaluation

The current evidence stack is documented in
[docs/advanced/evals.md](./docs/advanced/evals.md):

1. deterministic regressions for mechanics;
2. credentialed installed-plugin A/B for release behavior;
3. report-only PR replay for real-project correctness;
4. exact-tag install smoke after publication.

Published rows must identify source, harness, CLI, model, effort, prompt,
fixture, plugin, environment, status, and artifacts. Malformed, incomplete,
indeterminate, timed-out, or harness-error data fails closed.

## Release sequence

1. Write the changelog entry and freeze the candidate revision.
2. Run deterministic validation.
3. For a behavior-changing release, produce and review a source-bound
   installed-plugin A/B certificate for Claude Code and Codex.
4. Run `scripts/release.sh X.Y.Z`; it must refuse missing or mismatched required
   certification before changing versioned files.
5. Review the diff, then perform separately authorized tag and publish actions.
6. Run exact-tag install smoke against the public tag.

Never use a local checkout or post-publish smoke result as a substitute for
candidate certification.
