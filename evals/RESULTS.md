# Megapowers evaluation evidence

Source of record for current models: the 2026-10-09 installed-plugin A/B on
Claude Opus 5.5 and GPT-6.1 Sol. The 2026-09-05 study below measured earlier
models with graders that are now known to fail correct answers. Reproduction
commands, case filters, resume rules, and metric definitions are in
[Installed-plugin A/B](./studies/installed-ab/README.md). Earlier measurements
are in [RESULTS-archive.md](./RESULTS-archive.md); they used different graders
and cannot serve as a direct before/after comparison.

## 2026-10-09 installed-plugin A/B on Opus 5.5 and Sol 6.1

On these models the plugin changes few outcomes. Both arms finish the coding
tasks equally; the measured differences are in a handful of judgment cases,
mostly in the plugin's favor.

### Setup

- 30 cases, five control/treatment pairs per case per harness: 600 runs plus
  a 20-run rerun of `safe-effects-broad-approval-identity` after a fixture fix.
- Claude Code 2.1.295 with `claude-opus-5-5`; Codex 0.162.0 with
  `gpt-6.1-sol`; high effort; both through Subswapper; plugin at `c4c8941`
  skill text with the hook and review changes that followed it.
- Fact checks re-graded by the [fact judge](./tools/fact-judge/main.go) with
  `claude-opus-5-5` as a blind judge; other cases use the runner's outcome.
  The judge's outcome rule matched the runner on every row whose literal facts
  pass (`--check`, 0 mismatches). All result files pass `evals/score.go
  --strict`. `orchestration-output-only-evidence` is report-only and excluded.

### Results

| Harness | Treatment | Control |
|---|---:|---:|
| Claude Opus 5.5 | 138/145 | 130/145 |
| Codex GPT-6.1 Sol | 125/145 | 120/145 |

Cases where the arms differ (treatment vs control, out of 5):

| Case | Opus 5.5 | Sol 6.1 |
|---|---|---|
| `humanizing-prose-pr-summary` | 5 vs 0 | 5 vs 5 |
| `design-plan-ambiguous-contract` | 5 vs 5 | 5 vs 0 |
| `evidence-research-contested-rationale` | 4 vs 2 | 3 vs 3 |
| `humanizing-prose-review-comment` | 5 vs 5 | 5 vs 3 |
| `safe-effects-broad-approval-identity` | 5 vs 4 | 5 vs 4 |
| `verify-finish-commit-gate` | 5 vs 5 | 5 vs 4 |
| `independent-review-approval-boundary` | 5 vs 4 | 0 vs 0 |
| `humanizing-prose-thread-reply` | 4 vs 5 | 5 vs 5 |
| `autonomous-run-resume-status` | 5 vs 5 | 4 vs 5 |
| `systematic-debugging-before-mitigation` | 5 vs 5 | 1 vs 2 |
| `verify-finish-local-only` | 5 vs 5 | 3 vs 5 |

The other 18 cases tie. `author-instructions-smallest-scope` fails in every
run on both harnesses: no answer names the helper language the task asks
for. On Sol, `verify-finish-local-only` treatment answers lead with
`VERIFIED:` and two of five omit that deployment is unverified, a fact the
case requires and the task wording does not.

### What changed in the evaluation

Most cases that never passed were grader defects, not model failures:
phrase matching rejected paraphrases, negations, and quoted mentions; red test
runs did not bind because they omitted the oracle's `-tags`; Codex commands
ran with no `PATH` or `HOME` and were hidden behind the receipt shell wrapper;
`external_write` never fired; one case forbade a local draft and another
required delegating a 112-byte read. The 2026-10-09 rows are not comparable
with the 2026-09-05 rows.

Open: three Claude treatment arms ended with broker exit 125 after a
successful result; each retry passed and the cause is not isolated.

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
can reject an extra successful skill read.

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
