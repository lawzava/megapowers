---
name: autonomous-run
description: Use when an approved goal must continue unattended across many steps, milestones, context resets, or sessions.
when_to_use: Trigger phrases: run unattended, keep going across resets, babysit this PR until merged, continue until done, overnight run, resume the approved goal. Not for one bounded task.
metadata:
  short-description: Durable unattended progress across steps, resets, and sessions
---

# Autonomous Run

Prefer native goals and scheduling. Select this skill only for a currently
approved autonomous goal with an existing or newly approved charter. A crash,
compaction, ordinary handoff, or harness switch does not inherit authority.

Keep durable state in one ignored `.megapowers/run/<id>/` directory:

- `charter.md`: objective, done criteria, scope, authority, and cap.
- `checkpoint.md`: milestone, workspace, branch or worktree, HEAD, artifact
  identities, completed evidence, delegate ownership and expected return
  artifacts, blockers, next safe action, remaining effect authority, and UTC
  freshness.
- `journal.jsonl`: append-only observed transitions and their evidence.

Update the checkpoint only at a milestone or real state transition. Journal
what an oracle proved, not intent or a progress guess. Derive status from the
journal, checkpoint, and fresh evidence; chat history is not a status oracle.

On resume, reread repository instructions, charter, checkpoint, journal tail,
and current workspace state. Compare the repository, worktree, branch, HEAD,
runtime, and relevant external state. If evidence is missing, contradictory, or
shows a workspace mismatch, stop before acting. Do not execute the next command
until scope and authority are re-established. A native goal does not transfer
automatically across harnesses.

Use `paused` when a cap or intentional stop ends authorized execution. Preserve
the checkpoint and wait for renewed authority. Use `blocked` only for a concrete
external dependency outside current authority, such as a provider limit or
expired credential, with its evidence and unblocking event; surface the block
once rather than waiting silently. Mark done only after every criterion passes its stated oracle. Use
`safe-effects` for every external mutation.
