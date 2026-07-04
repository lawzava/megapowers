---
name: systematic-debugging
description: Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes — including when asked to fix something ("the test suite is failing", "find the cause", "fix this bug", "why is this failing"), and for flaky or intermittent tests. After finding the root cause, hand off to test-driven-development to write the fix.
license: MIT
---

# Systematic Debugging

## Overview

Random fixes waste time and create new bugs. Quick patches mask the underlying issue and it resurfaces later.

**Core principle:** find the root cause before attempting a fix. A fix that only addresses the symptom is not a fix.

## The Core Rule

Find the root cause before you change any code. If you haven't completed Phase 1, you're not ready to propose fixes.

## When to Use

Use for any technical issue:
- Test failures
- Bugs in production
- Unexpected behavior
- Performance problems
- Build failures
- Integration issues

This is most valuable exactly when it feels least convenient:
- Under time pressure, when guessing is tempting
- When "just one quick fix" seems obvious
- When you've already tried multiple fixes
- When a previous fix didn't work
- When you don't fully understand the issue

It applies just as much when:
- The issue seems simple (simple bugs still have root causes)
- You're in a hurry (rushing tends to cause rework)
- Someone wants it fixed immediately (systematic is faster than thrashing)

## The Four Phases

Complete each phase before moving to the next.

### Phase 1: Root Cause Investigation

Before attempting any fix:

1. **Read error messages carefully**
   - Don't skip past errors or warnings
   - They often contain the exact solution
   - Read stack traces completely
   - Note line numbers, file paths, error codes

2. **Reproduce consistently**
   - Can you trigger it reliably?
   - What are the exact steps?
   - Does it happen every time?
   - If it isn't reproducible, gather more data rather than guessing
   - A flaky test is a bug with a root cause. A test that fails intermittently and passes on rerun is not fixed — find the source of nondeterminism (shared state, time, ordering, randomness) before you claim the suite reliable. If the flaky test is outside your task's scope, report it honestly rather than fixing it

3. **Check recent changes**
   - What changed that could cause this?
   - Git diff, recent commits
   - New dependencies, config changes
   - Environmental differences

4. **Gather evidence in multi-component systems**

   When the system has multiple components (CI → build → signing, API → service → database), add diagnostic instrumentation before proposing fixes:

   ```
   For each component boundary:
     - Log what data enters the component
     - Log what data exits the component
     - Verify environment/config propagation
     - Check state at each layer

   Run once to gather evidence showing where it breaks,
   then analyze the evidence to identify the failing component,
   then investigate that specific component.
   ```

   Example (multi-layer system):
   ```bash
   # Layer 1: Workflow
   echo "=== Secrets available in workflow: ==="
   echo "IDENTITY: ${IDENTITY:+SET}${IDENTITY:-UNSET}"

   # Layer 2: Build script
   echo "=== Env vars in build script: ==="
   env | grep IDENTITY || echo "IDENTITY not in environment"

   # Layer 3: Signing script
   echo "=== Keychain state: ==="
   security list-keychains
   security find-identity -v

   # Layer 4: Actual signing
   codesign --sign "$IDENTITY" --verbose=4 "$APP"
   ```

   This reveals which layer fails (secrets → workflow works, workflow → build fails).

5. **Trace data flow**

   When the error is deep in the call stack, trace backward to the source. See `root-cause-tracing.md` in this directory for the complete technique.

   Quick version:
   - Where does the bad value originate?
   - What called this with the bad value?
   - Keep tracing up until you find the source
   - Fix at the source, not at the symptom

### Phase 2: Pattern Analysis

Find the pattern before fixing:

1. **Find working examples**
   - Locate similar working code in the same codebase
   - What works that's similar to what's broken?

2. **Compare against references**
   - If implementing a pattern, read the reference implementation completely
   - Read every line rather than skimming
   - Understand the pattern fully before applying it

3. **Identify differences**
   - What's different between the working and broken code?
   - List every difference, however small
   - Don't assume "that can't matter"

4. **Understand dependencies**
   - What other components does this need?
   - What settings, config, environment?
   - What assumptions does it make?

### Phase 3: Hypothesis and Testing

Use the scientific method:

1. **Form a single hypothesis**
   - State it clearly: "I think X is the root cause because Y"
   - Write it down
   - Be specific, not vague

