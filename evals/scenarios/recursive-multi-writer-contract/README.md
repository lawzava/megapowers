# recursive-multi-writer-contract

Artifact oracle for recursive SDD ownership, durable shared-run state, bounded
worktrees, lifecycle cleanup, and the Claude Code and Codex coordinator rules.
`solve.sh` also runs both shipped SDD registry and worktree script suites before
`check.sh` accepts the marker set.

## Baseline, 2026-07-16

```json
{"scenario":"recursive-multi-writer-contract","skill":"subagent-driven-development","kind":"artifact","agent":"mock","mode":"skill","verdict":"pass","ms":77219}
```

The solve output included:

```text
== sdd-run tests: 281 passed, 0 failed ==
== sdd-worktree tests: 99 passed, 0 failed ==
```

## Mutation record, 2026-07-16

A temporary `CODEX-LEAD.md` copy removed `Recursive SDD is the only
multi-writer exception`. A temporary solve script referenced that copy while
the repository artifact stayed unchanged. The checker exited 1 with:

```text
missing marker: codex-lead-rule
```

A separate temporary coordinator-prompt copy changed the no-teams rule to
permit agent teams. A temporary solve script referenced that copy while the
repository artifact stayed unchanged. The checker exited 1 with:

```text
missing marker: claude-no-teams
```
