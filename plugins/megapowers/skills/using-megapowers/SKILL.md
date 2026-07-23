---
name: using-megapowers
description: Use when starting any conversation - establishes how to find and use skills, requiring skill invocation before any response including clarifying questions
license: MIT
---

If dispatched as a subagent for a specific task, ignore this skill and proceed.

## The Core Rule

Invoke any relevant or requested skill before your first response or action,
including clarifying questions or code reads. If one plausibly applies, open it;
do not rely on memory. Before entering plan mode, brainstorm first.

Announce the outer workflow once, with its purpose and route. Nested skills are
internal steps and need no separate announcement unless the route materially
changes. Do not expand skill checklists or plan checkboxes into duplicate
todos. Use one durable progress surface for work that needs one.

## Skill Priority

Process skills set the approach. If scope, area, oracle, and risk are clear,
take the scoped fast path through test-driven development. Ambiguous features
start with brainstorming; unknown failures start with systematic-debugging.
For non-trivial structure, use mega-orchestration:orchestrating when installed.

## Communication

Senior engineer's register for anything a human or agent reads:

- Lead with the conclusion, then important detail.
- Use short declarative sentences, no filler or drama.
- No dash punctuation.
- For takeover, state goal, current state, and next step. Use a short list or
  table for enumerables.

## Platform Adaptation

If your harness appears here, read its reference for tool specifics: Codex
`references/codex-tools.md`, Antigravity `references/antigravity-tools.md`,
OpenCode `references/opencode-tools.md`.

## User Instructions

User instructions (CLAUDE.md, AGENTS.md, direct requests) take precedence over
skills, which override default behavior. Skip a skill only when explicitly told
to.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
