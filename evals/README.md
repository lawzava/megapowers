# megapowers evals

The eval stack answers four different questions. Do not promote evidence from
one layer into another.

| Layer | Question | Credentials | Release role |
|---|---|---|---|
| Deterministic regressions | Do manifests, hooks, tools, schemas, and runners work? | No | Required PR gate |
| Trigger recall | Does the shipped trigger surface activate the right skill, and only it? | Yes | Enforced for Claude; report-only for Codex |
| Installed-plugin A/B | Does this exact plugin revision change target behavior? | Yes | Optional diagnostic evidence |
| PR replay | Can the installed plugin improve hidden-test correctness on pinned real changes? | Yes | Report-only |

Exact-tag install smoke runs after publication and proves delivery from the
public ref. It is a delivery check, not behavioral evidence.

The maintainer-only session observability study accepts explicit normalized
files and emits content-minimized aggregate diagnostics. It is neither a shipped
skill nor release evidence. See
[`studies/session-observability/`](./studies/session-observability/).

## Deterministic regressions

Run the bounded suite:

```bash
bash evals/run-all.sh --json results.jsonl
go run evals/score.go --strict results.jsonl
```

`run-all.sh` executes local runner selftests. It emits
schema-versioned regression rows and persists them even when a check fails.
Each child has a timeout.

Strict scoring fails closed on:

- empty or malformed input;
- unknown fields or verdicts;
- duplicate run identities;
- incomplete treatment and control blocks;
- mixed source, prompt, fixture, plugin, harness, model, effort, or environment
  provenance inside a comparison;
- indeterminate, timed-out, or harness-error rows.

Regression rows never contribute to behavioral effect estimates.

Activation rows (`evidence_class: "activation"`) are single-arm skill-trigger
measurements from the trigger-recall study. Strict scoring additionally
requires the `treatment` arm, an installed-plugin hash, a binary
`activation_success` metric matching the verdict, unique rep blocks, and
balanced rep counts across the cases of one run. Activation evidence never
enters treatment/control comparisons or effect estimates.

## Result row contract

Every row records:

- schema, study, evidence class, case, run, block, and arm;
- harness, CLI, model, effort, source revision, and environment;
- prompt, fixture, plugin, and artifact hashes;
- status, process return code, duration, verdict, timestamp, and named metrics.

Use immutable source identities and exact model and effort values. A convenient
alias is not an exact identity. Publish sanitized rows only.

## Trigger recall

Trigger recall measures whether each shipped skill activates on prompts that
should select it and stays quiet on prompts that should not. It is the
regression oracle for skill-text edits: run the affected slice before merging
a `SKILL.md` description change. The current gate applies to Claude. Codex
remains report-only. The
[`trigger-recall` study](./studies/trigger-recall/README.md) owns its commands,
corpus rules, gates, publish boundary, and oracle mutation check.

## Installed-plugin A/B

The treatment and control receive identical tasks and fixture bytes in separate
private homes. The treatment installs the current checkout. The control remains
empty. This optional study measures treatment reliability and paired control
outcomes; it does not gate releases or establish general model improvement. The
[`installed-plugin A/B` study](./studies/installed-ab/README.md) owns its
commands, cases, thresholds, resume rules, broker contract, and publish boundary.

## PR replay

PR replay starts an actor from a pinned base commit, withholds the historical
patch and hidden oracle files, then uses the declared correctness command.
Credentialed execution stays disabled until the runner adopts broker schema
`2`. PR replay remains report-only. The
[`PR replay` study](./studies/pr-replay/README.md) owns its command, case schema,
isolation contract, and verdict rules.

## Artifact policy

Credentialed studies require a reviewed, hash-pinned broker. The repository's
[`sandbox broker`](tools/sandbox-broker/README.md) implements schema `2` for
installed A/B and trigger recall on Linux. PR replay still requires migration
from schema `1`.

Share only each study's sanitized bundle:

```text
publish/manifest.json
publish/results.jsonl
```

Do not publish raw config homes, repositories, prompts, responses, transcripts,
credentials, or absolute paths. Inspect the publish bundle before sharing it.

## Add or change an eval

1. Define the behavior and failure before changing guidance.
2. Prefer a deterministic correctness oracle over output-shape heuristics.
3. Mutation-test the oracle with a deliberately wrong artifact.
4. Keep treatment and control inputs identical except for plugin installation.
5. Pin source and environment identities.
6. Treat infrastructure failures as failures, never missing data.
7. Report null and negative results.

Detailed release sequencing is in
[docs/advanced/evals.md](../docs/advanced/evals.md). Historical measurements and
their limitations remain in [RESULTS-archive.md](./RESULTS-archive.md).
