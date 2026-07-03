---
name: autonomous-run
description: >-
  Use to run a long, largely-unattended task across many steps or sessions —
  keeping durable state the run survives on (a frozen charter, a plan with
  acceptance criteria, an operating runbook, an append-only journal, a
  machine-readable status) plus an autonomy level that decides what proceeds vs
  what waits for a human. Triggers on "work on this autonomously", "long-running
  task", "keep going until it's done", "run this unattended", "resume the run".
---

# Autonomous Run

Unsure whether a full run is warranted, or a lighter structure fits? Start at
mega-orchestration:orchestrating, the decision root.

Long, unattended work fails on two things: losing state across sessions, and doing
something irreversible the human didn't want. This skill fixes both with a small
durable file contract and an explicit autonomy dial. The files are plain text and
identical across every runtime; nothing here depends on a specific harness.

## The file contract

Everything for a run lives under `.megapowers/run/<run-id>/`:

| File | Role | Rule |
|---|---|---|
| `charter.md` | The frozen spec: goal, explicit **done-when** acceptance criteria, autonomy level, budget/turn caps, blast-radius limits. | Written once. **Never edit it** — if the goal changes, that's a new run. |
| `plan.md` | Milestones, each with its own acceptance check (prefer an executable oracle). | Update as milestones complete; don't rewrite history. |
| `runbook.md` | The operating loop: how to pick the next unmet milestone, when to stop, what to do on failure. | Stable; the loop you follow. |
| `journal.md` | Append-only audit log: every action, decision (with a confidence), result, and the acting model. | **Append only. Never rewrite.** This is the trail. |
| `status` | One machine-readable line the loop (and any hook) reads: state, cursor, last-verify. | Overwrite in place; it's the pointer, not the history. |

Run `scripts/run-init <run-id>` to scaffold these. It refuses to overwrite an
existing `charter.md`, so the charter stays frozen.

## Where the charter comes from (the spec pipeline joint)

The run does not invent its own goal; it executes one that already survived
design scrutiny:

- **Charter goal + done-when:** for an ambiguous or novel goal, refine it first
  via megapowers:brainstorming (if installed) and reference the resulting spec
  file in the charter. The spec's acceptance criteria become the charter's
  done-when list, copied verbatim, each with an executable check where one can
  exist.
- **Plan milestones:** a code milestone gets a task plan via
  megapowers:writing-plans (if installed) and executes through
  megapowers:subagent-driven-development; the milestone's declared check is
  then "plan file X fully checked off and its verification commands pass",
  never the executor's say-so.
- **The dial reaches the process skills:** while a charter is active at level
  `autonomous` or `on-the-loop`, the megapowers process skills' interactive
  gates (writing-plans' execution-choice question, SDD's pre-flight batch,
  executing-plans' concern check) resolve themselves and journal the decision
  instead of stopping. `in-the-loop` keeps every gate interactive.

Without the megapowers plugin, write the charter and milestones directly; the
file contract stands on its own.

## The loop (run → detect → fix → re-verify)

Read `runbook.md`; the default loop is:

1. Read `status` to find the cursor (the next unmet milestone). After a restart or
   context compaction, **trust the files, not your memory** — the journal and git
   history are the truth.
2. Do the next milestone's work (delegate where a different model is better — see
   mega-orchestration:multi-agent-delegation).
3. **Run that milestone's acceptance check** (an executable oracle where one
   exists — a test, a compile, a megapowers eval). Checks are declared per
   milestone in `plan.md` up front; a result entry must cite the declared
   check, not a substitute the work happens to pass. For an external
   dependency, the declared check asserts *where it resolves from* (e.g. its
   import path), not just that it imports — a vendored local substitute must
   not satisfy it. Detect failure honestly.
4. On failure: fix and re-verify (bounded — see the stopping rule). On success:
   append a journal line and advance the cursor in `status`.
5. Stop when every done-when criterion in `charter.md` is met, the budget/turn cap
   is hit, or you hit a blocker only the human can clear.

