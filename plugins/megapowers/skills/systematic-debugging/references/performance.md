# Performance measurement

Use this reference when the failure is slowness, memory growth, or a throughput
or latency regression. Measure before reading source for a fix; code inspection
cannot establish a win.

## Build the measurement

1. Pick a realistic workload that reproduces the complaint. Name the
   dimensions that can move the result, such as data size, history, state, and
   concurrency. If no case reproduces the complaint, fix the reproduction first.
2. Fix one metric, the direction that counts as better, and a stop condition.
   Pair a target with a minimum number of attempts so one lucky run cannot end
   the work. Use the user's numbers when given.
3. Prove the harness is sensitive: the target case shows the symptom and an
   easier case separates from it. Then freeze the harness as one repeatable
   command. Report the median of several runs, not a single run.
4. Record the baseline and a passing run of the tests that must stay green
   before any change.

## Change one thing at a time

Tie each hypothesis to a mechanism the measurement shows. A family of fixes
earns an attempt only when the profile shows its signal:

- elimination of work nobody consumes;
- smaller or parallel pieces when cost scales with input size;
- caching repeated work on identical inputs, with its invalidation named;
- an index, queue, or other cheaper intermediate on the hot path;
- batching many small operations that each pay a fixed overhead;
- deferring work until first use;
- moving unavoidable work away from the moment someone waits.

Make one change, measure with the frozen harness, and run the regression tests.
Keep the change only when the metric moves beyond run-to-run noise and the tests
stay green; otherwise revert it completely. Do not stack unmeasured changes.
Keep a private log of each attempt, its before and after numbers, and whether
it was kept. Correctness outranks the number. Do not relax the stop condition
to declare success.

## Captured profiles and traces

When the evidence is an existing profile, trace, heap snapshot, or dump, read
it rather than re-running the workload. Load it into a queryable form, such as
one row per sample or frame, before reading it. Map the hot frame to a source
file and symbol; a frame without source mapping is not yet a diagnosis. Without
a paired before-and-after capture, report the strongest supported hypothesis,
not a confirmed cause.

Report the baseline, final measurement, delta, run count, harness command, and
kept and reverted attempts.
