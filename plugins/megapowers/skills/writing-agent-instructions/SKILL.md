---
name: writing-agent-instructions
description: "Use when creating, revising, or auditing skills, AGENTS.md, CLAUDE.md, nested project instructions, or native harness memory. Covers skill triggers, progressive disclosure, harness-aware scope, behavioral validation, session retrospectives, auditing or pruning stale memory, and turning repeated agent mistakes into instruction or tooling changes."
---

# Writing Agent Instructions

Choose the smallest instruction surface for the work:

- Put durable repository facts, commands, boundaries, and conventions in the
  nearest applicable project instruction file.
- Put reusable task judgment or a multi-step capability in a skill.
- Put rules that must execute or block an action in tests, hooks, settings, or
  a deterministic tool. Prose can guide a model but cannot enforce behavior.

Read applicable repository instructions, nearby instructions, code, tests, and
configured tools before drafting. Attribute load-bearing statements to direct
observation, repository policy, upstream documentation, or explicit author
decisions. Remove claims that code or a command can reveal cheaply unless
their omission repeatedly causes bad decisions.

Write the minimum instruction that changes behavior. State the condition, the
required outcome, and any real boundary. Preserve user intent and existing
authorization. Say what to use ("use Y for Z"); a "do not use X" line steers
toward X and blocks edge cases where X fits. State real safety and authority
limits directly. Mark superseded routes legacy and say when to ask. Keep
examples, preferences, and hypothetical risks out of blanket prohibitions,
approval gates, fixed workflows, and extra files. Add detail only when omitting
it caused an observed failure or creates a concrete risk.

For a skill, read [skill authoring](references/skills.md). For project or
subfolder guidance, read
[repository instructions](references/repository-instructions.md). For a
retrospective, read [retrospective](references/retrospective.md); to audit native
memory, [memory hygiene](references/memory-hygiene.md).
Read multiple references only when the task spans more than one surface.

Validate new skills or material behavior changes at two boundaries: harness
discovery and format acceptance, then realistic positive, near-miss negative,
and pressure cases. Compare the same cases against the previous instructions
or no-skill baseline. Inspect task outcomes as well as activation, and keep the
change only when it improves the intended behavior without a material
regression outside its scope. For a typo or link fix that does not change
behavior, static checks suffice.
