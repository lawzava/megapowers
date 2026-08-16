# Evaluation and release evidence

megapowers keeps mechanics, behavior, and real-project correctness separate.
This prevents a passing selftest from becoming an unsupported quality claim.

## Deterministic regressions

```bash
bash evals/run-all.sh --json results.jsonl
go run evals/score.go --strict results.jsonl
```

These runs need no model credentials. They validate schemas, runners, hooks,
tools, and retained regression scenarios. Strict scoring rejects empty,
malformed, duplicated, incomparable, incomplete, indeterminate, timed-out, or
harness-error data.

Passing deterministic checks proves repository mechanics only.

## Installed-plugin A/B

Credential-free runner tests:

```bash
go run evals/studies/installed-ab/run.go --selftest
bash evals/studies/tests/installed-ab-contract.test.sh
```

A real run is explicit. Both arms receive identical tasks through a reviewed,
hash-pinned broker that keeps credentials and sibling state outside the actor's
OS isolation boundary. Supply its canonical non-symlink path. The runner copies
the pinned bytes to a private read-only execution path, so the broker must be a
self-contained executable:

```bash
go run evals/studies/installed-ab/run.go --run --credentialed \
  --harness codex --model <exact-model> --effort <exact-effort> \
  --sandbox-broker /absolute/path/to/reviewed-broker \
  --broker-sha256 "$BROKER_SHA256" --paired-runs 10 \
  --actor-timeout 20m \
  --out results/installed-ab-codex
```

Run Claude Code and Codex separately, then score the published JSONL with the
strict scorer. The treatment installs the current checkout. The control stays
empty. Task, fixture, source, plugin, harness, model, effort, and environment
identities travel with each row.

Current release gates include:

- prose retains every seeded fact, invents none, and leaves already-direct text
  unchanged;
- code quality reduces seeded defects without task or repository-convention
  regression;
- test-first work edits and executes a failing test before implementation;
- every treatment row passes after the balanced run count in `gates.json`.

Control rows remain mandatory paired diagnostics. Their results and exact
McNemar comparison are reported, but do not gate explicit tasks that already
tell both arms what correct behavior is. This certifies treatment reliability,
not a general claim that the plugin improves model capability.

The autonomous resume case remains report-only until repeated real runs justify
a threshold. A selftest never produces release certification.

## PR replay

Credential-free contracts:

```bash
go run evals/studies/pr-replay/replay.go --selftest
bash evals/studies/tests/pr-replay-contract.test.sh
```

Real replay requires a reviewed private case manifest with immutable base and
head commits, original task text, hidden oracle files, and a correctness command.
The actor sees the base tree but not the gold change or hidden tests.

```bash
go run evals/studies/pr-replay/replay.go --run --credentialed \
  --harness claude --model <exact-model> --effort <exact-effort> \
  --sandbox-broker /absolute/path/to/reviewed-broker \
  --broker-sha256 "$BROKER_SHA256" \
  --cases private-replays.json --out results/pr-replay
```

The broker must expose only the exported base project and installed plugin, not
the source mirror, head commit, oracle overlay, sibling state, or credentials.
The untouched base must fail the oracle. Actor work passes only when the hidden
correctness oracle passes. Historical file overlap is diagnostic and never
changes the verdict. PR replay is report-only.

## Publishable artifacts

The installed A/B and PR replay runners publish only `results.jsonl` and a
sanitized `manifest.json`. Do not publish raw homes, repositories, prompts,
responses, transcripts, credentials, or absolute paths.

## Release order

1. Set the release version, then freeze and commit the candidate revision.
2. Run deterministic validation.
3. Run source-bound installed A/B for Claude Code and Codex when behavioral
   guidance changed.
4. Review the certificate, then tag and publish the same committed revision.
5. Run exact-tag fresh-install smoke against the public tag.

The post-publish smoke proves delivery from the public ref. It cannot rescue a
candidate that was not certified before publication.
