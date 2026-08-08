---
name: using-megapowers
description: Use when starting a conversation or task, before any response including clarifying questions. Triggers on any new user request. Not for a subagent dispatched with a specific task.
license: MIT
---

If dispatched as a subagent for a specific task, ignore this skill and proceed.

## The Core Rule

A skill that covers the task owns the procedure. Open it and follow it rather
than working from your memory of what it says. A requested skill always loads.
Mechanical edits, single-fact lookups, and ordinary conversation need none.

Announce the outer workflow once, with its purpose and route. Nested skills are
internal steps and need no separate announcement unless the route materially
changes. Do not expand skill checklists or plan checkboxes into duplicate
todos. Use one durable progress surface for work that needs one.

## Skill Priority

Process skills set the approach. If scope, area, oracle, and risk are clear,
take the scoped fast path through test-driven development. Ambiguous features
start with `megapowers:brainstorming`; unknown failures start with
`megapowers:systematic-debugging`.
For non-trivial structure, use mega-orchestration:orchestrating when installed.

## Communication

Senior engineer's register for anything a human or agent reads:

- Lead with the conclusion, then important detail.
- Use short declarative sentences, no filler or drama.
- No dash punctuation (`—`, `–`, spaced `--`). Use a comma, colon, parentheses,
  new sentence, or rewrite. Scan before sending.
- Backtick a character you discuss, not use: tooling cannot tell them apart.
- For takeover, state goal, current state, and next step. Use a short list or
  table for enumerables.

## Platform Adaptation

For environment-specific tool details, read the relevant adapter reference:
[Codex](references/codex-tools.md) or [OpenCode](references/opencode-tools.md).
Resolve relative files and helpers from the directory of the skill that names
them. Resolve a cross-skill helper from the target skill's installed directory.

## User Instructions

User instructions (CLAUDE.md, AGENTS.md, direct requests) take precedence over
skills, which override default behavior. Skip a skill only when explicitly told
to.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
