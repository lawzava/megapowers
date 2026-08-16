---
name: autonomous-run
description: Use when an approved goal must continue unattended across many steps, milestones, context resets, or sessions.
---

# Autonomous Run

Prefer native goals and native scheduling when the harness provides them. Use
this skill only for the durable checkpoint mechanics needed to resume honestly
after context or process loss; do not build a second task runner around native
capabilities.

Freeze the objective, done criteria, boundaries, authorized effects, and time,
step, or token cap before starting. For a multi-session run, keep one ignored
directory at `.megapowers/run/<id>/` with:

- `charter.md`: frozen objective, done criteria, scope, authority, and cap.
- `checkpoint.md`: current milestone, completed evidence, blockers, next
  command, workspace identity, and UTC update time.
- `journal.jsonl`: append-only observed transitions and their evidence.

Update the checkpoint only at a milestone or real state transition. Journal
what a command or external system proved, not an intention or progress guess.
Derive status from the journal, checkpoint, and fresh evidence; chat history is
not a status oracle.

On resume, read repository instructions, charter, checkpoint, journal tail,
and current workspace state. Reconcile differences before acting. Mark done
only after every done criterion passes its stated oracle. Mark blocked only for
a concrete dependency outside the run's authority, with the evidence and next
unblocking event. Pause at the declared cap without presenting partial work as
completion. Use `safe-effects` for every external mutation regardless of
autonomy level.
