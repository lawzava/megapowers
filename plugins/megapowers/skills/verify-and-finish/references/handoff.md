# Handoff notes

Write a handoff note only when the work must travel: to another harness, such as
Claude Code to Codex; to another directory or repository; to a colleague; or to
a side task forked mid-task. When the same session can continue, continue it.
An approved autonomous goal keeps its handoff in `autonomous-run` state instead.

Save the note outside the working tree, under the user's temporary directory,
and report its path. Write it for an agent that has never seen this session:

- the outcome sought and the done criteria;
- the current state: repository, branch or worktree, HEAD, and uncommitted
  changes;
- verified facts with their evidence, labeled separately from assumptions and
  open questions;
- settled decisions and the reason for each, especially ones a diff cannot
  reveal;
- the next safe action, and blockers with what would clear them;
- skills the next agent should load;
- authority that was granted, and what still needs approval. A note records
  claims, not authority; the next session verifies them against current
  instructions.

Reference specifications, plans, issues, commits, and diffs by path or
identifier instead of copying them. Leave out secrets, tokens, personal data,
and raw transcripts.
