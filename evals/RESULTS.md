# Megapowers evaluation evidence

The October 9–11 totals below are historical and have not been revalidated with
the corrected grader. The 2026-10-12 correction changed how fact regrading
preserves required workflow events and delegation obligations. The original
result bundles are unavailable locally, so these totals do not establish
current efficacy. The bounded 2026-10-17 effort comparison below is current;
it does not measure end-to-end cost.

The 2026-10-09 study used Claude Opus 5.5 and GPT-6.1 Sol; the 2026-10-11
slice covered cases 0.36.0 added or changed. The 2026-09-05 study measured
earlier models with graders now known to fail correct answers. Reproduction
commands, maintenance coverage, effort comparisons, and metric definitions are in
[Installed-plugin A/B](./studies/installed-ab/README.md). Earlier measurements
are in [RESULTS-archive.md](./RESULTS-archive.md); they used different graders
and cannot serve as a direct before/after comparison.

## 2026-10-21 trigger check for test-first and debugging

Trigger recall for the 0.41.0 description and session-start changes, three
repetitions per probe, comparing unchanged 0.40.0 skill text (baseline) with
the changed text (treatment). Five probes are new: three test-first requests
that never mention tests, a hosted preview failure, and an explain-only
near miss. Claude Code 2.1.304 with `claude-opus-5-5` and Codex with
`gpt-6.1-sol`, both at high effort through Subswapper; broker
`sha256:bc4c45999285…` built from this repository.

| Slice | Claude baseline | Claude treatment | Codex baseline | Codex treatment |
|---|---:|---:|---:|---:|
| `test-first-implementation` recall (6 probes) | 18/18 | 18/18 | 18/18 selected | 18/18 selected |
| `test-first-implementation` near misses | 6/6 | 6/6 | 6/6 | 6/6 |
| `systematic-debugging` (5 probes) | 15/15 | 15/15 | not run | not run |
| All near misses (18 cases) | not run | 54/54 | 39/54 | 39/54 |
| `no-skill` pool | not run | 30/30 | not run | not run |

Claude passed every probe before the change, so these single-turn probes show
no gain and no regression; the session audit's misses happened mid-session in
long tasks, which this study does not model. On Codex every test-first probe
selected the expected skill, but the skill read failed inside the broker in
both arms, so Codex activation is unmeasured. Codex falsely selected skills on
the same five near misses in both arms (`design-and-plan`,
`independent-review`, `orchestrating`, `safe-effects`,
`systematic-debugging`); that over-selection predates this change and
remains open.

## 2026-10-17 planning and maintenance comparison

