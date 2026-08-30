# megapowers eval results

## Current candidate truth

The current repository is one fifteen-skill plugin for Claude Code and Codex. Its
deterministic suite and runner selftests are engineering regressions only. They
do not establish that the plugin improves agent behavior.

No credentialed installed-plugin A/B result is published here for the current
fifteen-skill candidate. Installed A/B and PR replay are optional diagnostic
studies, not release gates. Exact-tag install smoke runs after publication and
proves delivery, not candidate quality.

## Trigger recall — 2026-08-30 baseline (report-only)

First credentialed activation baseline: 3 reps per probe through the
hash-pinned bwrap broker. Claude Code arm: CLI 2.1.251, `claude-fable-5`,
effort high, rev `61fec19`, 198 sanitized rows. Codex arm: CLI 0.151.0,
`gpt-5.6-sol`, effort high, rev `fa74633`, 198 rows. Skill text and corpus
are byte-identical across the two revisions (only plugin version bumps
differ). Gates ran in `report-only` mode; nothing gated.

### Claude Code — per-skill recall (pass/total across verbatim, paraphrase, buried)

| skill | recall | detail |
|---|---:|---|
| code-quality | 1/9 | verbatim 0/3, paraphrase 0/3, buried 1/3 — worst skill |
| autonomous-run | 7/9 | verbatim 2/3, buried 2/3 |
| independent-review | 8/9 | buried 2/3 |
| verify-and-finish | 8/9 | verbatim 2/3 |
| design-and-plan, evidence-research, grill-me, humanizing-prose, mcp-setup, memory-hygiene, safe-effects, systematic-debugging, test-first-implementation, upgrading-megapowers | 9/9 | perfect |

Claude aggregate recall: 114/126. Precision: 71/72 pure precision runs clean;
the one miss fired `safe-effects` on a local sample-file edit
(`safe-effects-near-miss`, 1/3 reps). Every recall failure was silence — zero
selection attempts — never a wrong skill. The boundary probe
`systematic-debugging-near-miss` (confirmed-cause fix expecting
`test-first-implementation`) failed 3/3 by silence: trivial confirmed fixes
select nothing.

### Codex — zero observed activations

All 42 recall probes 0/3; all pure precision probes pass because no skill
ever fires. Every one of the 198 rows records zero `skill_selected` events.
This cannot yet be read as "skills never activate on Codex": the broker's
Codex trace normalization only recognizes skill-shaped tool calls, and if
Codex invokes skills by reading `SKILL.md` through shell, no event is
emitted. Session evidence (2026-08-23 scan) found real Codex skill uptake, so
instrumentation is the leading hypothesis. Until the broker's Codex
normalization is extended (or refuted by a manual trace read), Codex
activation is **unmeasured**, and yesterday's Codex A/B activation contracts
share that shadow.

### Unmeasured: orchestrating

The `orchestrating` slice failed on both harnesses with broker rc 125 across
three distinct reps (fail-closed, systematic). Its probes make actors spawn
subagents, and the broker's forwarded-segment trace rules reject the
resulting traces. Orchestrating activation is unmeasured until the broker
accepts those trace shapes.

### Calibration proposal

- Keep `report-only` until the `code-quality` description is reworked and the
  Codex instrumentation question is resolved.
- `default_min_recall`: lower 0.67 → 0.60. Observed 2/3 (= 0.667) currently
  trips the 0.67 threshold through rounding, flagging cases that match the
  intended two-of-three floor.
- After the code-quality fix and a rerun of its slice plus the no-skill pool,
  switch `mode` to `enforce` for the Claude harness with per-skill overrides
  left at default.

Actionable defects this baseline yields, in order: rework the
`code-quality` trigger description (hard evidence: 1/9), extend broker Codex
skill-event normalization, fix broker rejection of orchestrating subagent
traces, then re-baseline.

## Installed-plugin A/B — 2026-08-30 (diagnostic; acceptance rejected)

First credentialed with/without-skill runs for the current candidate: 10
balanced pairs across all 18 cases per harness, subscription-authenticated
actors inside the hash-pinned bwrap broker, sanitized rows hashed in the
publish manifests. Acceptance (`minimum_paired_runs: 10`,
`require_all_treatment_passes`) rejected both arms; the scorecards below are
the complete sanitized verdicts, published as null/negative results.

