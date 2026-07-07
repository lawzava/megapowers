---
name: writing-skills
description: Use when creating new skills, editing existing skills, or verifying skills work before deployment
license: MIT
---

# Writing Skills

## Overview

Writing skills is Test-Driven Development applied to process documentation. You write pressure scenarios (test cases), watch a baseline agent fail without the skill (RED), write the minimal skill that fixes those failures (GREEN), and close the loopholes testing exposes (REFACTOR).

**Core principle:** if you didn't watch an agent fail without the skill, you don't know whether the skill teaches the right thing.

Required background: megapowers:test-driven-development defines the RED-GREEN-REFACTOR cycle this skill adapts. For Anthropic's authoring guidance, see anthropic-best-practices.md. For the research on phrasing that lands, see effective-phrasing.md. When editing an existing skill down, de-prescription-rubric.md is the trim standard: it says what to remove, what to keep, and what never to touch.

## What a Skill Is

A reusable reference for a proven technique, pattern, or tool. Not a narrative about how you solved a problem once. Create one when the technique was not obvious to you, applies beyond one project, and will be needed again. Skip one-off fixes, practices well documented elsewhere, project-specific conventions (those belong in the project's instructions file), and mechanical constraints (automate those with validation; save documentation for judgment calls).

Structure: a SKILL.md, plus supporting files only for reusable tools or reference material too heavy to inline. The body covers an overview with the core principle, when to use and when not, the pattern itself, one example, and common mistakes. Label things by semantic meaning, not helper1 and step3, and put code in markdown blocks rather than flowchart nodes.

## Frontmatter and Description

Frontmatter requires `name` and `description` (see [agentskills.io/specification](https://agentskills.io/specification)). Limits are per field: `name` max 64 characters, letters, numbers, and hyphens only; `description` max 1024 characters, under 500 where possible.

The description is the trigger surface: an agent reads it to decide whether to load the skill. Write it in third person, start with "Use when", and describe only triggering conditions: concrete symptoms, situations, and error text, technology-agnostic unless the skill itself is technology-specific. Never summarize the skill's process or workflow. In testing, a description that summarized the workflow made agents follow the description and skip the body, one review instead of the skill's mandated two; rewriting it to triggers only restored full compliance.

```yaml
# Avoid: workflow summary agents will follow instead of reading the body
description: Use when executing plans, dispatching a subagent per task with review between tasks

# Prefer: triggering conditions only
description: Use when tests have race conditions, timing dependencies, or pass/fail inconsistently
```

## Discoverability and Word Budget

Name skills verb-first; gerunds work well (creating-skills, root-cause-tracing). Seed the body with terms an agent would search: error messages, symptoms, tool names, synonyms. An agent finds a skill by matching its problem against descriptions, skims the overview, and loads examples only when implementing; put searchable terms early. Cross-reference other skills by name with a requirement marker (`**Required background:** megapowers:systematic-debugging`), never by `@` path, which force-loads the file into context.

Every skill costs context each time it loads, so the word budget tightens where the skill loads most often: always-in-context material under 200 words, getting-started workflows under 150 words each, other skills as lean as the material allows with heavy reference pushed to files loaded on demand. Verify with `wc -w` rather than eyeballing. One excellent, complete, runnable example from a real scenario beats implementations in five languages; agents port well.

Do not invent impact statistics. A claim of effect needs a measured run behind it (see this repo's `evals/`); unsourced percentages get repeated to users as fact.

## The Core Rule

No behavioral guidance without a failing test first. If a change adds or alters what the skill tells an agent to do (a rule, prohibition, recipe, or conditional) and you wrote it before testing, delete it and restart the cycle; keeping it as reference or adapting it while the tests run is the same violation. Mechanical and editorial edits (typos, broken links, meaning-preserving rewording) carry no behavioral hypothesis and need only a correctness check. When unsure which kind an edit is, treat it as behavioral.

## Match the Form to the Failure

Classify the baseline failure before writing guidance; the form that fixes one failure type measurably backfires on another.

- Agent skips a rule under pressure: prohibition plus rationalization counters and red flags (see Bulletproofing).
- Output complies but has the wrong shape: a positive recipe stating what the output is, its parts in order. In head-to-head wording tests, the prohibition arm produced clearly more of the unwanted content than the recipe arm, with fully separated distributions, and trended worse than the no-guidance control.
- Required element omitted: a structural slot in the template the agent fills, not a prose reminder nearby.
- Behavior depends on a condition: a conditional keyed to an observable predicate, not an unconditional rule with exemption clauses.

No nuance clauses: appending one to a winning recipe degraded it from consistent to noisy in the same tests; express a real exception as its own conditional. Exemption clauses do not scope ("this limit doesn't apply to code blocks" still suppresses code blocks); restructure so the rule cannot reach the exempt part.

## Bulletproofing Discipline Skills

Discipline skills must survive agents negotiating under pressure. State the rule, then name the specific workarounds it forecloses (keeping deleted code as reference, adapting it while writing tests, "I already manually tested it"). Cut off spirit-versus-letter arguments with a foundational principle: violating the letter of the rules is violating the spirit of the rules. Build a rationalization table from the verbatim excuses baseline runs produce, and a red-flags list agents can self-check against. This toolkit is for discipline failures only; applied to shaping problems it backfires, so use the forms above instead.

## Test Before Shipping

Every skill is tested with subagents before deployment, and each excuse for skipping ("obviously clear", "just a reference", "no time", "I'm confident") fails the same way: untested skills hide issues you cannot see until an agent uses them. Complete one skill's full cycle before starting another; batching is the rationalization, not the efficiency.

RED: run pressure scenarios with a subagent that lacks the skill and document exact behavior, including rationalizations verbatim. You must see the natural failure before writing the fix. GREEN: write the minimal skill addressing those specific failures, nothing for hypotheticals, and re-run the same scenarios until agents comply. REFACTOR: each new rationalization gets an explicit counter; re-test until none surface.

Micro-test wording before full scenario runs: one fresh-context sample per call with the guidance in its realistic surrounding context, always against a no-guidance control (if the control does not exhibit the failure, there is nothing to fix; stop), 5+ reps per variant, and every flagged match read manually, since template echoes and quoted counter-examples masquerade as hits. Variance is a metric: reps converging on one shape mean the wording binds; five interpretations mean tighten the form before adding words. Micro-tests verify wording; discipline skills still need full pressure scenarios as the final gate.

Match the test to the skill type: discipline skills under combined pressures (time, sunk cost, exhaustion), techniques by application to new scenarios, patterns by recognition and counter-examples, references by retrieval and gap probes.

Full methodology, pressure types, hole-plugging, and meta-testing: see [testing-skills-with-subagents.md](testing-skills-with-subagents.md).

## Shipping

The finished skill lives in a discoverable skills directory, ready to use. Commit it only when the human directs; committing or pushing is never a side effect of authoring a skill. Consider contributing broadly useful skills back via PR.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
