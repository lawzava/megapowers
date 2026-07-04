---
name: using-megapowers
description: Use when starting any conversation - establishes how to find and use skills, requiring skill invocation before any response including clarifying questions
---

If dispatched as a subagent for a specific task, ignore this skill and proceed.

## The Core Rule

Invoke any relevant or requested skill *before* your first response or action,
including clarifying questions, exploring code, or reading files. If there's a
plausible chance one applies, check it first; if it turns out wrong, skip it.
Before entering plan mode, brainstorm first.

For every skill you invoke: announce "Using [skill] to [purpose]", follow it
exactly, and make a todo per checklist item.

## Skill Priority

When several apply, process skills set the approach, then implementation skills
carry it out. "Let's build X" → brainstorming first; "fix this bug" →
systematic-debugging first. For structuring non-trivial work (split, delegate,
parallelize, run long), mega-orchestration:orchestrating is the decision root
when that plugin is installed.

## Don't Skip on a Hunch

Simple-looking tasks are where skills get skipped. "Just a quick question", "let
me look first", "overkill here": each means check for a skill first. Memory
drifts: open the skill, don't recall it.

## Communication

Senior engineer's register for anything a human or agent reads:

- Lead with the conclusion; detail follows, by importance.
- Short declarative sentences. No filler, no hedging, no drama.
- No dash punctuation (no em dashes, no double hyphens): use commas, colons,
  or parentheses.
- Enumerables: a short list or table, not a paragraph.
- Write for takeover: state goal, current state, next step; skip unseen
  conversation.

## Platform Adaptation

If your harness appears here, read its reference for tool specifics: Codex
`references/codex-tools.md`, Pi `references/pi-tools.md`, Antigravity
`references/antigravity-tools.md`, OpenCode `references/opencode-tools.md`.

## User Instructions

User instructions (CLAUDE.md, AGENTS.md, direct requests) take precedence over
skills, which override default behavior. Skip a skill only when explicitly told
to.
