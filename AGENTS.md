# Repository Instructions

## Scope

This repository publishes one optional `megapowers` plugin for current Claude
Code and Codex. Skill Markdown may load elsewhere, but no other harness is a
supported or tested target. Keep every shipped artifact public-safe: no
personal secrets, machine-specific paths, or undeclared private services.

## Authority

Repository instructions, existing code, and configured project tools are authoritative; skills supply defaults only where the repository is silent.
Read the instructions and nearby implementation before choosing a pattern.

## Working Rules

- One writer owns a shared branch, its Git state, integration, and final
  verification. Other agents work read-only, in isolated worktrees, or return
  patches under explicit non-overlapping ownership.
- For behavior changes, write and run a failing test before implementation.
  Diagnose unknown failures before editing. Run focused checks while iterating
  and the canonical suite before a completion claim.
- Treat agent and reviewer output as claims. Re-run the relevant oracle
  yourself, and keep local, external, and user-visible verification distinct.
- Before a deploy, send, charge, migration, destructive action, or external
  write, confirm authority for the exact target, effect, environment, and
  scope. Read back the target after execution.
- Make the smallest complete change. Preserve user-owned edits, avoid adjacent
  cleanup, and remove only code made unused by the change.

## Communication

Apply the prose contract to plans, task briefs, commits, responses, reviews,
PRs, docs, release notes, and errors. Lead with the outcome; keep only facts
and reasoning the reader needs. Preserve identifiers, numbers, commands,
caveats, uncertainty, and decisions. Remove padding, never invent facts, and
leave already-direct prose alone.
Attribute load-bearing claims to a named source, direct observation, or explicit
uncertainty. Replace vague evaluation with the actor, mechanism, scope,
condition, or measurement.

## Skills

- Skill frontmatter contains portable `name` and task-triggering `description`
  fields only. Components remain at plugin root under `skills/` and `hooks/`.
- Keep judgment in skills and deterministic policy in tests or tools. Required
  references are linked directly; optional details load only when relevant.
- Do not commit generated agent workspaces or planning artifacts under
  `docs/megapowers/`, `docs/plans/`, `docs/specs/`, `.omc/`, `.claude-flow/`,
  or `.swarm/`.

## Tooling and Git

- New repository glue is Go. Application code follows its project language;
  harness-required hook entrypoints may remain shell.
- Commit only when the user directs. Use focused conventional commits, stage
  explicit paths, never bypass hooks or ignored-path policy, and never rewrite
  shared history without explicit approval.
- Honor `TMPDIR`. Resolve an unset value before use, keep large scratch data on
  disk-backed storage, and clean up only scratch this process owns.

## Verification

Run `scripts/validate.sh` after meaningful changes. When installed, also run:

```bash
claude plugin validate --strict .claude-plugin/marketplace.json
claude plugin validate --strict plugins/megapowers
```
