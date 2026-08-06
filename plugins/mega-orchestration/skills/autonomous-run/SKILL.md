---
name: autonomous-run
description: >-
  Use when a long task must continue unattended across many steps or sessions,
  preserve durable progress, resume later, or keep going until done.
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

[references/file-contract.md](references/file-contract.md) is the per-file
contract: what each of `charter.md`, `plan.md`, `runbook.md`, `journal.md`,
`status`, and `evidence.md` holds, the `## <tag>: <name>` and `- acceptance:`
milestone grammar `run-derive-status` parses, and the `plan-digest` fingerprint
that fails a done-claim when a declared milestone or its acceptance line
changed. Read it before authoring `charter.md` or `plan.md`.

## Where the charter comes from

The run executes a goal that already survived design scrutiny; it does not
invent one. Refine an ambiguous goal through megapowers:brainstorming (if
installed) and copy the resulting spec's acceptance criteria into the charter's
done-when list verbatim, each with an executable check where one can exist. A
code milestone gets a plan only when a durable multi-step handoff is useful.
Autonomous execution chooses inline work for small or coupled milestones and
SDD for independently owned tasks where delegation pays for itself, then
journals that choice. Its declared check names the literal acceptance oracle,
never the executor's say-so. While a charter is active at level `autonomous` or
`on-the-loop`, the
megapowers process skills' interactive gates resolve themselves and journal the
decision instead of stopping; `in-the-loop` keeps every gate interactive.
Without the megapowers plugin, write the charter and milestones directly; the
contract stands on its own.

## The loop

The runbook owns the procedure, and it ships split by enforceability.
`run-init` copies [references/runbook-template.md](references/runbook-template.md)
into the run verbatim: a MUST section where every line is one mechanically
checkable constraint that blocks, and a SHOULD section holding the judgment
calls, which never blocks. Read it before working the first milestone. Put any
run-specific rule into whichever half it belongs to; a judgment call promoted
into MUST to be safe is how a run ends up blocked on nothing.

The outcomes that contract exists to produce:

- One session owns the run (`scripts/run-claim <run-id>`) and `plan.md` is
  frozen (`scripts/run-init <run-id> --replan`) before the first milestone.
- Each milestone closes against the acceptance check declared for it, and its
  journal entry cites what that check output.
- Status is derived (`scripts/run-derive-status`), certified once
  (`scripts/run-verify-status`), and never declared.
- Irreversible actions wait for the approval the autonomy level requires.

`run-derive-status` reads `done` only when every milestone declared in
`plan.md` (and every tagged milestone in the journal) has a final result entry,
and a milestone whose last entry is blocked derives to blocked. A passing
`run-verify-status` is the only sanctioned way a run reads finished, and it is
eval-guarded (evals/scenarios/autonomous-run-contract). When a step delegates,
the exact brief lives in the delegation artifacts; the journal cites them by
path so the run stays replayable without bloating the log.

Selecting an autonomous workflow grants no authorization of its own; the
runbook's Authorization section is where that rule blocks.

## Autonomy level (the dial, not blind autonomy)

`charter.md` declares one level; `scripts/autonomy-level <level>` prints the
policy so every step reads the same dial. It emits one disposition per action
class, REVERSIBLE, STAGED, and IRREVERSIBLE, because the dial gates by
reversibility and blast radius, never by "is it simple":

- **autonomous**: do reversible and staged work without asking; only
  irreversible or high-blast actions stop for approval (stage them through an
  effect broker).
- **on-the-loop** (default): proceed, but checkpoint legibly so a human
  watching the journal can interrupt; pause for irreversible actions.
- **in-the-loop**: the tightest oversight cadence. Pause for approval before
  every staged or irreversible action, and checkpoint at each milestone
  boundary so the human approves the direction before the next milestone.

Why the dial is shaped that way: the level sets checkpoint granularity, the
oversight the user asked for, not per-action friction on reversible work.
Minimizing human presence means making supervision cheap (a legible journal, a
readable report, decisions ranked by confidence), not removing the human's
ability to see. The per-class rules themselves live in the runbook's
Authorization section and are stated only there.

## The stopping rule (adaptive compute)

Spend by stakes and uncertainty, and stop deliberately. The mechanics are in
the runbook template (the per-milestone attempt cap, the blocked entry at the
cap, stakes-scaled verification effort). What belongs here is why they exist:
an uncapped fix/re-verify loop spends the whole budget on the one milestone
that was never going to pass, and a milestone journaled blocked with what was
tried is worth more to the next session than a tenth attempt.

## Reporting

`scripts/run-report <run-id>` emits a skimmable report: what is done, what is
left, decisions ranked by confidence lowest first (that is where to look),
failures surfaced plainly, and the provenance trail. The runbook says when to
run it; the reason is that supervision should cost the human a glance, not an
investigation. That is also why journal messages and report prose use the
handoff register (megapowers:using-megapowers, Communication, if installed):
conclusion first, declarative, self-contained.

## Guards

- The frozen charter and append-only journal are what let the run be trusted
  and replayed. Tidying either one destroys the evidence the done-claim rests
  on.
- The template is the only copy of the operating contract, and
  `scripts/tests/runbook-template.test.sh` keeps it that way: it pins the MUST
  and SHOULD counts per section and asserts `run-init` emits the file verbatim,
  so a rule cannot be dropped or quietly demoted into the advisory half.
- Declared milestones are fingerprinted against `plan-digest`, so a vanished
  milestone or a changed acceptance line fails the done-claim until you re-run
  `--replan` (references/file-contract.md, Milestone digest).
- Irreversible actions go through staging appropriate to the autonomy level;
  the effect broker, when present, is the mechanism.
- On Claude Code, the `run-loop.sh` Stop hook blocks a premature stop while a
  run this session explicitly claimed still reads active and points at the next unmet
  milestone. It is an accelerator, not the mechanism: it fails open on any
  doubt, respects `in-the-loop` (milestone checkpoints belong to the human),
  and the honest exit is a journaled blocked, paused, or final result entry
  plus a re-derived status. A hand-edited STATE is not an exit; the next
  run-derive-status overwrites it and run-verify-status fails a done-claim the
  journal does not support. On other harnesses the loop rides on the runbook
  discipline alone.
