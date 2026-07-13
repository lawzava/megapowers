---
name: dispatching-parallel-agents
description: Use for two or more independent tasks that can run concurrently in separate agents. Triggers on "parallelize", "fan out", or "agent for each". Not for dependent steps or shared state.
license: MIT
---

# Dispatching Parallel Agents

## Overview

You delegate tasks to agents with isolated context. Each agent receives exactly the context it needs, constructed by you; it never inherits your session's history. That keeps agents focused and preserves your own context for coordination. When several tasks are independent, one agent per task working concurrently beats working them in sequence.

## When to Use

Walk the gate in order:

1. Multiple tasks? If not, this skill does not apply.
2. Independent? If one task's outcome could resolve or reshape another, a single agent handles them together.
3. Parallel-safe? If they would share state and interfere, dispatch agents sequentially. Otherwise dispatch in parallel.

Two neighbors to rule out first. Executing a written implementation plan belongs to megapowers:subagent-driven-development: plan tasks share one branch and working tree, so they run sequentially, never as parallel implementers; parallelize only disjoint work, each agent in its own worktree. Needing a different model or runtime rather than same-model parallelism is mega-orchestration:multi-agent-delegation (if installed), with mega-orchestration:orchestrating as the decision root when the right structure is unclear.

Also skip this skill when understanding the work requires seeing the whole system, or when the work is still exploratory and you do not yet know how it decomposes.

## Dispatching

Group the work by what is actually separate, then dispatch one agent per domain. Multiple dispatch calls in one response run in parallel. One per response runs sequentially.

Every agent prompt satisfies three requirements:

1. Focused: one clearly scoped task, with constraints on what not to touch.
2. Self-contained: everything needed to locate the problem, meaning paths, identifiers, and error messages. A document the agent needs goes in as its path plus an instruction to read it; quote verbatim only what the agent must match exactly. Never rely on the agent inheriting session context; it has none.
3. Explicit about output: state what the agent should return.

A prompt missing any of these produces an agent that wanders, guesses, or returns something you cannot integrate.

## Worked Example

Six test failures across three files after a refactoring: abort logic, batch completion, approval race conditions. The domains are independent and share no state, so three agents go out in one response. Each is scoped to one file, given its failing test names and error messages, constrained to fix root causes rather than pad timeouts, and asked to return a summary of what it found and changed. The fixes land without conflict and the full suite goes green in the time one fix would have taken.

## After Agents Return

1. Review each summary and understand what changed.
2. Check for conflicts between agents' edits.
3. Run the full verification suite; the results must hold together, not just per agent.
4. Spot-check the work; agents can make systematic errors.

You own integration.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
