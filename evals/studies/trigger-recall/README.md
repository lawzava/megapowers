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

Run Claude Code and Codex separately. `--filter <substring>` restricts a run
to matching case ids; use it for the pre-merge slice of one edited skill plus
the `no-skill` pool. The publish bundle contains only sanitized
`publish/results.jsonl` and `publish/manifest.json`; prompts, traces, and
private paths are never published.

## Corpus and gates

`cases.json` ships at least three recall probes per skill and at least ten
no-skill probes. Every probe records provenance. `gates.json` starts in
`report-only` mode: violations are printed and recorded in the manifest but do
not fail the run. After the first full baseline on both harnesses, calibrate
per-skill `min_recall` overrides against observed rates and switch `mode` to
`enforce`.

## Oracle mutation check

Before trusting a new baseline, prove the oracle measures trigger text: gut
one skill's frontmatter `description` in a scratch checkout, rerun that
skill's slice, and confirm its recall drops to floor while other slices hold.
Discard the scratch checkout afterward. The credential-free selftest already
proves the event-level oracle rejects wrong, failed, and unexpected
selections.
