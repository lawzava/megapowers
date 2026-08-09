---
name: writing-skills
description: Use when creating new skills, editing existing skills, or verifying skills work before deployment
license: MIT
---

# Writing Skills

## Overview

Writing skills is evidence-driven documentation design. Define the outcome a
skill should improve, evaluate representative tasks, make the smallest
justified change, and keep the evidence with the skill.

**Core principle:** judge a skill by honest task outcomes, not forced
obedience.

Required background: megapowers:test-driven-development defines a useful cycle
for executable changes. For current portable OpenAI and Anthropic guidance, see
authoring-best-practices.md. For the research on phrasing that lands, see
effective-phrasing.md. When editing an existing skill down,
de-prescription-rubric.md is the trim standard: it says what to remove, what to
keep, and what never to touch.

## What a Skill Is

A reusable reference for a proven technique, pattern, or tool. Not a narrative
about how you solved a problem once. Create one when the technique was not
obvious to you, applies beyond one project, and will be needed again. Skip
one-off fixes, practices well documented elsewhere, project-specific
conventions (those belong in the project's instructions file), and mechanical
constraints (automate those with validation; save documentation for judgment
calls).

Structure: a SKILL.md, plus supporting files only for reusable tools or
reference material too heavy to inline. The body covers an overview with the
core principle, when to use and when not, the pattern itself, one example, and
common mistakes. Label things by semantic meaning, not helper1 and step3, and
put code in markdown blocks rather than flowchart nodes.

## Frontmatter and Description

Frontmatter requires `name` and `description` (see
[agentskills.io/specification](https://agentskills.io/specification)). Limits
are per field: `name` max 64 characters, letters, numbers, and hyphens only;
`description` max 1024 characters, under 500 where possible.

The description is the trigger surface: an agent reads it to decide whether to
load the skill. Write it in third person, start with "Use when", and describe
only triggering conditions: concrete symptoms, situations, and error text,
technology-agnostic unless the skill itself is technology-specific. Never
summarize the skill's process or workflow: an agent that reads the workflow in
the description follows that summary and skips the body.

```yaml
# Avoid: workflow summary agents will follow instead of reading the body
description: Use when executing plans, dispatching a subagent per task with review between tasks

# Prefer: triggering conditions only
description: Use when tests have race conditions, timing dependencies, or pass/fail inconsistently
```

## Discoverability and Word Budget

Name skills verb-first; gerunds work well (creating-skills,
root-cause-tracing). Seed the body with terms an agent would search: error
messages, symptoms, tool names, synonyms. An agent finds a skill by matching
its problem against descriptions, skims the overview, and loads examples only
when implementing; put searchable terms early. Cross-reference other skills by
name with a requirement marker (`**Required background:**
megapowers:systematic-debugging`), never by `@` path, which force-loads the
file into context.

Keep loaded guidance concise and move optional detail to references. Use `wc
-w` as a diagnostic, not a universal target. Prefer clear words over invented
abbreviations. One complete, runnable example from a real scenario is usually
more useful than many shallow variants.

Do not invent impact statistics. A claim of effect needs a measured run behind
it (see this repo's `evals/`); unsourced percentages get repeated to users as
fact.

## Evidence Rule

Behavioral guidance needs evidence appropriate to its risk before shipping. Use
an executable regression when the behavior has one; otherwise use an honest
representative task evaluation with a clear oracle. Mechanical and editorial
edits need a correctness check. Record uncertainty instead of disguising it as
a passing evaluation.

## Test Before Shipping

Evaluate every behavioral change before deployment. Start with a representative
task and observable oracle. Use a no-guidance, prior-guidance, or alternative
control when it can distinguish the new wording from normal performance. Do not
disguise an evaluation as live work, force a chosen answer, or mistake a skill
quotation for successful task completion.

Scale evidence to risk: mechanical and editorial changes need correctness
checks; techniques need representative application; workflow and safety rules
need ordering, consent, and verification evidence; high-impact rules need
boundary cases and independent review when available. Preserve inconclusive
results and revise only for demonstrated gaps.

Full methodology and evidence templates: see
[testing-skills-with-subagents.md](testing-skills-with-subagents.md).

## Shipping

The finished skill lives in a discoverable skills directory, ready to use.
Commit it only when the human directs; committing or pushing is never a side
effect of authoring a skill. Consider contributing broadly useful skills back
via PR.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