**The status file must agree with the journal trail — so don't write it, derive
it.** The journal is the only hand-written record; never hand-edit the status
file — regenerate it with `scripts/run-derive-status <run-id>` after every
milestone. Tag journal messages with their milestone ("M2: ..."), and a result
entry must cite the check you ran and what it output — a milestone whose last
entry is blocked derives to blocked, never done. Before finishing, run
`scripts/run-verify-status <run-id>`; a run that fails the check cannot claim
completion.

Append to the journal with `scripts/run-journal <run-id> <kind> <confidence> <msg>`
(`kind` = action|decision|result|blocked|paused; `confidence` = 0.0–1.0; it records
the acting model from `MEGAPOWERS_MODEL`). `paused` marks a deliberate checkpoint:
a trailing paused entry derives STATE=paused, and any later entry resumes.
When every milestone declared in `plan.md` (and every tagged milestone in the
journal) has a final result entry, run-derive-status derives STATE=done —
that is the only sanctioned way a run reads finished. The journal captures the model and a short
description; when a step delegates, the exact brief/prompt is captured by the
delegation artifacts (e.g. subagent-driven-development's task briefs) — reference
those from the journal message so the run stays replayable without bloating the log.

## Autonomy level (the dial, not blind autonomy)

`charter.md` declares one level; `scripts/autonomy-level <level>` prints the policy
so every step reads the same dial. The dial gates by **reversibility × blast
radius**, never by "is it simple":

- **autonomous** — do reversible and *staged* work without asking; only irreversible
  or high-blast actions stop for approval (stage them through an effect broker).
- **on-the-loop** (default) — proceed, but checkpoint legibly so a human watching
  the journal can interrupt; pause for irreversible actions.
- **in-the-loop** — the user has chosen the tightest oversight *cadence*: pause for
  approval before every staged/irreversible action, and checkpoint at each milestone
  boundary so the human approves the direction before the next milestone.

The invariant is about **actions, not cadence**: never gate an individual *reversible
action* on a human just to add friction — at every level, reversible actions proceed.
What the level sets is the *checkpoint granularity* the user asked for (in-the-loop
checkpoints at milestone boundaries; that's requested oversight of the plan's
direction, not a per-action gate on reversible work). Minimizing human *presence*
means making supervision *cheap* (a legible journal, a readable report, decisions
ranked by confidence) — not removing the human's ability to see.

## The stopping rule (adaptive compute)

Spend by stakes × uncertainty, and stop deliberately:

- Cap fix/re-verify attempts per milestone (default 3). After the cap, mark the
  milestone `blocked` in the journal with what you tried and the next idea, and move
  on or surface it — don't loop forever.
- Scale delegation/verification effort to the milestone's stakes (a money- or
  auth-touching milestone earns a cross-model verification pass; a doc tweak
  doesn't).
- Respect the budget/turn cap in the charter; when it's near, finish the current
  milestone cleanly and report rather than starting new work.

## Reporting

`scripts/run-report <run-id>` emits a skimmable report: what's done, what's left,
decisions **ranked by confidence (lowest first — that's where to look)**, failures
surfaced plainly, and the provenance trail. Run it at checkpoints and at the end so
supervision costs the human a glance, not an investigation. Journal messages and
report prose use the handoff register (megapowers:using-megapowers, Communication,
if installed): conclusion first, declarative, self-contained.

## Guards

- The charter is frozen and the journal is append-only — that's what lets the run be
  trusted and replayed. Don't "tidy" them.
- Irreversible actions go through staging appropriate to the autonomy level; the
  effect broker (when present) is the mechanism.
- This plugin ships a loop driver for Claude Code: the `run-loop.sh` Stop hook
  blocks a premature stop while a run this session touched still reads active,
  and points at the next unmet milestone. It is an accelerator, not the
  mechanism — it fires only on Claude Code, fails open on any doubt, respects
  `in-the-loop` (milestone checkpoints belong to the human, so it never blocks
  those), and a run exits it honestly: journal a blocked, paused, or final
  result entry and re-derive status. A hand-edited STATE is not an exit — the
  next run-derive-status overwrites it and run-verify-status fails a done-claim
  the journal doesn't support. On other harnesses the loop rides on this
  skill's runbook discipline alone.
