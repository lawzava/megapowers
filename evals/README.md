# megapowers evals

The eval stack answers three different questions. Do not promote evidence from
one layer into another.

| Layer | Question | Credentials | Release role |
|---|---|---|---|
| Deterministic regressions | Do manifests, hooks, tools, schemas, and runners work? | No | Required PR gate |
| Installed-plugin A/B | Does this exact plugin revision change target behavior? | Yes | Behavioral release evidence |
| PR replay | Can the installed plugin improve hidden-test correctness on pinned real changes? | Yes | Report-only |

Exact-tag install smoke runs after publication and proves delivery from the
public ref. It is not candidate behavior certification.

## Deterministic regressions

Run the bounded suite:

```bash
bash evals/run-all.sh --json results.jsonl
go run evals/score.go --strict results.jsonl
```

`run-all.sh` executes retained scenarios plus local runner selftests. It emits
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

## Result row contract

Every row records:

- schema, study, evidence class, case, run, block, and arm;
- harness, CLI, model, effort, source revision, and environment;
- prompt, fixture, plugin, and artifact hashes;
- status, process return code, duration, verdict, timestamp, and named metrics.

Use immutable source identities and exact model and effort values. A convenient
alias is not an exact identity. Publish sanitized rows only.

## Installed-plugin A/B

The treatment and control receive identical tasks and fixture bytes in separate
private homes. The treatment installs the current checkout. The control has no
megapowers plugin. User configuration and unrelated plugins enter neither arm.

Credential-free mechanics:

```bash
go run evals/studies/installed-ab/run.go --selftest
bash evals/studies/tests/installed-ab-contract.test.sh
```

Explicit real run:

```bash
go run evals/studies/installed-ab/run.go --run --credentialed \
  --harness codex --model <exact-model> --effort <exact-effort> \
  --sandbox-broker /absolute/path/to/reviewed-broker \
  --broker-sha256 "$BROKER_SHA256" --paired-runs 10 \
  --actor-timeout 20m \
  --out results/installed-ab-codex
```

Run Claude Code and Codex separately. Score the combined publish rows with
`score.go --strict`.

Cases and thresholds live in
[`studies/installed-ab/`](./studies/installed-ab/). Current gates cover prose
fact retention and no-op behavior, code-quality defect reduction without
convention regression, and test-first ordering with an observed red run. The
autonomous resume case is report-only. Release certification requires every
treatment run to pass after the configured number of balanced pairs. Control
outcomes remain required diagnostics; they are not an uplift gate for these
explicit regression tasks.

`--selftest` proves isolation-contract handling, cleanup, fail-closed execution,
result shape, and artifact sanitization with an in-process fake actor. Broker
selftests and the Claude preflight provide the actual OS-boundary evidence. No
selftest produces behavioral evidence.

## PR replay

PR replay starts an actor from a pinned base commit, withholds the historical
patch and hidden oracle files, then scores the actor using the declared
correctness command. The untouched base must fail that oracle.

Credential-free mechanics:

```bash
go run evals/studies/pr-replay/replay.go --selftest
bash evals/studies/tests/pr-replay-contract.test.sh
```

Explicit real run:

```bash
go run evals/studies/pr-replay/replay.go --run --credentialed \
  --harness claude --model <exact-model> --effort <exact-effort> \
  --sandbox-broker /absolute/path/to/reviewed-broker \
  --broker-sha256 "$BROKER_SHA256" \
  --cases private-replays.json --out results/pr-replay
```

The case manifest must use full immutable commit IDs. File overlap with the
historical patch is diagnostic only. Correctness comes from the hidden oracle.
PR replay remains report-only until repeated real runs justify a threshold.

## Artifact policy

Both runners require a reviewed, hash-pinned broker whose attestation proves
credentials, sibling state, and hidden oracle material are outside the actor's
OS boundary. The broker path must be canonical, contain no symlinks, remain
outside actor-visible and output trees, and identify a self-contained executable;
the runner executes a private read-only copy of the pinned bytes. The
credentialed runners publish only:

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
their limitations remain in [RESULTS.md](./RESULTS.md).
