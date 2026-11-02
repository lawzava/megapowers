---
name: close-off
description: Use only when the user invokes it at the end of a session, to confirm the task is complete, record what the session learned, and clean up what the session started.
when_to_use: "Trigger phrases: close off, close off this session, close out the session, end of session cleanup, wrap up and clean up."
disable-model-invocation: true
metadata:
  short-description: End-of-session completion check, learnings, and cleanup
---

# Close Off

The user invokes this skill to end the session. Never archive, delete, or close
the current session, and never ask the user to do more work. Report readiness
only.

## 1. Confirm the task is complete

Apply [verify-and-finish](../verify-and-finish/SKILL.md) to the session's task.
If in-scope work remains, do it. If something blocks, report the exact blocker
and what it needs. Then stop: skip steps 2 to 4 and never say the session is
ready.

## 2. Record the learnings

Write what this session taught that a later session needs into the learning
surface the project defines: its instruction files, learning log, or native
memory. Choose the target and evidence as in the
[learning loop](../writing-agent-instructions/references/learning-loop.md).
Invoking this skill approves lesson entries there, not a commit or push. Keep
only durable, evidenced lessons, not session history. If the project defines no
surface, skip this step and say so.

## 3. Clean up what this session created

Remove only items this session created and can identify by a recorded PID, ID,
or path:

- background processes, shells, local servers, and daemons;
- subagents and orchestrator sessions it started;
- temporary worktrees with no uncommitted work, never their branches;
- scratch files under `$TMPDIR`.

Read back each removal. Never stop or delete anything this session did not
create, anything holding uncommitted work, or anything the instructions say to
ask about first. List each such item with its reason instead.

## 4. Close

Report in a few lines: the completion evidence, lessons written and where (or
skipped), items removed, and items left with reasons. End the reply with
exactly this line and nothing after it:

It's ready for archiving.
