---
name: autonomous-run
description: >-
  Use when a long task must continue unattended across many steps or sessions,
  preserve durable progress, resume later, or keep going until done.
license: MIT
---

# Autonomous Run

Use this for bounded, long-running work that must survive handoffs. Do not use it to
invent a goal or authorization. Resolve an ambiguous goal before starting a run.

## Inputs and durable outputs

Input: a frozen goal, acceptance criteria, autonomy level, external stop budget,
authorization boundaries, and blast-radius limits. Durable output: a charter, milestone
plan, runbook, append-only journal, derived status, and evidence map. The file contract
defines their portable format: [references/file-contract.md](references/file-contract.md).

## Operating loop

1. Freeze the charter and plan before work begins. Each milestone has a declared
   acceptance oracle and a regression subset. A changed goal starts a new run; a
   deliberate plan change is recorded and re-baselined rather than rewritten silently.
2. Claim one accountable session, work one milestone, run its declared oracle, append
   the result and evidence, then derive status from the journal and plan. Durable files,
   not memory or a hand-written status, decide what is next and what is complete.
3. Stop a failed milestone at its declared attempt or budget limit with a blocked entry
   that names evidence, attempted approaches, and the next decision needed. Do not
   spend the remaining run budget retrying without new information.
4. Certify completion only when every declared milestone and acceptance criterion has
   recorded evidence and status verification succeeds. Local preparation is never
   external completion evidence.

The supplied helpers scaffold and freeze a run, claim its session, append journal
entries, derive status, and verify a completion claim. Resolve `scripts/` from
this skill's installed directory before invoking them:

```bash
scripts/run-init <run-id>
scripts/run-init <run-id> --replan
scripts/run-claim <run-id>
scripts/run-journal <run-id> <kind> <confidence> <message>
scripts/run-derive-status <run-id>
scripts/run-verify-status <run-id>
```

The [runbook template](references/runbook-template.md) is the executable operating
contract: it carries the required journal fields, milestone sequence, and
authorization gates. After initial plan authoring, run
`--replan` before claiming the run or starting its first milestone, so it freezes the
milestone digest and declared acceptance checks before execution. A later plan change
requires a deliberate, recorded replan before resumed work, never a pre-certification
cleanup. `scripts/tests/runbook-template.test.sh` verifies that the template remains complete and
that scaffolding copies it unchanged.

## Safety

The `autonomous`, `on-the-loop`, and `in-the-loop` levels set progressively
closer oversight cadence, never permission. Classify every effect by mutation,
reversibility, sensitivity, environment, and blast radius. Effects outside the
charter, and irreversible or high-blast effects, wait for the approval required
by `mega-orchestration:effect-broker`. Scheduled runs simulate or defer any
action whose approval cannot be obtained from an attending human.

## Oracles

The milestone's declared acceptance check is the completion oracle. Preserve its output
in the journal and evidence map. Derive status from those records, verify it before a
completion claim, and retain the charter and journal unchanged so a later session can
audit the claim.

The oracle belongs to the charter, not to the session working the milestone. A run that
edits its own acceptance check has certified nothing, so record such a change as a
deliberate replan before the milestone resumes, never as part of the work that makes it
pass. Keep oracle output small in the session and detailed in the file: verbose test
output displaces the state that decides what happens next.

## Context

Durable files, not a long conversation, carry a run. Budget the working set well under
the nominal window and treat a context reset that reloads the charter, plan, journal,
and derived status as the normal way to continue a long run, not as recovery from a
mistake. Effective capacity is a fraction of the advertised number, and a session near
what it believes is its limit begins wrapping up prematurely, which reads as a milestone
finishing early rather than as an error. Compaction carries that pressure across; a
reset backed by the file contract does not, which is what the file contract is for.
