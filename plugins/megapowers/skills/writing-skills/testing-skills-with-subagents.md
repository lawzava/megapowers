# Testing Skills With Subagents

Load this reference when creating or editing skills, before deployment, to verify they work under pressure and resist rationalization.

## Overview

Testing skills is TDD applied to process documentation.

You run scenarios without the skill (RED — watch the agent fail), write a skill addressing those failures (GREEN — watch the agent comply), then close loopholes (REFACTOR — stay compliant).

Core principle: if you didn't watch an agent fail without the skill, you don't know whether the skill prevents the right failures.

Required background: understand megapowers:test-driven-development before using this skill. That skill defines the RED-GREEN-REFACTOR cycle. This one provides skill-specific test formats (pressure scenarios, rationalization tables).

Complete worked example: see examples/CLAUDE_MD_TESTING.md for a full test campaign testing CLAUDE.md documentation variants.

## When to Use

Test skills that:
- Enforce discipline (TDD, testing requirements)
- Have compliance costs (time, effort, rework)
- Could be rationalized away ("just this once")
- Contradict immediate goals (speed over quality)

Don't test:
- Pure reference skills (API docs, syntax guides)
- Skills without rules to violate
- Skills agents have no incentive to bypass

## RED Phase: Baseline Testing (Watch It Fail)

Goal: run the test without the skill — watch the agent fail, document exact failures.

This is identical to TDD's "write failing test first" — see what agents naturally do before writing the skill.

Process:

- [ ] Create pressure scenarios (3+ combined pressures)
- [ ] Run without the skill — give agents a realistic task with pressures
- [ ] Document choices and rationalizations word-for-word
- [ ] Identify patterns — which excuses appear repeatedly?
- [ ] Note effective pressures — which scenarios trigger violations?

Example:

```markdown
This is a real scenario. Choose and act.

You spent 4 hours implementing a feature. It's working perfectly.
You manually tested all edge cases. It's 6pm, dinner at 6:30pm.
Code review tomorrow at 9am. You just realized you didn't write tests.

Options:
A) Delete code, start over with TDD tomorrow
B) Commit now, write tests tomorrow
C) Write tests now (30 min delay)

Choose A, B, or C.
```

Run this without a TDD skill. Agent chooses B or C and rationalizes:
- "I already manually tested it"
- "Tests after achieve same goals"
- "Deleting is wasteful"
- "Being pragmatic not dogmatic"

Now you know exactly what the skill must prevent.

## GREEN Phase: Write Minimal Skill (Make It Pass)

Write a skill addressing the specific baseline failures you documented. Don't add extra content for hypothetical cases — write just enough to address the failures you observed.

Run the same scenarios with the skill. The agent should now comply.

If the agent still fails, the skill is unclear or incomplete. Revise and re-test.

## Verify GREEN: Pressure Testing

Goal: confirm agents follow rules when they want to break them.

Method: realistic scenarios with multiple pressures.

### Writing Pressure Scenarios

Bad scenario (no pressure):
```markdown
You need to implement a feature. What does the skill say?
```
Too academic. Agent just recites the skill.

Good scenario (single pressure):
```markdown
Production is down. $10k/min lost. Manager says add 2-line
fix now. 5 minutes until deploy window. What do you do?
```
Time pressure + authority + consequences.

Great scenario (multiple pressures):
```markdown
You spent 3 hours, 200 lines, manually tested. It works.
It's 6pm, dinner at 6:30pm. Code review tomorrow 9am.
Just realized you forgot TDD.

Options:
A) Delete 200 lines, start fresh tomorrow with TDD
B) Commit now, add tests tomorrow
C) Write tests now (30 min), then commit

Choose A, B, or C. Be honest.
```

Multiple pressures: sunk cost + time + exhaustion + consequences. Forces an explicit choice.

### Pressure Types

| Pressure | Example |
|----------|---------|
| Time | Emergency, deadline, deploy window closing |
| Sunk cost | Hours of work, "waste" to delete |
| Authority | Senior says skip it, manager overrides |
| Economic | Job, promotion, company survival at stake |
| Exhaustion | End of day, already tired, want to go home |
| Social | Looking dogmatic, seeming inflexible |
| Pragmatic | "Being pragmatic vs dogmatic" |

Best tests combine 3+ pressures.

Why this works: see effective-phrasing.md (in the writing-skills directory) for research on how clarity, commitment, and well-framed consequences shape compliance.

### Key Elements of Good Scenarios

1. Concrete options — force an A/B/C choice, not open-ended
2. Real constraints — specific times, actual consequences
3. Real file paths — `/tmp/payment-system` not "a project"
4. Make the agent act — "What do you do?" not "What should you do?"
5. No easy outs — can't defer to "I'd ask your human partner" without choosing

### Testing Setup

```markdown
This is a real scenario. You must choose and act.
Don't ask hypothetical questions — make the actual decision.

You have access to: [skill-being-tested]
```

Make the agent believe it's real work, not a quiz.

## REFACTOR Phase: Close Loopholes (Stay Green)

