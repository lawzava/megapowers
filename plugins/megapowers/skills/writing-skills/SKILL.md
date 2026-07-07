---
name: writing-skills
description: Use when creating new skills, editing existing skills, or verifying skills work before deployment
license: MIT
---

# Writing Skills

## Overview

Writing skills is Test-Driven Development applied to process documentation.

Personal skills live in your runtime's skills directory.

You write test cases (pressure scenarios with subagents), watch them fail (baseline behavior), write the skill (documentation), watch tests pass (agents comply), and refactor (close loopholes).

**Core principle:** if you didn't watch an agent fail without the skill, you don't know whether the skill teaches the right thing.

Required background: understand megapowers:test-driven-development before using this skill. That skill defines the RED-GREEN-REFACTOR cycle. This skill adapts TDD to documentation.

Official guidance: for Anthropic's skill authoring best practices, see anthropic-best-practices.md. It provides additional patterns that complement the TDD-focused approach here.

## What Is a Skill?

A skill is a reference guide for proven techniques, patterns, or tools. Skills help future agents find and apply effective approaches.

Skills are: reusable techniques, patterns, tools, reference guides.

Skills are not: narratives about how you solved a problem once.

The mapping is direct: pressure scenario = test case, baseline violation =
RED, minimal skill that fixes it = GREEN, closing loopholes = REFACTOR (see
RED-GREEN-REFACTOR for Skills below).

## When to Create a Skill

Create when:
- The technique wasn't intuitively obvious to you
- You'd reference this again across projects
- The pattern applies broadly (not project-specific)
- Others would benefit

