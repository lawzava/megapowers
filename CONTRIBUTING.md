# Contributing

megapowers accepts small, measured changes to one plugin for current Claude Code
and Codex.

## Before a pull request

Run the deterministic checks:

```bash
scripts/validate.sh
bash evals/run-all.sh --json results.jsonl
```

Then add the evidence the change needs. An "oracle" below is the check that
decides whether an eval trial passed.

| Change | Required evidence |
|---|---|
| Hook, tool, manifest, or runner behavior | A regression test that fails first, then the focused test and the full deterministic checks |
| New or changed agent guidance | Reproduce the missing behavior or inspect the deficient contract; use task outcomes and an installed-plugin A/B comparison when a comparison would inform the change |
| Removed or compressed guidance | Deterministic checks, plus evidence for any published behavior the removed text carried |
| Editorial text | Link, reference, and deterministic checks only |
| Eval oracle | Proof that the oracle rejects a deliberately wrong artifact |

A runner selftest proves mechanics only. It is not behavioral evidence.

## Scope

- Keep the marketplace at exactly one plugin. `skills/catalog.json` is the
  skill inventory; tests and docs derive counts from it and never restate them.
- Keep skills portable across harnesses. Harness-specific mechanics belong in
  a narrow adapter or at a documented provider boundary.
- Prefer the agents, goals, permissions, worktrees, memory, and browser tools
  each harness already has.
- Do not add a model router, session prompt injection, scheduler, formatter,
  status line, or another harness without an explicit scope decision and fresh
  evidence.
- Add no unsourced model, benchmark, cost, context, or performance claims.

## Code and prose

- Follow repository instructions and neighboring conventions.
- Deterministic glue and tests use the Go standard library. Shell entrypoints
  only launch Go commands or the cached hook executable. Use `go test ./...`
  for package tests and `scripts/validate.sh` for the canonical check.
- Add or change behavior test-first. Keep hooks small and directly tested.
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
2. trigger recall, which checks from the trace that the right skill was
   selected;
3. optional credentialed installed-plugin A/B for comparative behavior;
4. exact-tag install smoke after publication.

Published rows must identify source, harness, CLI, model, effort, prompt,
fixture, plugin, environment, status, and artifacts. Malformed, incomplete,
indeterminate, timed-out, or harness-error data fails closed.

## Release sequence

1. Write the changelog entry, set the version, and freeze the candidate
   revision.
2. Run deterministic validation.
3. Run `scripts/release.sh X.Y.Z`; it validates the clean, already-versioned
   candidate without mutating, tagging, or publishing it.
4. Review the diff, push the exact revision, and wait for remote CI on it.
5. With CI green, create the signed tag and GitHub release; each needs its own
   authorization. The release workflow fast-forwards the `release` branch to
   the attested tag; installs track that branch.
6. Run exact-tag install smoke against the public tag.

The same order, with the study boundaries, is in
[docs/advanced/evals.md](./docs/advanced/evals.md).
