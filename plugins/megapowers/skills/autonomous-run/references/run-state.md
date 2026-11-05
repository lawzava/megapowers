# Run state

Use normal file and Git tools to maintain `.megapowers/run/<id>/`.
Keep it outside version control with a local exclude entry. Refuse to overwrite
an existing run. Keep the charter, checkpoint, append-only journal, and handoff
private. If files cannot be written, keep the charter and latest checkpoint
in the conversation.

- Initialize a charter with the objective and explicit done criteria (C1, C2,
  and so on). Complete placeholders before delegating.
- Checkpoint UTC time, workspace, branch, worktree, HEAD, dirty state, milestone,
  evidence, blockers, and next safe action. Refresh the handoff.
- Append journal evidence, decisions and reasons. For an autonomous decision,
  record risk (low, medium, high) and reversal cost. Review high risks first.
- On resume, compare each saved checkout field with live Git state. Reconcile
  mismatches before affected work. Inspect the journal tail, blockers, next
  action, runtime state, external state, artifact identities, delegate ownership,
  and remaining effect authority.
- Count continuations for the same item across context resets. Use a default
  cap of three unless agreed otherwise. At the cap, record the item as blocked
  and report it instead of silently restarting its count.
- Check that each implementer or verifier brief names every done criterion
  without changing its meaning. A matching ID alone is insufficient.
- Keep holdout scenarios outside the repository and worktree. Record their
  canonical location and a SHA-256 manifest of relative paths and contents.
  Reject empty sets, symlinks and non-regular files. Compare the manifest on
  resume. Rebaseline only with user approval, recording a high-risk decision.
  Keep holdout paths and scenario contents out of implementer briefs; only the
  verifier receives them. Path checks do not prevent an agent reading content.

Stop affected work and reconcile missing criteria, leaked holdouts, or changed
holdout bytes. Record oracle results, not intent. Never rewrite the journal.

## Portable record format

Use run IDs matching `[a-z0-9][a-z0-9-]{0,62}`. Checkpoint metadata uses
`- Time:`, `- Workspace:`, `- Branch:`, `- Worktree:`, `- HEAD:`, `- Dirty:`
and `- Milestone:` lines. Keep Evidence, Blockers and Next safe action sections.
Use canonical workspace and worktree paths and immutable commit identities.

Write each journal entry as one JSON object per line. Required fields are
`ts` (UTC timestamp), `event`, `evidence` and `head`. Include `branch`,
`worktree`, `workspace` and `next` when relevant. Event names `oracle`,
`continuation`, `blocked` and `holdout` preserve scorecard counts. Use `item`
for the stable continuation item. Record `decision`, `why`, `risk` and
`if_wrong` for choices; `risk: "high"` identifies high-risk decisions.
Holdout registration records use `holdout_dir` and `holdout_sha256`.
For an approved rebaseline, use a `holdout` event with `decision`, `why` and
`risk: "high"` so scorecards also count the rebaseline.
Keep the journal append-only and verify holdout evidence independently.