Don't create for:
- One-off solutions
- Standard practices well-documented elsewhere
- Project-specific conventions (put in your instructions file)
- Mechanical constraints (if it's enforceable with regex/validation, automate it — save documentation for judgment calls)

## Skill Types

### Technique
Concrete method with steps to follow (condition-based-waiting, root-cause-tracing).

### Pattern
Way of thinking about problems (flatten-with-flags, test-invariants).

### Reference
API docs, syntax guides, tool documentation (office docs).

## Directory Structure

```
skills/
  skill-name/
    SKILL.md              # Main reference (required)
    supporting-file.*     # Only if needed
```

Flat namespace — all skills in one searchable namespace.

Separate files for:
1. Heavy reference (100+ lines) — API docs, comprehensive syntax
2. Reusable tools — scripts, utilities, templates

Keep inline:
- Principles and concepts
- Code patterns (< 50 lines)
- Everything else

## SKILL.md Structure

Frontmatter (YAML):
- Two required fields: `name` and `description` (see [agentskills.io/specification](https://agentskills.io/specification) for all supported fields)
- Limits are per field: `name` max 64 characters, `description` max 1024
- `name`: letters, numbers, and hyphens only (no parentheses, special chars)
- `description`: third-person, describes only when to use (not what it does)
  - Start with "Use when..." to focus on triggering conditions
  - Include specific symptoms, situations, and contexts
  - Do not summarize the skill's process or workflow (see the SDO section for why)
  - Keep under 500 characters if possible

```markdown
---
name: skill-name-with-hyphens
description: Use when [specific triggering conditions and symptoms]
---

# Skill Name

## Overview
What is this? Core principle in 1-2 sentences.

## When to Use
[Small inline flowchart if the decision is non-obvious]

Bullet list with symptoms and use cases
When NOT to use

## Core Pattern (for techniques/patterns)
Before/after code comparison

## Quick Reference
Table or bullets for scanning common operations

## Implementation
Inline code for simple patterns
Link to file for heavy reference or reusable tools

## Common Mistakes
What goes wrong + fixes
```

Do not add a "Real-World Impact" section with unsourced statistics; a claim of
effect needs a measured run behind it (see the eval methodology in this repo's
`evals/`), and invented percentages get repeated to users as fact.

## Skill Discovery Optimization (SDO)

Future agents need to find your skill.

### 1. Rich Description Field

Purpose: an agent reads the description to decide which skills to load for a given task. Make it answer: "Should I read this skill right now?"

Format: start with "Use when..." to focus on triggering conditions.

Description = when to use, not what the skill does. The description should only describe triggering conditions. Do not summarize the skill's process or workflow in the description.

Why this matters: in testing, a description that summarized the workflow
("code review between tasks") made agents follow the description and skip the
body — one review instead of the skill's mandated two. Rewriting it to
triggering conditions only restored full compliance. A workflow summary is a
shortcut agents will take.

Content rules:
- Use concrete triggers, symptoms, and situations that signal this skill applies
- Describe the *problem* (race conditions, inconsistent behavior) not *language-specific symptoms* (setTimeout, sleep)
- Keep triggers technology-agnostic unless the skill itself is technology-specific; if it is, name the technology in the trigger
- Write in third person (injected into the system prompt)
- Do not summarize the skill's process or workflow

```yaml
# Avoid: summarizes workflow — agents follow this instead of reading the skill
description: Use when executing plans - dispatches subagent per task with code review between tasks

# Avoid: vague, no trigger; or first person
description: For async testing

# Prefer: triggering conditions only, starts with "Use when", names the problem
description: Use when tests have race conditions, timing dependencies, or pass/fail inconsistently
```

### 2. Keyword Coverage

Use words an agent would search for:
- Error messages: "Hook timed out", "ENOTEMPTY", "race condition"
- Symptoms: "flaky", "hanging", "zombie", "pollution"
- Synonyms: "timeout/hang/freeze", "cleanup/teardown/afterEach"
- Tools: actual commands, library names, file types

### 3. Descriptive Naming

Use active voice, verb-first. Name by what you do or the core insight:
- `creating-skills` not `skill-creation`
- `condition-based-waiting` over `async-test-helpers`
- `using-skills` not `skill-usage`
- `flatten-with-flags` over `data-structure-refactoring`
- `root-cause-tracing` over `debugging-techniques`

Gerunds (-ing) work well for processes:
- `creating-skills`, `testing-skills`, `debugging-with-logs`
- Active, describes the action you're taking

### 4. Token Efficiency (Critical)

Problem: getting-started and frequently-referenced skills load into every conversation. Every token counts.

Target budgets (tightest where it loads most often):
- getting-started workflows: <150 words each
- Frequently-loaded skills (always in context): <200 words total
- Other skills: keep the SKILL.md body under ~500 lines (the Agent Skills
  convention) and as concise as the material allows — a rich process skill will
  run longer than 500 words, and that's fine; push heavy reference and examples
  into `references/` files loaded on demand rather than padding the body.

Techniques:
- Move flag/option details to the tool's `--help`; reference it instead of
  documenting every flag.
- Cross-reference other skills by name instead of repeating their workflow.
- Compress examples to the minimum that shows the pattern.
- Don't repeat what's in cross-referenced skills, explain what's obvious from
  the command, or include multiple examples of the same pattern.
- Verify with `wc -w` against the budgets above.

### 5. Cross-Referencing Other Skills

When writing documentation that references other skills, use the skill name only, with explicit requirement markers:
- Good: `**Required sub-skill:** use megapowers:test-driven-development`
- Good: `**Required background:** understand megapowers:systematic-debugging`
- Avoid: `See skills/testing/test-driven-development` (unclear if required)
- Avoid: `@skills/testing/test-driven-development/SKILL.md` (force-loads, burns context)

Why no @ links: `@` syntax force-loads files immediately, consuming 200k+ context before you need them.

## Code Examples

One excellent example beats many mediocre ones.

Choose the most relevant language:
- Testing techniques → TypeScript/JavaScript
- System debugging → Shell/Python
- Data processing → Python

Good example:
- Complete and runnable
- Well-commented explaining why
- From a real scenario
- Shows the pattern clearly
- Ready to adapt (not a generic template)

Don't:
- Implement in 5+ languages
- Create fill-in-the-blank templates
- Write contrived examples

You're good at porting — one great example is enough.

## File Organization

Three shapes, by need: SKILL.md alone (all content fits inline); SKILL.md plus
a working example file (when the tool is reusable code, not narrative);
SKILL.md plus reference docs and scripts (when reference material is too large
for inline).

## The Core Rule (same discipline as TDD)

No **behavioral guidance** without a failing test first.

A change needs test-first discipline when it adds or alters what the skill instructs
an agent to *do* — a rule, prohibition, recipe, or conditional meant to shape behavior
under pressure. For those: if you wrote the guidance before testing, delete it and
start over; editing behavioral guidance without testing is the same violation. Don't
keep untested changes as reference, and don't adapt them while running the tests —
delete and restart the cycle. This holds even for "simple additions" and "just adding
a section" when that section changes behavior.

**Proportionality — mechanical and editorial edits don't need a pressure test.** Fixing
a typo, a broken link, a stale path, or formatting, and rewording that preserves the
instruction's meaning, carry no behavioral hypothesis. By this skill's own micro-test
rule, a change whose no-guidance control wouldn't fail differently has nothing to test
(see *Micro-Test Wording Before Full Scenarios*). Still verify these are *correct* — the
link resolves, the example runs, the value is right — just don't stage a delete-and-restart
campaign for a change that alters no behavior. When unsure whether an edit is behavioral,
treat it as behavioral.

## Testing All Skill Types

Different skill types need different test approaches:

| Type | Test with | Success criterion |
|---|---|---|
| Discipline (rules) | Pressure scenarios, multiple pressures combined (time + sunk cost + exhaustion); capture rationalizations | Follows the rule under maximum pressure |
| Technique (how-to) | Application + variation scenarios; missing-information probes | Applies the technique to a new scenario |
| Pattern (mental model) | Recognition, application, counter-examples | Knows when and when not to apply it |
| Reference (docs/APIs) | Retrieval + application scenarios; gap testing | Finds and correctly applies the information |

## Rationalizations to Watch For When Skipping Testing

Test before deploying, in every case. The excuses below are common and each one is wrong for the same reason: untested skills have issues you can't see until an agent uses them.

- "Skill is obviously clear" — clear to you isn't clear to other agents.
- "It's just a reference" — references can have gaps and unclear sections; test retrieval.
- "Testing is overkill" — 15 minutes of testing saves hours of debugging a bad skill in production.
- "I'll test if problems emerge" — the problem is agents can't use the skill; test before deploying.
- "Too tedious to test" — testing is less tedious than debugging a bad skill in production.
- "I'm confident it's good" — overconfidence is where issues hide; test anyway.
- "Academic review is enough" — reading isn't using; test application scenarios.
- "No time to test" — deploying an untested skill costs more time later.

## Match the Form to the Failure

Before writing guidance, classify the baseline failure. The form that bulletproofs one failure type measurably backfires on another.

| Baseline failure | Right form | Wrong form |
|---|---|---|
| Skips/violates a rule under pressure (knows better, does it anyway) | Prohibition + rationalization table + red flags (see Bulletproofing below) | Soft guidance ("prefer...", "consider...") |
| Complies, but output has the wrong shape (bloated prompt, buried verdict, restated spec) | Positive recipe or contract: state what the output IS — its parts, in order | Prohibition list ("don't restate", "never narrate") |
| Omits a required element from something they already produce | Structural: required field or slot in the template they fill in | Prose reminders near the template |
| Behavior should depend on a condition | Conditional keyed to an observable predicate ("if the brief exists, reference it") | Unconditional rule + exemption clauses |

Why prohibitions backfire on shaping problems: under a competing incentive ("make the prompt self-contained"), agents negotiate with "don't X". In head-to-head wording tests on dispatch-prompt guidance, the prohibition arm produced clearly more of the unwanted content than the recipe arm (fully separated distributions), and trended worse than even the no-guidance control — micro-test your own case rather than assuming, but don't reach for the prohibition by default. A recipe leaves nothing to negotiate: the output matches the stated shape or it doesn't.

Rules for whichever form you pick:
- No nuance clauses. "Don't X unless it matters" reopens the negotiation — appending a single nuance clause to a winning recipe degraded it from consistent to noisy in the same wording tests. Express a real exception as its own conditional on an observable predicate.
- Exemption clauses don't scope. "This limit doesn't apply to code blocks" still suppresses code blocks. If part of the output must be exempt, restructure so the rule can't reach it.

## Bulletproofing Skills Against Rationalization

Skills that enforce discipline (like TDD) need to resist rationalization. Agents are smart and will find loopholes under pressure.

Scope: this toolkit is for discipline failures — an agent that knows the rule and skips it under pressure. For wrong-shaped output or omitted elements, prohibition-based bulletproofing backfires; use the forms in Match the Form to the Failure instead.

Note on effective phrasing: understanding what makes guidance land helps you apply it systematically. See effective-phrasing.md for the research foundation (Cialdini, 2021; Meincke et al., 2025) on clear, trustworthy skill design.

### Close Every Loophole Explicitly

Don't just state the rule — name the specific workarounds it forecloses.

Weak:
```markdown
Write code before test? Delete it.
```

Stronger:
```markdown
Write code before test? Delete it. Start over.

No exceptions:
- Don't keep it as "reference"
- Don't adapt it while writing tests
- Don't look at it
- Deleting means deleting
```

### Address "Spirit vs Letter" Arguments

Add a foundational principle early:

```markdown
Violating the letter of the rules is violating the spirit of the rules.
```

This cuts off an entire class of "I'm following the spirit" rationalizations.

### Build a Rationalization Table

Capture rationalizations from baseline testing (see the Testing section). Every excuse agents make goes in the table:

```markdown
| Excuse | Reality |
|--------|---------|
| "Too simple to test" | Simple code breaks. Test takes 30 seconds. |
| "I'll test after" | Tests passing immediately prove nothing. |
| "Tests after achieve same goals" | Tests-after = "what does this do?" Tests-first = "what should this do?" |
```

### Create a Self-Check List

Make it easy for agents to self-check when rationalizing:

```markdown
## Red flags — stop and start over

- Code before test
- "I already manually tested it"
- "Tests after achieve the same purpose"
- "It's about spirit not ritual"
- "This is different because..."

All of these mean: delete the code, start over with TDD.
```

## RED-GREEN-REFACTOR for Skills

### RED: Write Failing Test (Baseline)

Run a pressure scenario with a subagent without the skill. Document exact behavior:
- What choices did they make?
- What rationalizations did they use (verbatim)?
- Which pressures triggered violations?

This is "watch the test fail" — you must see what agents naturally do before writing the skill.

### GREEN: Write Minimal Skill

Write a skill that addresses those specific rationalizations. Don't add extra content for hypothetical cases.

Run the same scenarios with the skill. The agent should now comply.

### REFACTOR: Close Loopholes

Agent found a new rationalization? Add an explicit counter. Re-test until bulletproof.

### Micro-Test Wording Before Full Scenarios

Full pressure-scenario runs are the final gate, but they are slow and expensive per iteration. Verify the wording itself first with micro-tests:

1. One fresh-context sample per call — a raw API call, or a single-shot subagent if you don't have API access. System prompt = the realistic context the guidance will live in (the full skill or prompt template, not the guidance in isolation); user message = a task that tempts the failure.
2. Always include a no-guidance control. If the control doesn't exhibit the failure, there is nothing to fix — stop, don't author the guidance.
3. 5+ reps per variant. Single samples lie.
4. Manually read every flagged match. Score programmatically if you like, but template echoes and quoted counter-examples masquerade as hits; automated counts alone overstate both failure and success.
5. Variance is a metric. When guidance lands, reps converge on the same shape. Five different interpretations across five reps means the wording isn't binding — tighten the form before adding words.

Micro-tests verify wording; they do not replace pressure scenarios for discipline skills.

Testing methodology: see [testing-skills-with-subagents.md](testing-skills-with-subagents.md) for the complete testing methodology:
- How to write pressure scenarios
- Pressure types (time, sunk cost, authority, exhaustion)
- Plugging holes systematically
- Meta-testing techniques

## Anti-Patterns

### Narrative Example
"In session 2025-10-03, we found empty projectDir caused..."
Why it's bad: too specific, not reusable.

### Multi-Language Dilution
example-js.js, example-py.py, example-go.go
Why it's bad: mediocre quality, maintenance burden.

### Code in Flowcharts
A flowchart with nodes labeled "import fs" then "read file".
Why it's bad: can't copy-paste, hard to read. Put code in a markdown block instead.

### Generic Labels
helper1, helper2, step3, pattern4
Why it's bad: labels should have semantic meaning.

## Before Moving to the Next Skill

Complete the checklist below for each skill before starting another. No
batching: "batching is more efficient" is the rationalization; deploying an
untested skill is deploying untested code.

## Skill Creation Checklist (TDD Adapted)

Create a todo for each checklist item below.

RED phase — write failing test:
- [ ] Create pressure scenarios (3+ combined pressures for discipline skills)
- [ ] Run scenarios without the skill — document baseline behavior verbatim
- [ ] Identify patterns in rationalizations/failures

GREEN phase — write minimal skill:
- [ ] Name uses only letters, numbers, hyphens (no parentheses/special chars)
- [ ] YAML frontmatter with required `name` and `description` fields (`name` ≤ 64 chars, `description` ≤ 1024; see [spec](https://agentskills.io/specification))
- [ ] Description starts with "Use when..." and includes specific triggers/symptoms
- [ ] Description written in third person
- [ ] Keywords throughout for search (errors, symptoms, tools)
- [ ] Clear overview with core principle
- [ ] Address specific baseline failures identified in RED
- [ ] Guidance form matches the failure type (see Match the Form to the Failure)
- [ ] For behavior-shaping guidance: wording micro-tested against a no-guidance control (5+ reps, every flagged match read manually) — N/A for pure reference skills
- [ ] Code inline OR link to a separate file
- [ ] One excellent example (not multi-language)
- [ ] Run scenarios with the skill — verify agents now comply

REFACTOR phase — close loopholes:
- [ ] Identify new rationalizations from testing
- [ ] Add explicit counters (if a discipline skill)
- [ ] Build rationalization table from all test iterations
- [ ] Create red flags list
- [ ] Re-test until bulletproof

Quality checks:
- [ ] Small flowchart only if the decision is non-obvious
- [ ] Quick reference table
- [ ] Common mistakes section
- [ ] No narrative storytelling
- [ ] Supporting files only for tools or heavy reference

Deployment:
- [ ] Skill lives in a discoverable skills directory and is ready to use
- [ ] When the human directs a commit, the skill goes in (don't commit or push as a side effect of authoring it)
- [ ] Consider contributing back via PR (if broadly useful)

## Discovery Workflow

An agent finds your skill by matching a problem against descriptions, scans
the overview for relevance, then reads patterns and loads examples only when
implementing. Optimize for that flow: searchable terms early and often.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
