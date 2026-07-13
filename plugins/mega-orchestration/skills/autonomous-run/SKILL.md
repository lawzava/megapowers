---
name: autonomous-run
description: >-
  Use to run a long, largely-unattended task across many steps or sessions —
  keeping durable state the run survives on (a frozen charter, a plan with
  acceptance criteria, an operating runbook, an append-only journal, a
  machine-readable status) plus an autonomy level that decides what proceeds vs
  what waits for a human. Triggers on "work on this autonomously", "long-running
  task", "keep going until it's done", "run this unattended", "resume the run".
license: MIT
---

# Autonomous Run

Unsure whether a full run is warranted or a lighter structure fits? Start at
mega-orchestration:orchestrating, the decision root.

Long unattended work fails in two ways: state lost across sessions, and an
irreversible action the human did not want. This skill fixes both with a small
durable file contract and an explicit autonomy dial. The files are plain text
and identical across runtimes; nothing here depends on a specific harness.

## The file contract

Everything for a run lives under `.megapowers/run/<run-id>/`. Scaffold it with
`scripts/run-init <run-id> --model <model-id>`; the model flag records
provenance so every journal entry names the acting model. run-init refuses to
overwrite an existing charter.

Run IDs are lowercase kebab case (`a-z`, `0-9`, and single hyphens), for
example `release-check`. Every run helper rejects other forms before touching
the run directory.

| File | Contract |
|---|---|
| `charter.md` | The frozen spec: goal, explicit done-when acceptance criteria, autonomy level, blast-radius limits, and external stop budgets (time, step, or token caps) declared up front. Written once, never edited; a changed goal is a new run. |
| `plan.md` | Milestones, each with its own acceptance check, preferably an executable oracle. Update as milestones complete; do not rewrite history. |
| `runbook.md` | The operating loop: how to pick the next unmet milestone, when to stop, what to do on failure. |
| `journal.md` | Append-only audit trail of every action, decision, and result. Never rewritten. |
| `status` | Machine-readable `KEY=value` lines the loop and any hook read: STATE, CURSOR, LEVEL, LAST_VERIFY, PLAN_WARNINGS. Derived, never hand-written; the pointer, not the history. |

Milestone format matters because status derivation parses it: headings are
`## <tag>: <name>` where `<tag>` matches `[A-Za-z][A-Za-z0-9_-]*` (one token,
then a colon; `## M2: rollout`, not `## Phase 2: rollout`), and each acceptance
check sits on a line starting with
`- acceptance:`. A heading that does not parse drops out of done-derivation, so
`scripts/run-derive-status` counts it into `PLAN_WARNINGS` and refuses `done`
while any remain. An acceptance check written any other way escapes the digest
freeze and stays tamperable even with a digest present.

## Where the charter comes from

The run executes a goal that already survived design scrutiny; it does not
invent one. Refine an ambiguous goal through megapowers:brainstorming (if
installed) and copy the resulting spec's acceptance criteria into the charter's
done-when list verbatim, each with an executable check where one can exist. A
code milestone gets its plan via megapowers:writing-plans and executes through
megapowers:subagent-driven-development; its declared check is then "plan file X
fully checked off and its verification commands pass", never the executor's
say-so. While a charter is active at level `autonomous` or `on-the-loop`, the
megapowers process skills' interactive gates resolve themselves and journal the
decision instead of stopping; `in-the-loop` keeps every gate interactive.
Without the megapowers plugin, write the charter and milestones directly; the
contract stands on its own.

## The loop

The runbook owns the procedure. The outcomes it must produce:

- Once `plan.md` is authored, freeze it with `scripts/run-init <run-id>
  --replan` before working the first milestone, so the milestone digest exists
  to certify the eventual done-claim; `run-verify-status` fails a done-claim
  that has none. After a restart or context compaction, trust the files, not
  your memory. The journal and git history are the truth.
- Before opening a new milestone, confirm the completed ones still pass their
  acceptance checks (or the declared fast subset). A run whose earlier work
  broke is regressing, not progressing. Delegate milestone work where a
  different model is better (mega-orchestration:multi-agent-delegation).
- A milestone completes only against the acceptance check declared for it in
  `plan.md`, not a substitute the work happens to pass. For an external
  dependency, the declared check asserts where it resolves from (for example
  its import path), not merely that it imports, so a vendored local substitute
  cannot satisfy it. Detect failure honestly; on failure, fix and re-verify
  within the stopping rule's bounds.
