---
name: systematic-debugging
description: Use when a bug, failing or flaky test, production incident, unexpected output, or performance regression has an unknown cause.
when_to_use: "Trigger phrases: why is this failing, bug, broken, flaky, CI is red, error with no obvious cause, something went wrong in the setup, exit 1 with no output, performance regression, what could cause this, build or deploy failed, pasted CI or build log, platform rejects the config."
metadata:
  short-description: Diagnose an unknown-cause failure before editing
---

# Systematic Debugging

Find the root cause before attempting a fix. A symptom patch without a causal
explanation creates a second unknown.

Read the complete failure. Reproduce it with a fast, deterministic loop that
goes red on the reported symptom, then minimize it. Trace the bad value or
condition backward across boundaries, compare it with a working path, and
inspect recent changes without assuming they are causal. For flakiness,
identify nondeterministic input, timing, shared state, or resource contention
instead of retrying until green. For slowness, use
[performance measurement](references/performance.md).

If output is filtered or truncated, retrieve the relevant raw diagnostics before
diagnosing the failure. If raw evidence remains unavailable or incomplete, treat
the diagnosis as inconclusive. Recover missing context through bounded reads.
Redact secrets before quoting captured output.
Do not repeat a side effect solely to recover output.

Before choosing the fix location, inspect callers of the implicated code and
identify whether they depend on a shared invariant. Add regression coverage for
sibling paths affected by the same cause, including paths absent from the
report. Fix the invariant where it belongs; avoid unrelated caller cleanup.

Preserve the child exit status. Distinguish launcher or filesystem access,
authentication, quota, and provider failures. When a sandbox, permission layer,
or harness restriction could explain the failure, verify outside that
restriction before declaring the dependency broken. A working session does not
prove a separate launch path is healthy.

Rank several falsifiable, evidence-backed hypotheses before testing any, then
test the cheapest decisive prediction while changing one variable. A refuted
hypothesis is evidence: revert the edits it motivated and update the model.
Tag temporary instrumentation so one search removes it. After the cause is confirmed, write and run a failing
regression test at a stable boundary before changing production code. Make the
smallest cause-level fix, then run the regression, relevant checks, and the
original failure path.

When a deterministic local test is impossible, agree on a substitute oracle
and record the environment, correlation key, pre-change failure, post-change
result, and monitoring window. After three failed fixes on one approach, stop,
report the evidence and remaining uncertainty, and reconsider the design.
