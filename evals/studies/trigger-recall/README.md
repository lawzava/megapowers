# Trigger recall

This study measures skill activation, not task quality. Every run installs the
current checkout and executes a fixed probe corpus. The verdict for each probe
comes only from trace-proven `skill_selected` events:

- **Recall probes** (`verbatim`, `paraphrase`, `buried`) pass when the expected
  skill activates successfully.
- **Precision probes** (`near-miss`, `no-skill`) fail when any shipped skill
  outside the probe's `expected` and `allowed` sets is selected, counting
  failed attempts. Selections of non-megapowers skills never affect a verdict.

Rows carry `evidence_class: "activation"` and a single `treatment` arm. The
strict scorer validates them separately from behavioral treatment/control
evidence and reports per-case pass rates. Activation evidence never becomes an
efficacy claim; it answers one question: does the shipped trigger surface bind
on this harness and model?

## Purpose

The 2026-08-30 installed A/B run (see `evals/RESULTS.md`) could not separate
activation failures from capability failures. This corpus is the regression
oracle for skill-text edits: run the affected slice before merging any change
to a `SKILL.md` description or trigger surface.

## Credential-free mechanics

```bash
go run evals/studies/trigger-recall/run.go --selftest
go run evals/studies/trigger-recall/run.go --validate-config \
  --cases evals/studies/trigger-recall/cases.json \
  --gates evals/studies/trigger-recall/gates.json
```

`--selftest` proves oracle mechanics, fail-closed execution, publish
sanitization, and gate assessment with an in-process scripted actor. It is not
behavioral evidence.

## Explicit real run

Real actors run only through the same user-reviewed, hash-pinned isolation
broker as installed A/B (schema `2`). Direct harness execution is
intentionally unsupported.

```bash
go run evals/studies/trigger-recall/run.go --run --credentialed \
  --harness claude --model <exact-model> --effort <exact-effort> \
  --sandbox-broker /absolute/path/to/reviewed-broker \
  --broker-sha256 "$BROKER_SHA256" --reps 3 \
  --out results/trigger-recall-claude
```

Each probe absorbs one automatic retry for a transient actor failure (fresh
disposable directories per attempt; retry counts appear in the manifest). A
second failure fails the run closed and writes the attempt's trace and error
to `<out>/failures/` — private maintainer diagnostics, never part of the
publish bundle.

A broker `skills_catalog` attestation with `rendered: false` is the same
class of failure: the harness never presented the Megapowers catalog, so the
probe cannot measure recall. The attempt is retried once and a second miss
fails the run closed naming the harness. The manifest records
`catalog_rendered` (`true` when every probe was attested, `null` when the
harness path reports no signal) and `catalog_source`.

## Response length

Every row carries `final_words` (whitespace-separated words in the final
response after fenced code blocks are removed) and `final_em_dashes` (count
of U+2014). The manifest reports `median_precision_final_words` over
`no-skill` and `near-miss` probes and `em_dash_rate`, the fraction of all
finals containing an em dash. `gates.json` may set
`acceptance.max_median_final_words` and `acceptance.max_em_dash_rate`; absent
values are report-only, present values become violations enforced for the
harnesses in `enforce_harnesses`. The shipped gates set 120 words and 0.1
for Claude.

Run Claude Code and Codex separately. `--filter <substring>` restricts a run
to matching case ids; use it for the pre-merge slice of one edited skill plus
the `no-skill` pool. The publish bundle contains only sanitized
`publish/results.jsonl` and `publish/manifest.json`; prompts, traces, and
private paths are never published.

## Corpus and gates

`cases.json` ships at least three recall probes per model-selectable skill
and at least ten no-skill probes. Skills with
`disable-model-invocation: true` in Claude Code, or
`policy.allow_implicit_invocation: false` in Codex's `agents/openai.yaml`,
are exempt from implicit recall. The runner reads each harness's native policy.
`memory-hygiene` has separate explicit-invocation and implicit non-selection
probes. Every probe records provenance. `gates.json` enforces for the
harnesses in `enforce_harnesses` (currently Claude only) and records
violations without failing elsewhere. The 2026-09-02 Codex run reported
0/117 implicit recall despite a rendered catalog. Interactive sessions did
load skills. Probe shape, hook trust, and multi-turn context are hypotheses
for the difference, not established causes. Codex stays report-only until
controlled comparisons explain the gap.

One calibrated boundary is accepted, not a defect: `safe-effects-near-miss`
never gates (`per_case.max_false_selection_rate: 1`). Once the broker write
fix let actors actually perform the probe's file overwrite, the model
consistently consulted the skill first, which is defensible caution kept as
signal only. The former `verify-and-finish` exception (`min_recall: 0.3`) was
retired after the 2026-09-02 run measured 9/9 with revised trigger text.
Recalibrate against `evals/RESULTS.md` when skill text or models change.

## Oracle mutation check

Before trusting a new baseline, prove the oracle measures trigger text: gut
one skill's frontmatter `description` in a scratch checkout, rerun that
skill's slice, and confirm its recall drops to floor while other slices hold.
Discard the scratch checkout afterward. The credential-free selftest already
proves the event-level oracle rejects wrong, failed, and unexpected
selections.
