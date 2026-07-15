# debugging-loop-guidance

Artifact oracle for the contract selected from two fresh-context behavioral
probes. The prompts and omissions below are the behavioral RED evidence. The
grep markers only prevent the resulting guidance from drifting later.

## Fresh-context prompts

Probe 1:

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md and current systematic-debugging. Scenario: 1% production export bug, no test, possible ordering/shared state/slow downstream, user available for manual clicks. State first actions, gate before hypotheses, handling hypotheses, instrumentation cleanup, when regression test impossible. No edits. Return response and missing elements with line refs. Do not read external.
```

Probe 2:

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md and current plugins/megapowers/skills/systematic-debugging/SKILL.md. Scenario: endpoint latency regressed from an unknown prior level; logs show three plausible causes, a large integration test can reproduce it slowly, and a proposed regression test mocks an internal helper using the same calculation as production. State the diagnostic plan, hypothesis order, reproducer strategy, baseline, correct test seam, and temporary instrumentation lifecycle. Identify which requested elements are missing from current skill with exact line refs. No edits.
```

## Observed RED, 2026-07-15

The combined responses omitted these requested contracts:

- Neither probe response made construction of the red-capable loop a gate
  before hypothesis work; the current skill only said to reproduce reliably.
- Probe 1 omitted an explicit user-assisted correlation step when automation
  was unavailable. It also proposed temporary instrumentation without a
  remove-or-promote lifecycle.
- Probe 2 omitted minimizing the slow integration oracle while retaining that
  oracle as the ground truth for final verification.
- A pre-change performance baseline. The current skill names performance as an
  applicable symptom at line 17, but Phase 1 at lines 23 to 32 has no baseline.
- Hypothesis order by evidence and then test cost. Line 40 requires one specific
  hypothesis at a time, but gives no ordering rule for several plausible causes.
- Minimization of the slow integration-test oracle. Lines 28 and 40 require a
  reproduction and the smallest confirming change, not a smaller reproducer.
- A regression test at a stable public seam with an expected value derived
  independently of production logic. Line 44 only asks for a failing test.
- Tagged temporary instrumentation and explicit cleanup. Line 31 adds boundary
  instrumentation without a lifecycle.
- A documented substitute oracle when a deterministic regression test is
  genuinely impossible. Lines 44 and 54 offer a test or monitoring, but do not
  define the substitute evidence contract.

## Deterministic RED

`solve.sh` emits one `OK` or `MISSING` marker per observed omission. `check.sh`
rejects a missing `out.txt`, any absent marker, and every `MISSING` marker. On
the pre-guidance skill it ends with:

```text
RED: debugging-loop-guidance contract incomplete
```

## Checker mutation test, 2026-07-15

With `out.txt` deliberately absent:

```bash
WORKDIR="$(mktemp -d)" bash evals/scenarios/debugging-loop-guidance/check.sh
```

The checker exited 1 with:

```text
missing evidence: out.txt
```

With a complete expected marker set except `manual-correlation`:

```bash
WORKDIR=/tmp/debugging-loop-missing-marker bash evals/scenarios/debugging-loop-guidance/check.sh
```

The checker exited 1 with:

```text
missing marker: manual-correlation
```
