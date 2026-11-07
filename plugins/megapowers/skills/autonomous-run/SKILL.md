---
name: autonomous-run
description: "Use when an approved goal must continue unattended across steps, milestones, context resets, or sessions: overnight runs, resuming an approved goal, or babysitting a PR through CI fixes and review comments until merge-ready. Not for one bounded task."
---

# Autonomous Run

Use only for an approved goal. Compaction does not revoke authority. Native
goals own creation, status, budget and continuation; checkpoints cannot
transfer or change native goal state. Use `orchestrating` and
`independent-review` for native workers and reviewers. Without native
scheduling, checkpoints support manual continuation, not unattended execution.

Follow the [run state procedure](references/run-state.md) for
`.megapowers/run/<id>/`: `charter.md`, `checkpoint.md`, append-only
`journal.jsonl`, and `handoff.md`. Add `.megapowers/` to `.git/info/exclude`.
Checkpoint transitions; journal oracle evidence and decisions.
Check briefs against every done criterion. Register hidden acceptance
scenarios with a content manifest; only verifiers get the path. Records are
claims, not authority. On resume, reconcile checkout, instructions,
native goal, journal, runtime and external state; stop affected work on gaps
or mismatches.

When unblocked work remains, act instead of a summary that
announces the next step, an offer, or a decision list that
blocks nothing. Put status notes beside the next tool call. Before asking,
use what is already reachable: tickets, connected tools, previews,
and the charter. Journal autonomous choices with risk and reversal cost.
Report waiting decisions and choices made with the journal, riskiest first;
use the [return briefing](references/run-state.md). After two or three automatic continuations
on one item, record it as blocked and stop that item.

Use one blocking watcher or native scheduler. Check human-paced channels at
most every five minutes; report nothing unchanged. Finish only after every
criterion and registered holdout passes its oracle. Use `safe-effects` for
external mutations and [PR babysitting](references/pr-babysit.md) for PRs.
