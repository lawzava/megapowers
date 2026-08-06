# Run File Contract

The files `scripts/run-init` scaffolds under `.megapowers/run/<run-id>/`, the
milestone grammar the status derivation parses, and the digest that detects a
mid-run redefinition of success.

| File | Contract |
|---|---|
| `charter.md` | The frozen spec: goal, explicit done-when acceptance criteria, autonomy level, blast-radius limits, and external stop budgets declared up front under Caps as `budget`, `turns`, and `attempts` (the per-milestone fix-and-re-verify cap the runbook enforces, default 3). Written once, never edited; a changed goal is a new run. |
| `plan.md` | Milestones, each with its own acceptance check, preferably an executable oracle, plus one `Regression subset:` line naming which completed checks re-run before a new milestone opens (`all`, or a named fast subset). Update as milestones complete; do not rewrite history. |
| `runbook.md` | The operating loop, split by enforceability: a blocking MUST section of mechanically checkable constraints and a non-blocking SHOULD section of judgment calls. `run-init` copies [runbook-template.md](runbook-template.md) in verbatim and fails loudly if that template is missing; add run-specific rules to whichever half they belong to. |
| `journal.md` | Append-only audit trail of every action, decision, and result. Never rewritten. |
| `status` | Machine-readable `KEY=value` lines the loop and any hook read: STATE, CURSOR, LEVEL, LAST_VERIFY, PLAN_WARNINGS. Derived, never hand-written; the pointer, not the history. |
| `evidence.md` | Literal acceptance map: implementation target, local oracle, required external, UX, or database oracle, earned state, and evidence. |

Milestone format matters because status derivation parses it: headings are
`## <tag>: <name>` where `<tag>` matches `[A-Za-z][A-Za-z0-9_-]*` (one token,
then a colon; `## M2: rollout`, not `## Phase 2: rollout`), and each acceptance
check sits on a line starting with
`- acceptance:`. A heading that does not parse drops out of done-derivation, so
`scripts/run-derive-status` counts it into `PLAN_WARNINGS` and refuses `done`
while any remain. An acceptance check written any other way escapes the digest
freeze, so it can change mid-run without the done-claim noticing.

## Milestone digest

Declared milestones are fingerprinted: `run-init` snapshots each milestone
heading and its acceptance line into `plan-digest`. Thereafter
`run-verify-status` fails a done-claim (and `run-derive-status` refuses
`done`) if a declared milestone vanishes or its acceptance line changes. To
change the plan deliberately, re-run `--replan`, which re-snapshots and
journals a decision; the charter still never changes. This is drift
detection, not a security control: a long run forgets what it promised, and
the fingerprint makes a mid-run redefinition of success explicit rather than
silent. Anything that can edit the plan can also re-baseline the digest, so
it stops accidents and self-deception, not a determined actor.
