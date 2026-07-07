---
name: systematic-debugging
description: Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes — including when asked to fix something ("the test suite is failing", "find the cause", "fix this bug", "why is this failing"), and for flaky or intermittent tests. After finding the root cause, hand off to test-driven-development to write the fix.
license: MIT
---

# Systematic Debugging

## Overview

Random fixes waste time and create new bugs. Quick patches mask the underlying issue and it resurfaces later.

**Core principle:** find the root cause before attempting a fix, always. A fix that only addresses the symptom is not a fix. Find the root cause before you change any code.

## When to Use

Any technical issue: test failures, production bugs, unexpected behavior, performance, build or integration problems. It matters most exactly when it feels least convenient: under time pressure, when one quick fix looks obvious, or after previous fixes have already failed. Simple bugs still have root causes, and systematic is faster than thrashing.

## The Four Phases

Complete each phase before moving to the next.

### Phase 1: Root Cause Investigation

Goal: understand what is failing and why, backed by evidence.

- Read error output completely. Messages, stack traces, and line numbers often name the cause.
- Reproduce reliably. If you cannot, gather more data rather than guess.
- A flaky test is a bug with a root cause, not something to retry into passing. A test that fails intermittently and passes on rerun is not fixed; find the source of nondeterminism (shared state, time, ordering, randomness) before you claim the suite reliable. If the flaky test is outside your task's scope, report it honestly rather than fixing it.
- Check recent changes: diffs, new dependencies, config, environmental differences.
- In multi-component systems (CI to build to signing, API to service to database), instrument each boundary: log what enters and exits each component and verify config propagation, then run once so the evidence shows which layer fails before you investigate that layer.
- When the error surfaces deep in the call stack, trace the bad value backward to where it originates and fix at the source, not the symptom. See `root-cause-tracing.md` in this directory for the full technique.

### Phase 2: Pattern Analysis

Goal: know exactly how the broken code differs from something that works. Find similar working code in the same codebase, or read the reference implementation completely rather than skimming. List every difference, however small, without assuming any of them cannot matter, and understand what dependencies, config, and assumptions the code relies on.

### Phase 3: Hypothesis and Testing

Goal: a confirmed root cause. State one specific hypothesis ("I think X is the root cause because Y") and test it with the smallest change that can confirm or refute it, one variable at a time. A refuted hypothesis means a new hypothesis, not additional fixes stacked on top. When you do not understand something, say so and research it rather than pretend.

### Phase 4: Implementation

Goal: root cause fixed and proven. Write a failing test that reproduces the bug before touching the fix; hand off to the `megapowers:test-driven-development` skill to write the test and the fix. Make one change that addresses the identified root cause. Curb unrequested tidying: a bug fix does not need surrounding cleanup, and bundled refactoring obscures the fix. Then verify the failing test passes, no other tests broke, and the original issue is actually resolved.

If the fix does not work, return to Phase 1 and re-analyze with the new information. After three failed fixes, stop treating it as a bug: fixes that keep exposing new coupling, demand large refactors, or push symptoms elsewhere point at the architecture. Question whether the pattern is fundamentally sound and discuss it with your human partner before any fourth attempt.

## Rationalizations to Watch For

Thoughts like "quick fix for now, investigate later", "just try changing X", "it's probably X", "skip the test, I'll verify manually", or "one more attempt" are the signal that you are guessing, not debugging. Any of them means return to Phase 1. Partner messages such as "stop guessing", "is that not happening?", or a frustrated "we're stuck?" carry the same meaning: you assumed without verifying, so go back and gather evidence.

## When the Process Reveals No Root Cause

If systematic investigation shows the issue is in fact environmental, timing dependent, or external, you have completed the process: document what you investigated, implement appropriate handling (retry, timeout, clear error message), and add monitoring for future investigation. Most "no root cause" conclusions turn out to be incomplete investigation.

## Supporting Techniques

Available in this directory:

- **`root-cause-tracing.md`**: trace bugs backward through the call stack to find the original trigger
- **`defense-in-depth.md`**: add validation at multiple layers after finding the root cause
- **`condition-based-waiting.md`**: replace arbitrary timeouts with condition polling
- **`find-polluter.sh`**: bisect the test suite to find which earlier test pollutes shared state

Related skills:
- **megapowers:test-driven-development**: writes the failing test and the fix (Phase 4)
- **megapowers:verification-before-completion**: verify the fix worked before claiming success

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
