---
name: systematic-debugging
description: Use to diagnose or fix bugs, failing or flaky tests, or unexpected behavior. Triggers on "why is this failing", "find the cause", "test suite is failing", or intermittent failures. Use test-driven development after finding the cause.
license: MIT
---

# Systematic Debugging

**Core principle:** find the root cause before attempting a fix. A symptom-level
patch is not a diagnosis.

## Investigate

Read the failure completely and reproduce it with the smallest reliable
feedback loop. Treat observed behavior as the diagnostic authority; consult
project context and recent changes for intent and possible causes. For a
production-only failure, collect one correlation identifier and trace it across
the relevant boundaries. For a flaky failure, identify the nondeterministic
input or shared state rather than retrying to green.

Trace a bad value or condition back to its source. Compare with working behavior
in the same system, then state a specific, evidence-backed hypothesis. Test the
cheapest decisive hypothesis one variable at a time. A failed hypothesis is new
evidence, not a reason to stack another patch on top.

## Fix and Verify

After confirming the cause, write a failing test at a stable observable boundary
before changing the implementation. Make the smallest change that addresses the
cause, then run the regression test, relevant checks, and the original failure
path. Do not bundle unrelated cleanup.

If local deterministic testing is impossible, agree on a substitute oracle and
record conditions, correlation key, pre-change failure, post-change success,
and recurrence monitoring. After three failed fixes, stop and reconsider the
design with the requirement owner instead of continuing to guess.

`debugging-techniques.md` covers root-cause tracing and condition-based waiting.
Resolve `scripts/find-polluter.sh` from this skill's installed directory.
`scripts/find-polluter.sh <path-to-check> <test-glob>` locates the earlier test that
creates shared state; it refuses a zero-match glob rather than reporting clean.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
