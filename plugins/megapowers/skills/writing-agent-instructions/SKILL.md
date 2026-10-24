---
name: writing-agent-instructions
description: Use when creating, revising, or auditing an agent skill, AGENTS.md, CLAUDE.md, nested project instructions, or native harness memory, including trigger design, progressive disclosure, harness-aware scope, behavioral validation, past-session recall, session retrospectives that turn agent mistakes into instruction or tooling changes, and the per-project learning loop that scores skill outcomes and promotes or retires lessons.
when_to_use: "Trigger phrases: write a skill, create a skill instruction, improve AGENTS.md, audit CLAUDE.md, project instructions, subfolder instructions, skill trigger, run a retro on this session, what should we change so the agent avoids this next time, audit memory, prune stale memories, recall earlier sessions, what is megapowers doing for this project, learn from past sessions, score skill outcomes, review lessons, retire a lesson."
metadata:
  short-description: Write focused skills, instructions, and memory audits
---

# Writing Agent Instructions

Choose the smallest instruction surface that reaches the intended work:

- Put durable repository facts, commands, boundaries, and conventions in the
  nearest applicable project instruction file.
- Put reusable task judgment or a multi-step capability in a skill.
- Put rules that must execute or block an action in tests, hooks, settings, or
  a deterministic tool. Prose can guide a model but cannot enforce behavior.

Read the applicable repository instructions, nearby instruction files, code,
tests, and configured tools before drafting. For every load-bearing statement,
identify its source as direct observation, repository policy, upstream
documentation, or an explicit author decision. Remove claims that code or a
command can reveal cheaply unless their omission would repeatedly cause a bad
decision.

Write the minimum instruction that changes behavior. State the condition, the
required outcome, and any real boundary. Preserve user intent and existing
authorization. Do not turn examples, preferences, or hypothetical risks into
blanket prohibitions, approval gates, fixed workflows, or extra files. Add
detail only when omitting it caused an observed failure or creates a concrete
risk.

For a skill, read [skill authoring](references/skills.md). For project or
subfolder guidance, read
[repository instructions](references/repository-instructions.md). For
user-level instructions that apply to every repository, read
[global instructions](references/global-instructions.md), which links the
maintained templates. For a retrospective, read
[retrospective](references/retrospective.md); to recall past sessions or mine
corrections, [session mining](references/session-mining.md); to audit native
memory, [memory hygiene](references/memory-hygiene.md); to score a project's
skill outcomes or manage lessons, [learning loop](references/learning-loop.md).
Read more than one reference only when the task spans more than one surface.

Validate a new skill or material behavior change at two boundaries. First,
confirm the target harness discovers the file and accepts its format. Then
exercise realistic positive, near-miss negative, and pressure cases. Compare
the same cases against the previous instructions or no-skill baseline. Inspect
task outcomes as well as activation, and keep the change only when it improves
the intended behavior without a material regression outside its scope. For a
typo, link repair, or similarly small correction that does not change behavior,
use proportional static checks; do not require baseline model runs.