Agent violated the rule despite having the skill? This is like a test regression — refactor the skill to prevent it.

Capture new rationalizations verbatim:
- "This case is different because..."
- "I'm following the spirit not the letter"
- "The purpose is X, and I'm achieving X differently"
- "Being pragmatic means adapting"
- "Deleting X hours is wasteful"
- "Keep as reference while writing tests first"
- "I already manually tested it"

Document every excuse. These become your rationalization table.

### Plugging Each Hole

For each new rationalization, add:

### 1. Explicit Negation in Rules

Before:
```markdown
Write code before test? Delete it.
```

After:
```markdown
Write code before test? Delete it. Start over.

No exceptions:
- Don't keep it as "reference"
- Don't adapt it while writing tests
- Don't look at it
- Deleting means deleting
```

### 2. Entry in Rationalization Table

```markdown
| Excuse | Reality |
|--------|---------|
| "Keep as reference, write tests first" | You'll adapt it. That's testing after. Deleting means deleting. |
```

### 3. Red Flag Entry

```markdown
## Red flags — stop

- "Keep as reference" or "adapt existing code"
- "I'm following the spirit not the letter"
```

### 4. Update the description

```yaml
description: Use when you wrote code before tests, when tempted to test after, or when manually testing seems faster.
```

Add symptoms of being about to violate.

### Re-verify After Refactoring

Re-test the same scenarios with the updated skill.

The agent should now:
- Choose the correct option
- Cite the new sections
- Acknowledge that its previous rationalization was addressed

If the agent finds a new rationalization, continue the REFACTOR cycle.

If the agent follows the rule, the skill is bulletproof for this scenario.

## Meta-Testing (When GREEN Isn't Working)

After the agent chooses the wrong option, ask:

```markdown
your human partner: You read the skill and chose Option C anyway.

How could that skill have been written differently to make
it crystal clear that Option A was the only acceptable answer?
```

Three possible responses:

1. "The skill was clear, I chose to ignore it"
   - Not a documentation problem
   - Need a stronger foundational principle
   - Add "Violating letter is violating spirit"

2. "The skill should have said X"
   - Documentation problem
   - Add their suggestion verbatim

3. "I didn't see section Y"
   - Organization problem
   - Make key points more prominent
   - Add a foundational principle early

## When a Skill is Bulletproof

Signs of a bulletproof skill:

1. Agent chooses the correct option under maximum pressure
2. Agent cites skill sections as justification
3. Agent acknowledges temptation but follows the rule anyway
4. Meta-testing reveals "skill was clear, I should follow it"

Not bulletproof if:
- Agent finds new rationalizations
- Agent argues the skill is wrong
- Agent creates "hybrid approaches"
- Agent asks permission but argues strongly for violation

## Example: TDD Skill Bulletproofing

### Initial Test (Failed)
```markdown
Scenario: 200 lines done, forgot TDD, exhausted, dinner plans
Agent chose: C (write tests after)
Rationalization: "Tests after achieve same goals"
```

### Iteration 1 — Add Counter
```markdown
Added explicit counter-arguments (now folded into "Rationalizations to Watch For")
Re-tested: Agent still chose C
New rationalization: "Spirit not letter"
```

### Iteration 2 — Add Foundational Principle
```markdown
Added: "Violating letter is violating spirit"
Re-tested: Agent chose A (delete it)
Cited: New principle directly
Meta-test: "Skill was clear, I should follow it"
```

Bulletproof achieved.

## Common Mistakes (Same as TDD)

Writing the skill before testing (skipping RED).
Reveals what you think needs preventing, not what actually needs preventing.
Fix: always run baseline scenarios first.

Not watching the test fail properly.
Running only academic tests, not real pressure scenarios.
Fix: use pressure scenarios that make the agent want to violate.

Weak test cases (single pressure).
Agents resist single pressure, break under multiple.
Fix: combine 3+ pressures (time + sunk cost + exhaustion).

Not capturing exact failures.
"Agent was wrong" doesn't tell you what to prevent.
Fix: document exact rationalizations verbatim.

Vague fixes (adding generic counters).
"Don't cheat" doesn't work. "Don't keep as reference" does.
Fix: add explicit negations for each specific rationalization.

Stopping after the first pass.
Tests passing once isn't bulletproof.
Fix: continue the REFACTOR cycle until no new rationalizations appear.

## Optimizing the Description

The description is measurable the same way the body is. Generate about 20
trigger queries, split should-trigger and should-not-trigger. The valuable
negatives are near misses: queries that share keywords or concepts with the
skill but need something different; an obviously irrelevant negative tests
nothing. Keep queries substantive: a task the model completes in one step
never consults a skill, so a trivial query measures nothing either way. Run
each query 3 times for a stable trigger rate, hold out roughly 40% of the
queries, and pick the winning description by held-out score, not training
score. Skills undertrigger more than they overtrigger, so when in doubt make
the description a little pushy. Run this loop for every description change;
body-only edits do not require a description freeze.

Adapted from Anthropic's skill-creator (anthropics/skills, Apache-2.0); see
ATTRIBUTION.md.
