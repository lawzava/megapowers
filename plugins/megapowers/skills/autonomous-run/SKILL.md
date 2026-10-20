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

Keep state in `.megapowers/run/<id>/` with the
[run state tool](references/run-state.md), which adds `.megapowers/` to
`.git/info/exclude`: `charter.md` (objective, done criteria, scope, authority,
cap), `checkpoint.md` (milestone, branch, worktree, HEAD, evidence, blockers,
next safe action), append-only `journal.jsonl`, and `handoff.md`. Checkpoint at
milestones or transitions, journal oracle evidence and decision reasons, and
check briefs with `coverage`. Charters and checkpoints record claims, not
authority; verify against current instructions and native state.

On resume, run `resume`, read repository instructions, native goal state, and
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
`decisions`, riskiest first. After two or three automatic
continuations on the same item, record it as blocked for review; `continue`
enforces this.

Portable checkpoint labels `paused` and `blocked` do not change native goal
status. To watch for a change, run one blocking watcher that exits on state
change, or a harness scheduler; check human-paced channels at most every five
minutes and report nothing when unchanged. Mark done only after every criterion
passes its oracle. Use `safe-effects` for external mutations and
[PR babysitting](references/pr-babysit.md) for pull requests.