- Journal at every decision point with `scripts/run-journal <run-id> <kind>
  <confidence> <msg>` (kind = action, decision, result, blocked, paused;
  confidence 0.0 to 1.0). Tag messages with their milestone ("M2: ..."). Ground
  every progress claim in a tool result: a result entry cites the declared
  check it ran and what it output, evidence rather than intention. A trailing
  paused entry derives STATE=paused; any later entry resumes.
- At each milestone boundary, checkpoint the work (commit on the run's branch,
  or the workspace's native checkpoint), then regenerate status with
  `scripts/run-derive-status <run-id>`.
- Stop when every done-when criterion in `charter.md` is met, a stop budget
  declared in the charter is exhausted, or you hit a blocker only the human can
  clear. Near a budget cap, finish the current milestone cleanly and report
  rather than start new work.

Status is derived, never declared. The journal is the only hand-written record;
`run-derive-status` reads `done` only when every milestone declared in
`plan.md` (and every tagged milestone in the journal) has a final result entry,
and a milestone whose last entry is blocked derives to blocked. Before
finishing, run `scripts/run-verify-status <run-id>`; a run that fails the check
cannot claim completion. That closure is the only sanctioned way a run reads
finished, and it is eval-guarded (evals/scenarios/autonomous-run-contract).
When a step delegates, the exact brief lives in the delegation artifacts;
reference them from the journal message so the run stays replayable without
bloating the log.

## Autonomy level (the dial, not blind autonomy)

`charter.md` declares one level; `scripts/autonomy-level <level>` prints the
policy so every step reads the same dial. The dial gates by reversibility and
blast radius, never by "is it simple":

- **autonomous**: do reversible and staged work without asking; only
  irreversible or high-blast actions stop for approval (stage them through an
  effect broker).
- **on-the-loop** (default): proceed, but checkpoint legibly so a human
  watching the journal can interrupt; pause for irreversible actions.
- **in-the-loop**: the tightest oversight cadence. Pause for approval before
  every staged or irreversible action, and checkpoint at each milestone
  boundary so the human approves the direction before the next milestone.

The invariant is about actions, not cadence: at every level a reversible action
proceeds without a human gate, and an irreversible one always waits for one.
What the level sets is checkpoint granularity, the oversight the user asked
for, not per-action friction on reversible work. Minimizing human presence
means making supervision cheap (a legible journal, a readable report, decisions
ranked by confidence), not removing the human's ability to see. Scheduled and
cloud runners execute without permission prompts; anything the effect broker
would gate must be simulated or deferred to an attended session, and the
runbook says so.

## The stopping rule (adaptive compute)

Spend by stakes and uncertainty, and stop deliberately. Cap fix/re-verify
attempts per milestone (default 3); at the cap, journal the milestone as
blocked with what you tried and the next idea, then move on or surface it
rather than loop. Scale verification effort to the milestone's stakes: a money-
or auth-touching milestone earns a cross-model verification pass, a doc tweak
does not.

## Reporting

`scripts/run-report <run-id>` emits a skimmable report: what is done, what is
left, decisions ranked by confidence lowest first (that is where to look),
failures surfaced plainly, and the provenance trail. Run it at checkpoints and
at the end so supervision costs the human a glance, not an investigation.
Journal messages and report prose use the handoff register
(megapowers:using-megapowers, Communication, if installed): conclusion first,
declarative, self-contained.

## Guards

- The frozen charter and append-only journal are what let the run be trusted
  and replayed. Do not tidy them.
- Declared milestones are fingerprinted: `run-init` snapshots each milestone
  heading and its acceptance line into `plan-digest`. Thereafter
  `run-verify-status` fails a done-claim (and `run-derive-status` refuses
  `done`) if a declared milestone vanishes or its acceptance line weakens. To
  change the plan deliberately, re-run `--replan`, which re-snapshots and
  journals a decision; the charter still never changes.
- Irreversible actions go through staging appropriate to the autonomy level;
  the effect broker, when present, is the mechanism.
- On Claude Code, the `run-loop.sh` Stop hook blocks a premature stop while a
  run this session touched still reads active and points at the next unmet
  milestone. It is an accelerator, not the mechanism: it fails open on any
  doubt, respects `in-the-loop` (milestone checkpoints belong to the human),
  and the honest exit is a journaled blocked, paused, or final result entry
  plus a re-derived status. A hand-edited STATE is not an exit; the next
  run-derive-status overwrites it and run-verify-status fails a done-claim the
  journal does not support. On other harnesses the loop rides on the runbook
  discipline alone.
