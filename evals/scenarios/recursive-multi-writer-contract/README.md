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

## Acceptance-fix rerun, 2026-07-16

The strengthened oracle materializes each file before extended,
case-insensitive matching. It requires the exact depth-five policy and rejects
contradictory permissions even when every required positive sentence remains.
The fixed artifact eval passed:

```json
{"scenario":"recursive-multi-writer-contract","skill":"subagent-driven-development","kind":"artifact","agent":"mock","mode":"skill","verdict":"pass","ms":85855}
```

All four temporary-copy mutations reran both shipped SDD script suites. The two
approved replacement mutations remain covered, followed by two additive
contradictions that retain the required lifecycle or no-teams sentence:

```text
remove-codex-rule -> missing marker: codex-lead-rule
replace-no-teams -> missing marker: claude-no-teams
add-inexact-writer-release -> missing marker: no-inexact-writer-release
add-recursive-agent-teams -> missing marker: no-recursive-agent-teams
== Task 7 mutations: 4 rejected, 0 escaped ==
```