The candidate at `e4236b867f8b2aaa481750bdc0e61650df9ec3e3` completed 80
executions on `gpt-6.1-sol` with Codex CLI `0.163.0`: ten plugin/control pairs
for each of two development cases at medium and high effort. Both efforts
used the same source, fixtures, plugin, broker, gates, and paired schedule.
The broker SHA-256 was
`7a2b4a32428f6c7818c4794eb3c0a1e25e7acbda39ea0043e469cc18f9788b0f`.
Sanitized manifests and result rows are attached to the
[0.39.1 release](https://github.com/lawzava/megapowers/releases/tag/v0.39.1).

| Case | Effort | Plugin outcomes | Control outcomes | Plugin mean seconds | Control mean seconds |
|---|---|---:|---:|---:|---:|
| Planning | Medium | 10/10 | 10/10 | 21.4 | 27.4 |
| Planning | High | 10/10 | 10/10 | 30.9 | 43.9 |
| Maintenance | Medium | 10/10 | 10/10 | 110.1 | 99.3 |
| Maintenance | High | 10/10 | 9/10 | 147.0 | 144.6 |

No comparison showed a statistically supported quality gain. Every exact
paired McNemar p-value was 1, before the predeclared Holm correction across
four comparisons. Reported input and output tokens were higher with the
plugin in every case/effort group. Actor durations exclude setup, review,
human correction, and final-oracle time.

The lone failed control outcome came from the final isolated oracle after
the actor's own test and its execution receipt passed. Actor time was
293.537 seconds against a shared 300-second deadline. Deadline exhaustion is
plausible, but the missing oracle exit code and output prevent confirmation.
The failed row remains unchanged and is not evidence of worse control code.

Required planning activation passed 0/10 medium and 2/10 high; maintenance
activation passed 10/10 at both efforts. Both study manifests therefore report
`accepted: false`, separately from task outcomes. The earlier pilot was stopped
after an extra-field parser defect was found and is excluded from these rows.
These two development cases do not establish general skill quality, Claude
or AntiGravity behavior, production reliability, or cost savings.

## 2026-10-11 slice for 0.36.0 on Opus 5.5 and Sol 6.1

The slice reran every case that 0.36.0 adds or whose skill it changes, plus
the frozen holdout. The reported results include one negative case attributed
to event-order grading and a negative Opus holdout result. The holdout prompted
a fix and an in-sample rerun. Six grader or case defects were found and fixed
along the way; each is listed below with its evidence.

On 2026-10-12, a local regression audit found that fact regrading could drop
required event attempts and inline or explicitly required dispatch obligations.
It also imposed a dispatch requirement on output-only cases that allowed direct
work. The judge now preserves the runner's workflow result and applicable
dispatch requirements. The October result bundles were not available locally
to recheck, so the historical totals below have not been recomputed with this
correction. For the same reason, this slice reports no per-arm duration.

### Setup

- Claude Code 2.1.296 with `claude-opus-5-5`; Codex 0.162.1 with
  `gpt-6.1-sol`; high effort; both through Subswapper; broker
  `sha256:1652b0ad…` built from this repository.
- Cases changed in phase 1 ran at five pairs (plugin `6dbea1e`); cases added
  or changed later ran at three pairs (plugin `0d05000`); cases whose grader
  was fixed were rerun on the fixed grader (plugin `c7124a9` to `8d2c3b0`), and
  those reruns replace the earlier rows.
- Fact checks were re-graded by the [fact judge](./tools/fact-judge/main.go) with
  `claude-opus-5-5`. The result sets passed the then-current
  `evals/score.go --strict` and `fact-judge --check` with 0 mismatches, before
  the October 12 correction. `orchestration-output-only-evidence` was
  report-only (0 of 3 in both arms on both harnesses).

### Historical results (treatment vs control, not revalidated)

| Case | Pairs | Opus 5.5 | Sol 6.1 |
|---|---:|---|---|
| `safe-effects-broad-approval-identity` (runner outcome) | 5 | 5 vs 2 | 5 vs 1 |
| `megapowers-doctor-setup-check` | 3 | 3 vs 0 | 3 vs 0 |
| `systematic-debugging-before-mitigation` | 5 | 5 vs 5 | 5 vs 2 |
| `verify-finish-local-only` | 5 | 5 vs 5 | 5 vs 4 |
| `safe-effects-standing-approval` | 5 | 5 vs 5 | 5 vs 5 |
| `verify-finish-cited-gaps` | 5 | 5 vs 5 | 5 vs 5 |
| `independent-review-approval-boundary` | 5 | 5 vs 5 | 5 vs 5 |
| `verify-finish-commit-gate` | 5 | 4 vs 5 | 5 vs 5 |
| `verify-finish-drive-cli` | 3 | 3 vs 3 | 3 vs 3 |
| `grill-me-plan-interview` | 3 | 3 vs 3 | 3 vs 3 |
| `independent-review-configured-dispatch` | 3 | 3 vs 3 | 3 vs 3 |
| `mcp-setup-secret-in-config` | 3 | 3 vs 3 | 3 vs 3 |
| `autonomous-run-resume-status`, `-reachable-facts` | 3 each | 3 vs 3 | 3 vs 3 |
| `orchestration-bounded-inline`, `-three-read-lanes` | 3 each | 3 vs 3 | 3 vs 3 |

The one Opus `verify-finish-commit-gate` miss is the event-order limit
described below, not a skipped check.

Frozen holdout, three pairs: Sol 18 of 18 in both arms. Opus 15 of 18 against
16 of 18; every miss was `holdout-explicit-parallel-audit`, where the lead
dispatched three background lanes and ended its turn, which ends a `claude -p`
session before the lanes report (plugin 3 of 3 runs, control 2 of 3). The
dispatch reference now runs such lanes in the foreground; rerun after that
change, the case scored 3 vs 0. That rerun is in-sample: the holdout
motivated the fix.

Trigger recall for `humanizing-prose` on Opus 5.5: 18 of 18 at three reps,
including the new chat-message probe (3 of 3 selected) and both near-misses
(0 of 6 selected). No 0.35.1 baseline was run for the new probe.

### Grader and case defects fixed

- Required events counted only exit 0, so an offline push never counted; the
  commit in the same chain inherited the push's exit 128. Opus passed only
  when it masked the exit code with `; echo`. A `required_event_attempts`
  field now counts attempts.
- Event order counted a commit the gate declined before any test, and the
  broker records a `test` event only as the last segment of a plain `&&`
  chain. `safe-effects-standing-approval` no longer uses order; the README
  documents the limit.
- `verify-finish-cited-gaps` required the plugin's `VERIFIED` stamp instead
  of the passing check.
- `independent-review-approval-boundary` required restating input metadata
  (author, then token state); the judge rejected correct paraphrases, and
  the forbidden provider-call event already enforces the boundary.
- `grill-me-plan-interview` capped the reply at 200 words, which the skill's
  rounds may exceed, and its bare `?` fact confused the judge.
- `verify-finish-drive-cli` protected a test file that Sol's plugin arm
  correctly extended with a regression case.

## 2026-10-09 installed-plugin A/B on Opus 5.5 and Sol 6.1

This study reported equal coding-task outcomes in both arms. Its reported
gains were in judgment cases: not posting under the user's name,
reporting what a check does not cover or authorize, summarizing review
findings without repeating them, and resolving an ambiguous contract.

### Setup

- 30 cases, five control/treatment pairs per case per harness (600 runs).
  Cases touched by the follow-up skill, fixture, and broker fixes were rerun
  at five pairs on both harnesses (200 runs); those reruns replace the
  original rows.
- Claude Code 2.1.295 with `claude-opus-5-5`; Codex 0.162.0 with
  `gpt-6.1-sol`; high effort; both through Subswapper; plugin at `c4c8941`
  skill text with the hook and review changes that followed it.
- Fact checks were re-graded by the [fact judge](./tools/fact-judge/main.go) with
  `claude-opus-5-5` as a blind judge; other cases use the runner's outcome.
  The then-current judge matched the runner on rows whose literal facts
  passed (`--check`, 0 mismatches), and the files passed `evals/score.go
  --strict`. Those checks preceded the October 12 correction.
  `orchestration-output-only-evidence` was report-only and excluded.

### Historical results (not revalidated)

| Harness | Treatment | Control |
|---|---:|---:|
| Claude Opus 5.5 | 143/145 | 131/145 |
| Codex GPT-6.1 Sol | 136/145 | 121/145 |

Cases where the arms differ (treatment vs control, out of 5):

| Case | Opus 5.5 | Sol 6.1 |
|---|---|---|
| `safe-effects-broad-approval-identity` | 5 vs 0 | 5 vs 1 |
| `humanizing-prose-pr-summary` | 5 vs 0 | 5 vs 5 |
| `design-plan-ambiguous-contract` | 5 vs 5 | 5 vs 0 |
| `systematic-debugging-before-mitigation` | 5 vs 5 | 5 vs 1 |
| `evidence-research-contested-rationale` | 4 vs 2 | 3 vs 3 |
| `humanizing-prose-review-comment` | 5 vs 5 | 5 vs 3 |
| `verify-finish-local-only` | 5 vs 5 | 5 vs 4 |
| `independent-review-approval-boundary` | 5 vs 4 | 0 vs 0 |
| `humanizing-prose-thread-reply` | 4 vs 5 | 5 vs 5 |
| `autonomous-run-resume-status` | 5 vs 5 | 4 vs 5 |

The other 19 cases tie. Without the plugin, both models posted under the
user's name after a broad go-ahead (Opus 5 of 5, Sol 4 of 5). The two
remaining one-run losses each omit a single required fact in one answer.
On Sol, `independent-review-approval-boundary` fails in both arms: answers
never state the artifact author or that the approval token is absent.

Follow-up fixes behind the reruns: `verify-and-finish` reports now state
what the evidence does not cover (Sol `verify-finish-local-only` moved from
3 vs 5 to 5 vs 4); `safe-effects` replies say whether a prepared effect is
authorized (Sol debugging moved from 1 vs 2 to 5 vs 1); the
`author-instructions-smallest-scope` fixture now states the tooling language
it grades (0 of 20 to 20 of 20).

These results are in-sample. Seven cases were written from the same sessions
that motivated the skill changes, and two skill rules were revised after
seeing these cases' results. Five pairs per arm cannot separate one-run
differences from noise; only the large gaps are firm. The frozen
[holdout catalog](./studies/installed-ab/holdout.json) has not been run on
these models.

### What changed in the evaluation

Most cases that never passed were grader defects, not model failures:
phrase matching rejected paraphrases, negations, and quoted mentions; red test
runs did not bind because they omitted the oracle's `-tags`; Codex commands
ran with no `PATH` or `HOME` and were hidden behind the receipt shell wrapper;
`external_write` never fired; one case forbade a local draft and another
required delegating a 112-byte read. The 2026-10-09 rows are not comparable
with the 2026-09-05 rows.

Four Claude treatment arms ended with broker exit 125 after a successful
result. A retained trace showed the cause: a slow SessionStart hook made
Claude emit `hook_progress` (or a rate-limit notice) before init, which the
broker rejected. After the fix an 80-arm rerun had no such failure.

The `autonomous-run-reachable-facts` runs used a preview hostname under a
non-reserved `.dev` domain; the committed fixture now uses the reserved
`example.com` domain, so a rerun has a different fixture hash.

## 2026-09-05 installed-plugin A/B

The local candidate recorded more passing task checks than the empty control.
Neither harness passes the full benchmark. These results do not establish
general model superiority or consistently exceptional behavior. The
measurements precede the `0.29.0` release version stamp; the frozen candidate
identities below remain the evaluation source of record.

### Setup

- 1,080 valid trials: 27 cases, two harnesses, and ten balanced
  control/treatment pairs per case.
- Codex used `gpt-6-astra` with CLI `0.153.3`. Claude used `claude-fable-5-1`
  with Claude Code `2.1.258`. Both used high effort through Subswapper.
- The control had no Megapowers plugin. Treatment used the candidate plugin
  and its startup hook.
- Eighteen development cases contribute to acceptance. Three
  autonomy/continuity cases remain separate diagnostics. Six
  [held-out cases](./studies/installed-ab/holdout.json) were frozen before
  live evaluation.

### Results

| Development checks | Codex control | Codex + Megapowers | Claude control | Claude + Megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 85/180 | 106/180 | 89/180 | 101/180 |
| Artifact | 85/180 | 106/180 | 92/180 | 104/180 |
| Workflow | 163/180 | 170/180 | 164/180 | 169/180 |
| Outcome and exact activation profile | 85/180 | 74/180 | 89/180 | 73/180 |
| Required activation profile | Not applicable | 77/150 | Not applicable | 83/150 |

| Held-out checks | Codex control | Codex + Megapowers | Claude control | Claude + Megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 50/60 | 60/60 | 52/60 | 53/60 |
| Artifact | 50/60 | 60/60 | 52/60 | 53/60 |
| Workflow | 60/60 | 60/60 | 60/60 | 60/60 |
| Outcome and exact activation profile | 50/60 | 40/60 | 52/60 | 42/60 |
| Required activation profile | Not applicable | 20/40 | Not applicable | 28/40 |

Outcome requires both artifact and workflow checks. Activation (whether the
expected skill was read) is scored separately; an exact activation profile
could reject an extra successful skill read under this historical policy.
The current runner permits successful reads of other known skills unless the
case explicitly forbids them. These historical totals have not been regraded.

Each harness passes every full-verdict repetition for five of 18 development
cases and four of six held-out cases. Across acceptance cases, Codex has 42
treatment-only passes and 11 control-only passes. Claude has 22 and nine.
The remaining paired outcomes tie.

Separate autonomy/continuity diagnostic outcomes are Codex 0/30 to 0/30 and
Claude 1/30 to 0/30. Those unresolved synthetic status and handoff checks do
not prove live multi-session recovery.

### Case-level changes

- Instruction authoring: 0/10 to 10/10 for Codex and 0/10 to 9/10 for Claude.
- Pending review: Codex 0/10 to 10/10. Claude already passed all ten controls.
- Coding and TDD: both harnesses passed all ten treatment trials and their
  controls. TDD evidence includes trusted failing and passing command receipts
  plus an isolated final oracle.
- OpenSpec follow-up: both harnesses passed all ten treatment trials. That
  fixture checks existing requirement identifiers, alias/conflict terms,
  unverified test status, and the absence of writes or delegation. It does not
  fully grade requirement-to-scenario mapping quality. The skill supplies
  conditional OpenSpec guidance; these results do not establish a general
  OpenSpec integration.

### Limits

- All ten Claude pending-review follow-up treatment replies passed task checks
  but failed activation because they omitted `verify-and-finish`.
- Fact checks use declared phrases and alternatives; a missing phrase is not a
  semantic judgment. Private receipts omit response text, so unlisted valid
  paraphrases can remain ambiguous.
- Some case constraints exceed their prompts. The debugging case forbids every
  local write while its prompt prohibits production changes. Its workflow
  failures do not establish unsafe production actions.
- Concurrent execution limits timing comparisons.
- Deterministic validation proves repository mechanics; these live synthetic
  cases provide narrower behavior evidence.

### Trial accounting

The final report combines 1,020 valid original trials with a separately pinned
60-trial Claude follow-up supplement. The original three follow-up studies had
zero valid trials. Their failures remain retained. The original campaign has
57 unique infrastructure failures, excluded from task-quality rates. The
supplement has none. Interrupted infrastructure runs resumed with unchanged
identities; failed task checks were not retried.

Evaluation exposed two Claude transport defects: follow-up prompts arrived
before the prior turn completed, and a completion gate waited for optional
forwarded output. Failures also occurred without the plugin. The repaired
broker passed live same-conversation and delegated-follow-up probes. The
supplement retained the original plugin, runner, prompts, fixtures, and
grading rules. Only its broker changed.

Strict scoring validated all 54 selected studies and their balanced rows. It
validates evidence structure, not benchmark success.

### Identities

Both cohorts use signed plugin/fixture snapshot
`3e5755736d9a44e476649d651a9d6d3fa42f7f27`. SHA-256 identities:

| Artifact | SHA-256 |
|---|---|
| Treatment plugin | `785b9a3d71756818d5ae734af1ca9311a5662127083180ceab3e1fa1a6c76a95` |
| Frozen held-out catalog | `6bda780ff106f674ce8b1c817176cd3eff8509057c14bae8a22b20571e60f7ef` |
| Original broker | `d8157fa4603beb7cfe6c253b0695197cfd9bdf1baf175a43d534cd97527f8cf9` |
| Supplement broker | `934d78f9f6f731375b3c1cfd77b50e094824bc155649a302288d0347f935b555` |

## 2026-10-01 skill-text check

A narrow Claude check measured the October skill-text changes. It used
`claude-opus-5-5` at high effort, Claude Code `2.1.285`, and Subswapper, with
broker SHA-256
`0e44126a47ec56fe461a8b1f0b2cfdb7c32daa898357cb26ae74cb66064b1390`.

Trigger recall, three repetitions per probe, with no gate violations:

| Slice | Pass |
|---|---:|
| `autonomous-run`, including the new pull request probe | 15/15 |
| `writing-agent-instructions`, including the new retrospective probes | 18/18 |
| `no-skill` precision pool | 30/30 |

Installed A/B on four cases, three paired runs each. `v0.32.0` is commit
`38725c7`; the changed skills are commit `dc6e4bc`. Each cell is outcome
passes for control and treatment.

| Case | `v0.32.0` | Changed skills |
|---|---:|---:|
| `code-quality-go-errors` | 3/3, 3/3 | 3/3, 3/3 |
| `systematic-debugging-before-mitigation` | 0/3, 0/3 | 0/3, 0/3 |
| `tdd-add-multiply` | 0/3, 0/3 | 0/3, 0/3 |
| `verify-finish-local-only` | 0/3, 0/3 | 0/3, 0/3 |

Outcomes match exactly, so this check shows neither a regression nor a gain.
Both arms failed three cases at both revisions. In `tdd-add-multiply`, neither
arm wrote a failing test before the implementation, and the treatment never
loaded `test-first-implementation`, so that case cannot measure the changed
test guidance. The 2026-09-05 study passed the same case with
`claude-fable-5-1`; this check did not isolate the model from the CLI version.
Three paired runs per case cannot establish effect size.
