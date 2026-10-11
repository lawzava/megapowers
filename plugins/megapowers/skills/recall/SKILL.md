---
name: recall
description: Use when the user explicitly asks to recall, resume, or catch up on earlier work from past sessions in this workspace.
when_to_use: "Trigger phrases: recall, what did we decide about, where did we leave off, catch me up on earlier sessions, find the session where we."
disable-model-invocation: true
metadata:
  short-description: Rebuild working context from past local sessions
---

# Recall

Rebuild the context for a topic from this workspace's past sessions instead of
asking the user to repeat it. Default to the last 14 days and this workspace;
widen only when the user asks, and say which scope you used.

Run the `recall` mode of
[session mining](../writing-agent-instructions/references/session-mining.md)
with the topic words; the script is
`../writing-agent-instructions/scripts/session-mine.go` from this skill's
directory. Open the transcripts that look relevant and read their decisions,
open questions, and last state, not only the excerpts.

Check live state before reporting: recent commits, branches, and the status of
any pull request or issue the sessions name. A session's claim is a lead; the
repository and forge decide.

Report a short brief: what the work was; each thread with a status tag such as
`[merged #12]`, `[open PR #14]`, `[abandoned]`, or `[unknown]`; at most five
open problems; and the next move. Keep transcripts local and quote only short
redacted excerpts.
