---
name: close-off
description: "Use only when the user explicitly asks to close off the session to confirm completion, record learnings, and clean up what the session started. Covers close off, close out the session, end-of-session cleanup, and wrap up and clean up."
---

# Close Off

The user invokes this skill to end the session. Never archive, delete, or close
the current session, and never ask the user to do more work. Report readiness
only.

## 1. Confirm the task is complete

Apply [verify-and-finish](../verify-and-finish/SKILL.md) to the session's task.
If in-scope work remains, do it, then run verify-and-finish again. If
something blocks, report the exact blocker and what it needs, skip steps 2 to
4, and never say the session is ready.

## 2. Record the learnings

Write what this session taught that a later session needs into the learning
surface the project defines: its instruction files, learning log, or native
memory. Choose the narrowest existing target and cite direct observations
from this session. Revalidate facts that can change before recording them.
Invoking this skill approves lesson entries there, not a commit or push. Edit
only lesson entries, and keep only durable, evidenced lessons, not session
history. If the session taught none, or the project defines no surface, skip
this step and say so.

## 3. Clean up what this session created

Remove only items this session created and recorded when it created them (PID,
ID, or path). Before signalling a PID, confirm it is still the process this
session started. Targets:

- background processes, shells, local servers, and daemons;
- subagents and orchestrator sessions it started;
- temporary worktrees that are clean, whose commits are reachable from a
  branch, and that are not the current directory; never their branches;
- scratch files under `$TMPDIR` (skip when it is unset).

Read back each removal. Never stop or delete anything this session did not
create, anything holding uncommitted work, or anything the instructions say to
ask about first. List each such item with its reason instead.

## 4. Close

Report in a few lines: the completion evidence, lessons written and where (or
skipped), items removed, and items left with reasons. End the reply with
exactly this line and nothing after it:

It's ready for archiving.
