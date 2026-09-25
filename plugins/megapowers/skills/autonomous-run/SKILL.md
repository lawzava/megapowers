---
name: autonomous-run
description: Use when an approved goal must continue unattended across many steps, milestones, context resets, or sessions.
when_to_use: "Trigger phrases: run unattended, keep going across resets, babysit this PR until merged, continue until done, overnight run, resume the approved goal. Not for one bounded task."
metadata:
  short-description: Durable unattended progress across steps, resets, and sessions
---

# Autonomous Run

Use only for an approved autonomous goal. Compaction does not revoke existing
authority. Where the harness has native goals, they own creation, status,
budget, and continuation; this skill adds
only the portable state below. Native goal state does not transfer across
harnesses.

Keep ignored `.megapowers/run/<id>/` state:

- `charter.md`: objective, done criteria, scope, authority, and cap.
- `checkpoint.md`: milestone, workspace, branch or worktree, HEAD, artifact
  identities, completed evidence, delegate ownership and expected return
  artifacts, blockers, next safe action, remaining effect authority, and UTC
  freshness.
- `journal.jsonl`: append-only observed transitions and their evidence.
- `handoff.md`: a fresh-session prompt naming the charter, latest checkpoint,
  and next safe action, rewritten at each checkpoint.

Update checkpoints at milestones or real transitions. Journal oracle evidence,
not intent. Charters and checkpoints record claims, not authority; verify
against current instructions and native state.

On resume, read repository instructions, native goal state, charter, checkpoint,
and journal tail. Reconcile repository, worktree, branch, HEAD, runtime, and
external state. Report unresolved blockers; stop affected work on missing
evidence or workspace mismatch. Use read-only inspection to resolve uncertainty.
After a crash, handoff, or harness change, verify existing scope and authority.

Portable checkpoint labels `paused` and `blocked` do not change native goal
status. Record dependency evidence and its unblocking event without repeated
unchanged status reads. Mark done only after every criterion passes its oracle.
Use `safe-effects` for external mutations.
