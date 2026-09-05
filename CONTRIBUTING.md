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
| New or changed agent guidance | Reproduce the missing behavior or inspect the deficient contract; use task outcomes and installed-plugin A/B when comparison would inform the change |
| Removed or compressed guidance | Deterministic gate, plus evidence for any published behavior the removed text carried |
| Editorial text | Link, reference, and deterministic checks only |
| Eval oracle | Mutation proof that the oracle rejects a deliberately wrong artifact |

A runner selftest proves mechanics only. It is not behavioral evidence.

## Scope

- Keep the marketplace at exactly one plugin. `skills/catalog.json` is the
  skill inventory; tests and docs derive counts from it, never restate them.
- Keep semantic skills portable. Harness-specific mechanics belong at a narrow
  adapter or documented provider boundary.
- Prefer native agents, goals, permissions, worktrees, memory, and browser tools.
- Do not add a model router, session prompt injection, scheduler, formatter,
  status line, or another harness without an explicit scope decision and fresh
  evidence.
- Add no unsourced model, benchmark, cost, context, or performance claims.

## Code and prose

- Follow repository instructions and neighboring conventions.
- Deterministic glue and tests use Go standard library. Shell entrypoints only
  launch Go commands or the cached hook executable. Use `go test ./...` for
  package tests and `scripts/validate.sh` for the canonical gate.
- Add or change behavior test-first. Keep hooks bounded and directly tested.
- Human-facing prose leads with the outcome, preserves source facts, and omits
  unsupported claims and session history.
- Keep commits conventional and focused. Do not add attribution or session
  trailers.

Use `writing-agent-instructions` when authoring skills or scoped project
instructions. Keep trigger terms in `description`, quote frontmatter values
that contain colons, and compare task outcomes separately from skill selection.

## Evaluation

The current evidence stack is documented in
[docs/advanced/evals.md](./docs/advanced/evals.md):

1. deterministic regressions for mechanics;
2. optional credentialed installed-plugin A/B for comparative behavior;
3. report-only PR replay for real-project correctness;
4. exact-tag install smoke after publication.

Published rows must identify source, harness, CLI, model, effort, prompt,
fixture, plugin, environment, status, and artifacts. Malformed, incomplete,
indeterminate, timed-out, or harness-error data fails closed.

## Release sequence

1. Write the changelog entry and freeze the candidate revision.
2. Run deterministic validation.
3. Run `scripts/release.sh X.Y.Z`; it validates the clean, already-versioned
   candidate without mutating, tagging, or publishing it.
4. Review the diff, then perform separately authorized tag and publish actions.
5. Wait for remote CI on the exact revision. The release workflow fast-forwards
   the `release` branch to the attested tag; installs track that branch.
6. Run exact-tag install smoke against the public tag.
