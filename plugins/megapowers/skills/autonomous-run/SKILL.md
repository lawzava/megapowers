---
name: autonomous-run
description: Use when an approved goal must continue unattended across many steps, milestones, context resets, or sessions, including keeping a pull request moving until it is merge-ready.
when_to_use: "Trigger phrases: run unattended, keep going across resets, babysit this PR until merged, keep fixing CI and review comments until the PR is merge-ready, continue until done, overnight run, resume the approved goal. Not for one bounded task."
metadata:
  short-description: Durable unattended progress across steps, resets, and sessions
---

# Autonomous Run

Use only for an approved autonomous goal. Compaction does not revoke existing
authority. Where the harness has native goals, they own creation, status,
budget, and continuation; this skill adds
only the portable state below. Native goal state does not transfer across
harnesses.

Keep `.megapowers/run/<id>/` state, adding `.megapowers/` to
`.git/info/exclude` when Git does not already ignore it:

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

While work is owed and nothing blocks it, do not end a turn with a summary
that announces the next step instead of taking it, an offer to continue, a
decision list that blocks nothing, or a milestone report. Put status notes and
recommendations in the same message as the next tool call. Stop only when no
work can advance without the user or protected access. Before asking, use
what is already reachable: linked pull requests and tickets, authenticated
CLIs, existing previews, and the charter. Ask only for missing authority,
secrets, or decisions. A text-only turn is a
report, not completion: if checklist items remain, continue. After two or
three automatic continuations on the same item, record it as blocked for
review. This never overrides confirmation for risky or destructive actions.

Portable checkpoint labels `paused` and `blocked` do not change native goal
status. Record dependency evidence and its unblocking event. To watch for a
change, run one blocking watcher that exits when the state changes, or a
harness scheduler; check human-paced channels at most every five minutes and
report nothing for an unchanged check. Mark done only after every criterion passes its oracle.
Use `safe-effects` for external mutations. To babysit a pull request,
follow [PR babysitting](references/pr-babysit.md).
