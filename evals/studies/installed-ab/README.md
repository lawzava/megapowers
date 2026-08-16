# Installed-plugin A/B

This study compares a current-checkout `megapowers` installation with an empty
control under Claude Code and Codex. Each arm receives the same task and fixture
bytes in a separate disposable work directory. No user plugins, guidance,
configuration, or credentials enter either arm. Every disposable directory is
removed on success and failure.

Credential-free validation:

```bash
go run evals/studies/installed-ab/run.go --selftest
bash evals/studies/tests/installed-ab-contract.test.sh
```

Real runs are always explicit. They never fall back to a fake actor:

```bash
go run evals/studies/installed-ab/run.go --run --credentialed \
  --harness codex --model gpt-5.6-sol --effort high \
  --sandbox-broker /usr/local/libexec/megapowers-eval-broker \
  --broker-sha256 "$BROKER_SHA256" --paired-runs 10 \
  --out results/installed-ab
```

Real actors run only through a user-reviewed, hash-pinned isolation broker. The
broker owns provider authentication outside the actor-visible filesystem and
receives its request on standard input. It must attest a real OS boundary and
prove the actor can read only its current project plus `plugins/megapowers` in
the treatment arm and can write only the current project. Any credential
access, sibling-arm access, extra read or write root,
missing attestation, or broker hash mismatch fails closed. Direct Claude or
Codex execution is intentionally unsupported.

The broker reads one versioned JSON request from standard input and returns one
versioned JSON object. Its response supplies CLI version, result, trace-derived
events, exact plugin inventory, exit code, duration, and an isolation
attestation. The attestation explicitly sets both credential and sibling-state
readability to false and repeats the exact task read and write roots. Omitted
fields, extra JSON, unrecognized boundaries, or inventory mismatches fail.

The output contains only `publish/results.jsonl` and
`publish/manifest.json`. Raw responses, transcripts, prompts, config homes,
credentials, and absolute paths are never published. `--selftest` proves runner
mechanics only. It is not behavioral evidence or release certification.

Prose gates require all seeded facts, zero seeded inventions, and exact no-op
behavior for text that is already direct, ignoring trailing whitespace only.
Code-quality
gates require passing task tests, fewer seeded defects, and no repository
convention regression. The TDD case requires a test edit and observed red test
run before the implementation edit. Aggregate release decisions additionally
use the paired-run, absolute-lift, and confidence-lower-bound thresholds in
`gates.json`. The manifest records the per-case calculation and a `publishable`
verdict. Too few pairs, weak lift, or uncertain lift writes the sanitized report
but returns failure. A single passing row cannot certify the plugin. Every arm
records the observed plugin inventory and its hash; the control row uses the
scorer's canonical empty-plugin hash.

The autonomous-run resumption case records whether the actor reads durable
status, resumes the current task, and preserves completed work. It is explicitly
report-only until repeated real runs establish a useful release threshold.