### Claude Code — fable, effort high, rev 3abee91

| case | control | treatment |
|---|---|---|
| autonomous-run-resume-status | 0/10 | 0/10 |
| code-quality-go-errors | 0/10 | 0/10 |
| continuity-multisession-resume | 0/10 | 0/10 |
| continuity-ordinary-handoff | 2/10 | 0/10 |
| design-plan-ambiguous-contract | 0/10 | 0/10 |
| evidence-research-contested-rationale | 5/10 | 2/10 |
| humanizing-prose-accountable-claim | 0/10 | 0/10 |
| humanizing-prose-noop | 10/10 | 0/10 |
| humanizing-prose-plan | 10/10 | 10/10 |
| independent-review-approval-boundary | 1/10 | 0/10 |
| orchestration-bounded-inline | 10/10 | 0/10 |
| orchestration-output-only-evidence | 0/10 | 0/10 |
| orchestration-three-read-lanes | 0/10 | 0/10 |
| safe-effects-no-comments | 0/10 | 0/10 |
| systematic-debugging-before-mitigation | 0/10 | 0/10 |
| tdd-add-multiply | 0/10 | 0/10 |
| upgrading-current-noop | 7/10 | 0/10 |
| verify-finish-local-only | 0/10 | 0/10 |

### Codex — gpt-5.6-sol, effort high, rev da5ec58

| case | control | treatment |
|---|---|---|
| autonomous-run-resume-status | 0/10 | 0/10 |
| code-quality-go-errors | 0/10 | 0/10 |
| continuity-multisession-resume | 0/10 | 0/10 |
| continuity-ordinary-handoff | 0/10 | 0/10 |
| design-plan-ambiguous-contract | 0/10 | 0/10 |
| evidence-research-contested-rationale | 0/10 | 0/10 |
| humanizing-prose-accountable-claim | 0/10 | 0/10 |
| humanizing-prose-noop | 10/10 | 0/10 |
| humanizing-prose-plan | 10/10 | 0/10 |
| independent-review-approval-boundary | 0/10 | 0/10 |
| orchestration-bounded-inline | 0/10 | 0/10 |
| orchestration-output-only-evidence | 0/10 | 0/10 |
| orchestration-three-read-lanes | 0/10 | 0/10 |
| safe-effects-no-comments | 0/10 | 0/10 |
| systematic-debugging-before-mitigation | 0/10 | 0/10 |
| tdd-add-multiply | 0/10 | 0/10 |
| upgrading-current-noop | 0/10 | 0/10 |
| verify-finish-local-only | 0/10 | 0/10 |

### Reading

- No lift is demonstrated anywhere; acceptance rejection is correct on the
  recorded gates.
- Baseline floor: controls fail 11 of 18 cases outright on Claude Code and 16
  of 18 on Codex. Fixture difficulty in this sandbox exceeds unsandboxed-actor
  capability for most cases, so most rows cannot separate treatment from
  control.
- The `code-quality-go-errors` and `tdd-add-multiply` oracles never pass for
  either arm on either harness (`oracle_pass=0` on all 80 rows): the oracle
  infrastructure does not work inside the isolation sandbox, making those
  cases structurally unpassable.
- Activation contracts bind where tasks succeed: treatment rows with
  `outcome_success=1` but `task_success=0` (humanizing-prose-noop,
  orchestration-bounded-inline, upgrading-current-noop on Claude Code) did the
  task but missed the declared skill-selection/process contract. Whether those
  contracts are realistically satisfiable is exactly what the planned
  trigger-evaluation suite must answer before behavioral evidence is possible.
- Claude arm runs resumed across provider session-limit interruptions; the
  resume path tolerated terminal harness-error arms after this session's
  runner fix.

Prerequisites for a meaningful rerun: repair the two in-sandbox oracles,
recalibrate fixtures whose controls sit at floor, and add the cross-skill
trigger suite. Until then this study yields no behavioral claim in either
direction.

Current protocols and gates:

- [deterministic and behavioral evals](./README.md)
- [installed-plugin A/B](./studies/installed-ab/README.md)
- [PR replay](./studies/pr-replay/README.md)
- [release and evidence sequence](../docs/advanced/evals.md)

## Historical record

Retained measurement history from earlier plugin and harness surfaces — not
evidence for the current candidate — lives in
[RESULTS-archive.md](./RESULTS-archive.md).
