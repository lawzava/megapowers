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
  --actor-timeout 20m \
  --out results/installed-ab
```

Real actors run only through a user-reviewed, hash-pinned isolation broker. The
broker owns provider authentication outside the actor-visible filesystem and
receives its request on standard input. It must attest a real OS boundary and
prove the actor can read only its current project plus `plugins/megapowers` in
the treatment arm and can write only the current project. Any credential
access, sibling-arm access, extra read or write root,
missing attestation, or broker hash mismatch fails closed. Direct Claude or
Codex execution is intentionally unsupported. Each actor also has a bounded
deadline; a timeout is recorded as an infrastructure failure and stops the run.

The broker reads one versioned JSON request from standard input and returns one
versioned JSON object. The request includes a bounded `timeout_ms`; the broker
must apply it to the harness process tree before the runner's outer deadline.
Its response supplies CLI version, result, trace-derived
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
run before the implementation edit. Release certification requires the balanced
run count in `gates.json` and every treatment run to pass its public oracle.
Control results remain mandatory diagnostic evidence, but they do not impose an
artificial uplift gate on tasks whose instructions already state the expected
behavior. The scorecard reports the paired comparison with an exact McNemar
test; it does not turn this small regression suite into an efficacy claim. Too
few pairs or any treatment failure writes the sanitized report but returns
failure. Every arm records the observed plugin inventory and its hash; the
control row uses the scorer's canonical empty-plugin hash. The publish manifest
also binds the canonical full case catalog and gate definitions, including
oracles and grading rules, to prevent stale or weakened certificates.

The autonomous-run resumption case records whether the actor reads durable
status, resumes the current task, and preserves completed work. It is explicitly
report-only until repeated real runs establish a useful release threshold.
