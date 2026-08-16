---
name: systematic-debugging
description: Use when a bug, failing or flaky test, production incident, unexpected output, or performance regression has an unknown cause.
---

# Systematic Debugging

Find the root cause before attempting a fix. A symptom patch without a causal
explanation creates a second unknown.

Read the complete failure and reproduce it with the smallest reliable loop.
Trace the bad value or condition backward across boundaries, compare it with a
working path, and inspect recent changes without assuming they are causal. For
flakiness, identify nondeterministic input, timing, shared state, or resource
contention instead of retrying until green.

State one evidence-backed hypothesis and test the cheapest decisive prediction
while changing one variable. A failed hypothesis is evidence; update the model
before trying another. After the cause is confirmed, write and run a failing
regression test at a stable boundary before changing production code. Make the
smallest cause-level fix, then run the regression, relevant checks, and the
original failure path.

When a deterministic local test is impossible, agree on a substitute oracle
and record the environment, correlation key, pre-change failure, post-change
result, and monitoring window. After three failed fixes on one approach, stop,
report the evidence and remaining uncertainty, and reconsider the design.
