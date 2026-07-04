---
name: dispatching-parallel-agents
description: Use when you have 2+ independent tasks with no shared state or ordering and want them done in parallel by separate agents — "do these at the same time", "parallelize", "fan out", "spin up an agent for each", bulk independent edits across files or modules. Not for sequential or dependent steps.
---

# Dispatching Parallel Agents

## Overview

You delegate tasks to specialized agents with isolated context. By precisely crafting their instructions and context, you keep them focused and set them up to succeed. They should never inherit your session's context or history — you construct exactly what they need. This also preserves your own context for coordination work.

When you have several independent tasks — separate files to edit, separate subsystems to build, separate questions to research — working them sequentially wastes time. Each task stands on its own and can happen in parallel.

**Core principle:** dispatch one agent per independent task and let them work concurrently.

## When to Use

Walk the decision like this:

1. Do you have multiple tasks? If not, this skill doesn't apply.
2. Are they independent? If one task's outcome could change another (finishing one might resolve or reshape another), have a single agent handle them together.
3. If they're independent, can they work in parallel? If they'd share state and interfere, dispatch agents sequentially. Otherwise dispatch them in parallel.

Concrete signs it fits:

- Bulk independent edits across different files or modules
- Multiple subsystems to build or fix, each self-contained
- Each task can be understood without context from the others

Two neighbors to rule out first. Executing a written implementation plan?
Plan tasks share one branch and working tree, so they run sequentially under
subagent-driven-development, never as parallel implementers; parallelize only
truly disjoint work, each agent in its own worktree. Need a different model or
runtime rather than same-model parallelism? That is
mega-orchestration:multi-agent-delegation (if installed), and its
orchestrating skill is the decision root when the right structure is unclear.

Debugging is one common instance: several unrelated test failures (different files, different subsystems, different bugs) are just independent tasks, so they parallelize the same way. That case runs as the illustrative example below.

## The Pattern

### 1. Identify Independent Domains

Group the work by what's actually separate. For a batch of failing tests, that might be:

- File A tests: Tool approval flow
- File B tests: Batch completion behavior
- File C tests: Abort functionality

Each domain is independent — fixing tool approval doesn't affect abort tests.

### 2. Create Focused Agent Tasks

Each agent gets:

- **Specific scope:** one file, subsystem, or task
- **Clear goal:** what "done" looks like
- **Constraints:** what not to touch
- **Expected output:** summary of what you found and did

### 3. Dispatch in Parallel

Issue all the subagent dispatches in the same response — they run in parallel:

```text
Subagent (general-purpose): "Fix agent-tool-abort.test.ts failures"
Subagent (general-purpose): "Fix batch-completion-behavior.test.ts failures"
Subagent (general-purpose): "Fix tool-approval-race-conditions.test.ts failures"
# All three run concurrently.
```

Multiple dispatch calls in one response run in parallel. One per response runs sequentially.

On a harness with resumable subagents (Claude Code's SendMessage), send follow-up work to the same agent instead of re-dispatching a fresh one with a recap; a resumed subagent keeps its full history. Agents that need identical starting context can be forks, which inherit the conversation and share the parent's prompt cache, so they are cheaper than fresh subagents.

### 4. Review and Integrate

When agents return:

- Read each summary
- Verify the results don't conflict
- Run the full verification (e.g. the test suite)
- Integrate all changes

## Agent Prompt Structure

Good agent prompts are:

1. **Focused** — one clear task
2. **Self-contained** — all context needed to understand the problem
3. **Specific about output** — what should the agent return?

```markdown
Fix the 3 failing tests in src/agents/agent-tool-abort.test.ts:

1. "should abort tool with partial output capture" - expects 'interrupted at' in message
2. "should handle mixed completed and aborted tools" - fast tool aborted instead of completed
3. "should properly track pendingToolCount" - expects 3 results but gets 0

These are timing/race condition issues. Your task:

1. Read the test file and understand what each test verifies
2. Identify root cause - timing issues or actual bugs?
3. Fix by:
   - Replacing arbitrary timeouts with event-based waiting
   - Fixing bugs in abort implementation if found
   - Adjusting test expectations if testing changed behavior

Do not just increase timeouts - find the real issue.

Return: Summary of what you found and what you fixed.
```

## Common Mistakes

- Too broad: "Fix all the tests" — the agent gets lost. Instead scope it: "Fix agent-tool-abort.test.ts".
- No context: "Fix the race condition" — the agent doesn't know where. Instead paste the error messages and test names.
- No constraints: the agent might refactor everything. Instead constrain it: "Do not change production code" or "Fix tests only".
- Vague output: "Fix it" — you don't know what changed. Instead ask for a specific return: "Return summary of root cause and changes".

## When Not to Use

- **Dependent tasks:** finishing one could resolve or reshape another — handle them together first.
- **Need full context:** understanding requires seeing the entire system.
- **Exploratory work:** you don't yet know how the work decomposes.
- **Shared state:** agents would interfere (editing same files, using same resources).

## Illustrative Example

Scenario: 6 test failures across 3 files after a major refactoring.

Failures:

- agent-tool-abort.test.ts: 3 failures (timing issues)
- batch-completion-behavior.test.ts: 2 failures (tools not executing)
- tool-approval-race-conditions.test.ts: 1 failure (execution count = 0)

Decision: independent domains — abort logic is separate from batch completion is separate from race conditions.

Dispatch, all in one response:

```text
Agent 1 → Fix agent-tool-abort.test.ts
Agent 2 → Fix batch-completion-behavior.test.ts
Agent 3 → Fix tool-approval-race-conditions.test.ts
```

Results:

- Agent 1: Replaced timeouts with event-based waiting
- Agent 2: Fixed event structure bug (threadId in wrong place)
- Agent 3: Added wait for async tool execution to complete

Integration: all fixes independent, no conflicts, full suite green — 3 problems solved in parallel instead of sequentially, with zero conflicts between agent changes.

## Key Benefits

1. **Parallelization** — multiple tasks happen simultaneously
2. **Focus** — each agent has narrow scope, less context to track
3. **Independence** — agents don't interfere with each other
4. **Speed** — 3 tasks done in the time of 1

## Verification

After agents return:

1. **Review each summary** — understand what changed
2. **Check for conflicts** — did agents edit the same code?
3. **Run full verification** — confirm all results hold together
4. **Spot check** — agents can make systematic errors
