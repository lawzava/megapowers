---
name: systematic-debugging
description: Use to diagnose or fix bugs, failing or flaky tests, or unexpected behavior. Triggers on "why is this failing", "find the cause", "test suite is failing", or intermittent failures. Use TDD after finding the cause.
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

- Read repository instructions (they govern process), canonical `CONTEXT.md`
  if present (current domain vocabulary), relevant accepted ADRs when present
  (narrower design intent), and matching project memories when present
  (hidden historical hints: reverify before use). Surface conflicts;
  never silently resolve one.
- Read error output completely. Messages, stack traces, and line numbers often name the cause.
- Reproduce reliably. If you cannot, gather more data rather than guess.
- Actual observed behavior is authoritative for diagnosis. Complete the
  diagnosis before planning a change; documents describe intended state but
  do not overrule contradictory runtime evidence.
- Before forming hypotheses, build the smallest red-capable feedback loop that can distinguish failure from success. Minimize a slow integration or system oracle into a faster reproducer, but retain the slow oracle as ground truth for final verification.
- When automation cannot capture a production-only failure, ask the user to perform one concrete action and correlate it with a request or job identifier and timestamp.
- When performance is the symptom, record a controlled pre-change performance baseline under the same workload and environment used for the later comparison.
- A flaky test is a bug with a root cause, not something to retry into passing. A test that fails intermittently and passes on rerun is not fixed; find the source of nondeterminism (shared state, time, ordering, randomness) before you claim the suite reliable. If the flaky test is outside your task's scope, report it honestly rather than fixing it.
- Check recent changes: diffs, new dependencies, config, environmental differences.
- In multi-component systems (CI to build to signing, API to service to database), instrument each boundary: log what enters and exits each component and verify config propagation, then run once so the evidence shows which layer fails before you investigate that layer. Tag each temporary probe with a searchable label and its question; when answered, remove it or deliberately promote it to permanent monitoring.
- When the error surfaces deep in the call stack, trace the bad value backward to where it originates and fix at the source, not the symptom. See `debugging-techniques.md` in this directory for the full technique.

### Phase 2: Pattern Analysis

Goal: know exactly how the broken code differs from something that works. Find similar working code in the same codebase, or read the reference implementation completely rather than skimming. List every difference, however small, without assuming any of them cannot matter, and understand what dependencies, config, and assumptions the code relies on.

### Phase 3: Hypothesis and Testing

Goal: a confirmed root cause. Write a short list and rank hypotheses by evidence; among similarly supported hypotheses, test the cheapest decisive one first. State one specific hypothesis ("I think X is the root cause because Y") and test it with the smallest change that can confirm or refute it, one variable at a time. A refuted hypothesis means a new hypothesis, not additional fixes stacked on top. When you do not understand something, say so and research it rather than pretend.

### Phase 4: Implementation

Goal: root cause fixed and proven. Write a failing test that reproduces the bug before touching the fix; hand off to the `megapowers:test-driven-development` skill to write the test and the fix. Exercise a stable public seam or observable boundary, and derive the expected value independently of production logic; do not mock an internal helper or repeat the same calculation as production. Make one change that addresses the identified root cause. Curb unrequested tidying: a bug fix does not need surrounding cleanup, and bundled refactoring obscures the fix. Then verify the failing test passes, no other tests broke, and the original issue is actually resolved.

If the correction changes code behavior, TDD still applies to every deterministic behavior that can be exercised locally. Substitute evidence is allowed only with explicit agreement from your human partner, and only when the correction has no deterministic behavior that can be exercised locally because the failure is irreducibly external or nondeterministic. Document why and use a substitute oracle: record the conditions and correlation key, pre-change failure evidence, post-change success evidence under the same conditions, and monitoring for recurrence.

If the fix does not work, return to Phase 1 and re-analyze with the new information. After three failed fixes, stop treating it as a bug: fixes that keep exposing new coupling, demand large refactors, or push symptoms elsewhere point at the architecture. Question whether the pattern is fundamentally sound and discuss it with your human partner before any fourth attempt.

## Rationalizations to Watch For

Thoughts like "quick fix for now, investigate later", "just try changing X", "it's probably X", "skip the test, I'll verify manually", or "one more attempt" are the signal that you are guessing, not debugging. Any of them means return to Phase 1. Partner messages such as "stop guessing", "is that not happening?", or a frustrated "we're stuck?" carry the same meaning: you assumed without verifying, so go back and gather evidence.

## When the Process Reveals No Root Cause

If systematic investigation shows the issue is in fact environmental, timing dependent, or external, you have completed the process: document what you investigated, implement appropriate handling (retry, timeout, clear error message), and add monitoring for future investigation. Most "no root cause" conclusions turn out to be incomplete investigation.

## Supporting Techniques

Available in this directory:

- **`debugging-techniques.md`**: root-cause tracing, defense-in-depth validation, and condition-based waiting
- **`find-polluter.sh`**: bisect the test suite to find which earlier test pollutes shared state

Related skills:
- **megapowers:test-driven-development**: writes the failing test and the fix (Phase 4)
- **megapowers:verification-before-completion**: verify the fix worked before claiming success

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
