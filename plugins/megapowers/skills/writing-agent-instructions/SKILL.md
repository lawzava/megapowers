---
name: writing-agent-instructions
description: Use when creating or revising an agent skill, AGENTS.md, CLAUDE.md, or nested project instructions, including trigger design, progressive disclosure, harness-aware scope, and behavioral validation.
when_to_use: "Trigger phrases: write a skill, create a skill instruction, improve AGENTS.md, audit CLAUDE.md, project instructions, subfolder instructions, skill trigger."
metadata:
  short-description: Write focused skills and repository instructions
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

Write every new deterministic helper or policy tool in Go.

For a skill, read [skill authoring](references/skills.md). For project or
subfolder guidance, read
[repository instructions](references/repository-instructions.md). Read both
only when the task changes both surfaces.

Validate a new skill or material behavior change at two boundaries. First,
confirm the target harness discovers the file and accepts its format. Then
exercise realistic positive, near-miss negative, and pressure cases. Compare
the same cases against the previous instructions or no-skill baseline. Inspect
task outcomes as well as activation, and keep the change only when it improves
the intended behavior without a material regression outside its scope. For a
typo, link repair, or similarly small correction that does not change behavior,
use proportional static checks; do not require baseline model runs.
