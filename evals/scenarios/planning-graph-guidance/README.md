# planning-graph-guidance

Artifact oracle for the contract selected from a fresh-context behavioral
probe. The prompt and omissions below are the behavioral RED evidence. The grep
markers only prevent the resulting guidance from drifting later.

## Fresh-context prompt

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md and plugins/megapowers/skills/writing-plans/SKILL.md. Scenario: plan a wide database column rename used by API, worker, and frontend, while also shipping a user-visible feature; every phase must keep CI and deploy green. State the task plan shape, dependencies, whether to use expand-contract, and how blockers are represented. No edits. Return proposed response plus missing elements in current skill with exact line refs. Do not read external repos.
```

Context-authority probe:

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md plus plugins/megapowers/skills/{writing-plans,systematic-debugging,project-memory}/SKILL.md. Scenario: a repo contains CONTEXT.md, docs/adr/0007-payments.md, and .megapowers/project-memory.md. Asked to plan and diagnose a payments change, state which sources you read first, what each source governs, and how conflicts are resolved. No edits. Return proposed response and identify missing or ambiguous shipped guidance with exact line refs.
```

## Observed RED, 2026-07-15

The response proposed sequencing, but the current skill omitted the fields and
staged replacement rule needed to make that sequencing executable:

- No `Blocked by` relationship. Task boundaries at lines 39 to 58 and the task
  structure at lines 91 to 109 do not expose task dependencies.
- No owner for a material unresolved input and no explicit unblock condition.
  The same task contract has no `Owner` or `Unblocks when` slot.
- No ordered expand, migrate, contract path for a compatibility-sensitive
  replacement. The current task guidance does not define staged mixed-state
  checkpoints for API, worker, and frontend consumers.

The context-authority probe found no deterministic authority model across the
three skills. The required ordering and roles are:

- Actual observed behavior governs diagnosis. Repository instructions govern
  process. Diagnosis precedes change planning.
- `CONTEXT.md` supplies vocabulary and current domain context. An accepted ADR
  governs narrower design intent.
- Matching project memory supplies hidden historical hints, must be reverified,
  and must not duplicate canonical context or ADR content.
- Conflicts between sources are surfaced for resolution, never silently chosen.

## Deterministic RED

`solve.sh` emits one `OK` or `MISSING` marker per observed omission. `check.sh`
rejects a missing `out.txt`, any absent marker, and every `MISSING` marker. On
the pre-guidance skill it ends with:

```text
RED: planning-graph-guidance contract incomplete
```

## Mutation record

On 2026-07-15, deleting the `context-and-adr-pass` line from a generated
`out.txt` made `check.sh` exit 1 with:

```text
missing marker: context-and-adr-pass
```

On 2026-07-15, a temporary skill overlay reversed the debugging rule to
`Actual behavior is not authoritative`. The oracle emitted:

```text
MISSING source-role-authority
RED: planning-graph-guidance contract incomplete
```

In a separate temporary overlay, reversing the ordering rule to `diagnosis
does not precede planning a change` emitted:

```text
MISSING diagnosis-before-plan
RED: planning-graph-guidance contract incomplete
```

Four separate temporary overlays reversed one source role at a time:
repository instructions `do not govern process`, `CONTEXT.md` `is not current
domain vocabulary`, accepted ADRs `do not govern design intent`, and project
memories `should not be reverified`. Each emitted:

```text
MISSING source-role-authority
RED: planning-graph-guidance contract incomplete
```

## Parallel plan contract, 2026-07-16

After adding the four plan contract markers, but before adding the guidance,
`evals/run.sh planning-graph-guidance` returned JSON verdict `fail`. The direct
checker output was:

```text
MISSING parallel-safety
MISSING ownership
MISSING may-decompose
MISSING overlap-forces-sequential
RED: planning-graph-guidance contract incomplete
```

After adding the execution fields, sequential conditions, and exact ownership
guidance, the same eval returned:

```json
{"scenario":"planning-graph-guidance","skill":"writing-plans","kind":"artifact","agent":"mock","mode":"skill","verdict":"pass","ms":1554}
```

For the mutation check, a temporary copy changed `Ownership overlaps an active
task` to `Ownership overlap is allowed`. The real skill remained unchanged.
The solve and check scripts rejected the copy with:

```text
MISSING overlap-forces-sequential
RED: planning-graph-guidance contract incomplete
```
