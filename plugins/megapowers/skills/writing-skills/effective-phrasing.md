# Effective Phrasing for Skill Design

## Overview

A skill only works if the agent reading it actually does the thing. This document is about writing skills that land: descriptions that trigger accurately, instructions an agent can follow under pressure, and emphasis used only where it earns its place. The goal is clarity and trust, not coercion. An agent that understands why a step matters follows it more reliably than one that's merely told to.

Research foundation: work on how language models respond to framing (Cialdini, 2021; Meincke et al., 2025) shows that clear, well-structured instructions with stated reasons and concrete triggers are followed far more consistently than vague ones. Use that finding to write clearer skills, not to manufacture false urgency.

## What Makes a Skill Land

### 1. Trigger-Accurate Descriptions

The description decides whether the skill loads at all. Write it so an agent facing the right situation recognizes the match immediately.

- Start with "Use when..." and name concrete triggers, symptoms, and contexts.
- Describe the problem (race conditions, flaky tests), not just the technique.
- Third person, since it's injected into the system prompt.
- Don't summarize the workflow — that tempts agents to follow the summary instead of reading the skill.

```markdown
Prefer: Use when tests have race conditions, timing dependencies, or pass/fail inconsistently.
Avoid:  Helps with async testing.
```

### 2. State the Reason, Not Just the Rule

An agent that knows why a step exists applies it correctly to cases the skill didn't anticipate. A bare command doesn't generalize.

```markdown
Prefer: Write the test first — tests-after answer "what does this do?" instead of
        "what should this do?", so they rubber-stamp whatever you already built.
Avoid:  Always write tests first.
```

### 3. Clear Triggers Paired With Actions

"When X, do Y" is more reliable than "generally do Y". A concrete trigger plus a concrete action leaves little to interpret.

```markdown
Prefer: After completing a task, request code review before starting the next one.
Avoid:  Review code when convenient.
```

### 4. Minimal Necessary Emphasis

Emphasis is a scarce resource. If everything is bold and urgent, nothing reads as important. Reserve strong wording for the few rules that genuinely break the work when skipped, and let the rest stand as plain declarative prose.

- Use a firm, plain statement for real requirements: "Delete the change and start over."
- Skip the ALL-CAPS, the threats, and the exclamation points — they don't add force, they add noise.
- One clearly-marked requirement carries more weight than ten competing ones.

### 5. Commitment Through Structure

Asking an agent to make an explicit choice or track progress produces more consistent follow-through than a passing mention. Use it where it fits the workflow.

- Force a real decision: "Choose A, B, or C" rather than leaving it open.
- Track multi-step work with a checklist the agent copies and checks off.
- Announce which skill is in use, when that helps accountability.

### 6. Collaborative Framing

Skills that involve judgment (code review, honest feedback) work better with "we're working on this together" framing than with top-down commands. Shared goals invite the honest technical judgment those skills depend on.

```markdown
Prefer: We both want this to be correct — tell me where the reasoning breaks down.
Avoid:  You should probably mention if something is wrong.
```

## Matching Framing to Skill Type

| Skill Type | Lean on | Go light on |
|------------|---------|-------------|
| Discipline-enforcing (TDD, verification) | Clear reasons, firm requirements, stated consequences | Excess emphasis, threats |
| Guidance / technique | Reasons + collaborative framing | Heavy imperatives |
| Collaborative (review, feedback) | Shared goals, explicit choices | Top-down commands |
| Reference (API, syntax) | Clarity and structure only | Any persuasion |

## Why This Works

Clear rules reduce second-guessing.
- A firm, well-reasoned requirement removes the "is this an exception?" loop.
- Naming the specific loophole ("don't keep it as reference") closes it more reliably than a general "don't cheat".

Implementation intentions create dependable behavior.
- A concrete trigger paired with a concrete action executes more consistently than a general preference.
- "When X, do Y" beats "generally do Y".

Language models respond to the patterns in their training text.
- Instructions that state a reason before an action are common in well-written documentation, and models follow them well.
- Commitment sequences (state the choice, then act) and clearly-marked requirements are patterns the model has seen work.

## Staying Honest

Legitimate uses:
- Ensuring genuinely critical practices are followed
- Writing documentation an agent can act on the first time
- Preventing predictable, costly failures

Avoid:
- Manufacturing false urgency where none exists
- Guilt or threat framing to force compliance
- Emphasis that overstates how much a step actually matters

The test: would this framing still serve the reader's genuine interests if they saw exactly why it was written that way? If yes, it's clarity. If no, cut it.

## Research Citations

Cialdini, R. B. (2021). *Influence: The Psychology of Persuasion (New and Expanded).* Harper Business.

Meincke, L., Shapiro, D., Duckworth, A. L., Mollick, E., Mollick, L., & Cialdini, R. (2025). Call Me A Jerk: Persuading AI to Comply with Objectionable Requests. University of Pennsylvania. Tested how framing shifts language-model compliance across a large sample; read here as evidence that clear structure and stated reasons drive reliable behavior.

## Quick Reference

When designing a skill, ask:

1. What type is it? (Discipline vs. guidance vs. reference)
2. What behavior am I trying to produce?
3. Have I stated the reason, not just the rule?
4. Is my emphasis reserved for what actually matters?
5. Would this framing hold up if the reader saw exactly why I wrote it?
