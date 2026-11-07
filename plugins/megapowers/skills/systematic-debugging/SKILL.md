---
name: systematic-debugging
description: "For unknown failures, inspect raw diagnostics and test a cause before fixes or mitigation; missing evidence means inconclusive diagnosis. Load for reproduction, competing causes, flakiness, cross-system failures, or performance investigation. A known-cause fix or clear diagnostic with an established remedy needs no full workflow unless a harness, sandbox, permission layer, or truncated output could explain it."
---

# Systematic Debugging

Find the root cause before attempting a fix.

Read the complete failure. Group failing checks by likely shared cause.
Reproduce and minimize the symptom with a fast deterministic loop. Trace bad
values backward across boundaries, compare a working path, and inspect recent
changes without assuming causation. Check status pages for hosted-service
failures. Investigate flaky inputs, timing, shared state, and contention;
do not retry until green. Use
[performance measurement](references/performance.md) when a reproducible slow
path and measurement access are available, not for a timeout report alone.

Retrieve raw diagnostics behind filtered or truncated output. Missing or
incomplete evidence leaves diagnosis inconclusive. Redact secrets before
quoting; never repeat a side effect just to recover output.

Inspect callers and shared invariants before choosing the fix location. Cover
sibling paths affected by the same cause, including unreported paths. Fix the
invariant where it belongs; avoid unrelated caller cleanup.

Preserve child exit status. Distinguish launcher/filesystem, authentication,
quota, and provider failures. Test suspected sandbox or harness restrictions
through an authorized unrestricted path before declaring a dependency broken;
otherwise report that uncertainty. A working session does not prove another
launch path healthy.

Test the cheapest decisive prediction, changing one variable. When evidence
supports competing explanations, rank several falsifiable hypotheses; otherwise
collect diagnostics rather than invent rankings. For a refuted hypothesis,
revert the edits it motivated. Tag temporary instrumentation for removal. After
confirming the cause, run a failing regression at a stable boundary before
production edits. Make the smallest cause-level fix; run the regression,
relevant checks, and original failure path.

If deterministic local testing is impossible, agree on a substitute oracle;
record environment, correlation key, before/after results, and monitoring window.
After two failed fixes on one premise, test that premise. After three failures
on one approach, record evidence and uncertainty, then switch approach.

After a long diagnosis, propose a cheap permanent signal; add it only on request.

Keep evidence requests and mitigation drafts inline unless a durable artifact
is requested or needed for handoff. Missing diagnostics do not justify scaffolding.