2. **Test minimally**
   - Make the smallest possible change to test the hypothesis
   - One variable at a time
   - Don't fix multiple things at once

3. **Verify before continuing**
   - Did it work? Yes → Phase 4
   - Didn't work? Form a new hypothesis
   - Don't stack more fixes on top

4. **When you don't know**
   - Say "I don't understand X"
   - Don't pretend to know
   - Ask for help
   - Research more

### Phase 4: Implementation

Fix the root cause, not the symptom:

1. **Create a failing test case**
   - Simplest possible reproduction
   - Automated test if possible
   - One-off test script if no framework
   - Have this before fixing
   - Use the `megapowers:test-driven-development` skill for writing proper failing tests

2. **Implement a single fix**
   - Address the root cause you identified
   - One change at a time
   - No "while I'm here" improvements
   - No bundled refactoring

3. **Verify the fix**
   - Does the test pass now?
   - Are any other tests broken?
   - Is the issue actually resolved?

4. **If the fix doesn't work**
   - Stop
   - Count how many fixes you've tried
   - If fewer than 3: return to Phase 1, re-analyze with the new information
   - If 3 or more: stop and question the architecture (step 5 below)
   - Don't attempt fix #4 without an architectural discussion

5. **If 3+ fixes failed: question the architecture**

   Signs of an architectural problem:
   - Each fix reveals new shared state, coupling, or a problem in a different place
   - Fixes require "massive refactoring" to implement
   - Each fix creates new symptoms elsewhere

   Step back and question fundamentals:
   - Is this pattern fundamentally sound?
   - Are we sticking with it through sheer inertia?
   - Should we refactor the architecture rather than continue fixing symptoms?

   Discuss this with your human partner before attempting more fixes.

   This is not a failed hypothesis; it's a sign the architecture is wrong.

## Rationalizations to Watch For

Under pressure, a few thoughts tend to justify skipping the process. When you notice one, that's the cue to return to Phase 1:

- "Quick fix for now, investigate later" — the first fix sets the pattern; do it right from the start.
- "Just try changing X and see if it works" — that's guessing, not debugging.
- "Add multiple changes, run tests" — you can't isolate what worked, and it introduces new bugs.
- "Skip the test, I'll manually verify" — untested fixes don't stick.
- "It's probably X, let me fix that" — seeing a symptom isn't understanding the root cause.
- "I don't fully understand but this might work" — partial understanding guarantees bugs.
- "The pattern says X but I'll adapt it differently" — read the reference completely first.
- Listing "the main problems" as fixes before tracing data flow.
- "One more fix attempt" after already trying two or more — 3+ failures point at the architecture, not the next patch.

If 3+ fixes have failed, question the architecture (see Phase 4, step 5).

## Signals From Your Human Partner That You're Off Track

Watch for these redirections:
- "Is that not happening?" — you assumed without verifying
- "Will it show us...?" — you should have added evidence gathering
- "Stop guessing" — you're proposing fixes without understanding
- "Ultra-think this" — question fundamentals, not just symptoms
- "We're stuck?" (frustrated) — your approach isn't working

When you see these, return to Phase 1.

## Quick Reference

| Phase | Key Activities | Success Criteria |
|-------|---------------|------------------|
| 1. Root Cause | Read errors, reproduce, check changes, gather evidence | Understand what and why |
| 2. Pattern | Find working examples, compare | Identify differences |
| 3. Hypothesis | Form theory, test minimally | Confirmed or new hypothesis |
| 4. Implementation | Create test, fix, verify | Bug resolved, tests pass |

## When Process Reveals "No Root Cause"

If systematic investigation shows the issue is truly environmental, timing-dependent, or external:

1. You've completed the process
2. Document what you investigated
3. Implement appropriate handling (retry, timeout, error message)
4. Add monitoring/logging for future investigation

Keep in mind that most "no root cause" conclusions turn out to be incomplete investigation.

## Supporting Techniques

These techniques are part of systematic debugging and available in this directory:

- **`root-cause-tracing.md`** — trace bugs backward through the call stack to find the original trigger
- **`defense-in-depth.md`** — add validation at multiple layers after finding the root cause
- **`condition-based-waiting.md`** — replace arbitrary timeouts with condition polling

Related skills:
- **megapowers:test-driven-development** — for creating the failing test case (Phase 4, step 1)
- **megapowers:verification-before-completion** — verify the fix worked before claiming success

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
