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
budget, and continuation; this skill adds only the portable state below. Native
goal state does not transfer across harnesses.

Use native workers/reviewers through `orchestrating`/`independent-review`.
Without native scheduling, checkpoints support manual continuation, not an
unattended runtime; follow the run-state procedure.

Keep state in `.megapowers/run/<id>/` with the
[run state procedure](references/run-state.md). Add `.megapowers/` to
`.git/info/exclude`: `charter.md` (objective, done criteria, scope, authority,
cap), `checkpoint.md` (milestone, branch, worktree, HEAD, evidence, blockers,
next safe action), append-only `journal.jsonl`, and `handoff.md`. Checkpoint at
milestones or transitions, journal oracle evidence and decision reasons, and
check briefs against every done criterion. Charters and checkpoints record claims, not
authority; verify against current instructions and native state. Register
hidden acceptance scenarios with a content manifest; only verifiers get the path.

On resume, compare the saved checkpoint with the live checkout, read repository instructions, native goal state, and
the journal tail, and reconcile runtime and external state. Stop affected work
on missing evidence or a mismatch.

While work is owed and nothing blocks it, do not end a turn with a summary that
announces the next step, an offer to continue, a decision list that blocks
nothing, or a milestone report. Put status notes in the same message as the
next tool call. Before asking, use what is already reachable: linked pull
requests and tickets, authenticated CLIs, connected MCP tools, environment key
names (never values), previews, and the charter. Journal each choice made
without the user with its risk and cost if wrong. When the user returns, open
with the count of waiting decisions, then why now, options with costs, a
recommendation, and what happens if unanswered; then list choices made with
the journal, riskiest first. After two or three automatic continuations on
one item, record it as blocked and stop that item.

Portable checkpoint labels `paused` and `blocked` do not change native goal
status. Watch for change with one blocking watcher or a harness scheduler;
check human-paced channels at most every five minutes and report nothing when
unchanged. Mark done only after every criterion and registered holdout passes
its oracle. Use `safe-effects` for external mutations and
[PR babysitting](references/pr-babysit.md) for pull requests.
