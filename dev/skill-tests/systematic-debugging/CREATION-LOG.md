# Creation Log: Systematic Debugging Skill

Reference example of extracting, structuring, and hardening a critical skill.

## Source Material

Extracted the debugging framework from `~/.claude/CLAUDE.md`:
- 4-phase systematic process (Investigation → Pattern Analysis → Hypothesis → Implementation)
- Core mandate: find the root cause, don't fix symptoms
- Rules designed to resist time pressure and rationalization

## Extraction Decisions

What to include:
- Complete 4-phase framework with all rules
- Anti-shortcuts ("find the root cause", "stop and re-analyze")
- Pressure-resistant guidance ("even if faster", "even if I seem in a hurry")
- Concrete steps for each phase

What to leave out:
- Project-specific context
- Repetitive variations of the same rule
- Narrative explanations (condensed to principles)

## Structure Following skill-creation/SKILL.md

1. Rich when-to-use — included symptoms and anti-patterns
2. Type: technique — concrete process with steps
3. Keywords — "root cause", "symptom", "workaround", "debugging", "investigation"
4. Decision point for "fix failed" → re-analyze vs add more fixes
5. Phase-by-phase breakdown — scannable checklist format
6. Anti-patterns section — what not to do (critical for this skill)

## Hardening Elements

The framework is designed to resist rationalization under pressure:

### Language Choices
- Clear directives rather than "should" / "try to"
- "even if faster" / "even if I seem in a hurry"
- "stop and re-analyze" (explicit pause)
- "don't skip past" (catches the actual behavior)

### Structural Defenses
- Phase 1 required — can't skip to implementation
- Single hypothesis rule — forces thinking, prevents shotgun fixes
- Explicit failure mode — "if your first fix doesn't work" with a mandatory action
- Anti-patterns section — shows exactly what shortcuts look like

### Redundancy
- Root cause mandate in overview + when-to-use + Phase 1 + implementation rules
- "Don't fix the symptom" appears in several different contexts
- Each phase has explicit "don't skip" guidance

## Testing Approach

Created 4 validation tests following skills/meta/testing-skills-with-subagents:

### Test 1: Academic Context (No Pressure)
- Simple bug, no time pressure
- Result: full compliance, complete investigation

### Test 2: Time Pressure + Obvious Quick Fix
- User "in a hurry", symptom fix looks easy
- Result: resisted the shortcut, followed the full process, found the real root cause

### Test 3: Complex System + Uncertainty
- Multi-layer failure, unclear whether a root cause could be found
- Result: systematic investigation, traced through all layers, found the source

### Test 4: Failed First Fix
- Hypothesis doesn't work, temptation to add more fixes
- Result: stopped, re-analyzed, formed a new hypothesis (no shotgun)

All tests passed. No rationalizations found.

## Iterations

### Initial Version
- Complete 4-phase framework
- Anti-patterns section
- Decision guidance for "fix failed"

### Enhancement 1: TDD Reference
- Added a link to skills/testing/test-driven-development
- Note explaining that TDD's "simplest code" is not the same as debugging's "root cause"
- Prevents confusion between the two methodologies

## Final Outcome

A durable skill that:
- Clearly mandates root cause investigation
- Resists time pressure rationalization
- Provides concrete steps for each phase
- Shows anti-patterns explicitly
- Was tested under multiple pressure scenarios
- Clarifies the relationship to TDD
- Ready for use

## Key Insight

The most important hardening element is the anti-patterns section showing the exact shortcuts that feel justified in the moment. When you think "I'll just add this one quick fix", seeing that exact pattern listed as wrong creates useful friction.

## Usage Example

When encountering a bug:
1. Load the skill: skills/debugging/systematic-debugging
2. Read the overview (10 sec) — reminded of the mandate
3. Follow the Phase 1 checklist — forced investigation
4. If tempted to skip — see the anti-pattern, stop
5. Complete all phases — root cause found

Time investment: 5-10 minutes
Time saved: hours of symptom whack-a-mole

---

*Created: 2025-10-03*
*Purpose: Reference example for skill extraction and hardening*
