# Session observability study

This maintainer-only diagnostic finds repeated workflow gaps without publishing
session content. It does not discover files, read session stores, or run an
agent. Maintainers must normalize private logs into explicit typed JSONL inputs.

Each `--root-group` file represents one independent source root. A line has one
supported `type`: `session`, `turn`, `tool_call`, `agent_spawn`,
`agent_complete`, `wait`, `handoff`, `resume`, `compaction`, `error`, or
`external_effect_attempt`. A `session` line also needs `status` set to
`completed`, `interrupted`, or `abandoned`. Other event fields may remain in a
private input, but the runner ignores them.

Each root needs two strict JSON sidecars in the same argument order:

```json
{"schema_version":"1","type":"effect_gate","decisions":{"allow":0,"deny":0}}
```

```json
{"schema_version":"1","type":"oracle","results":{"pass":1,"fail":0,"indeterminate":0}}
```

Run the study with at least two distinct roots:

```bash
go run evals/studies/session-observability/run.go \
  --root-group /private/normalized-root-a.jsonl \
  --effect-gate /private/root-a-effect-gate.json \
  --oracle /private/root-a-oracle.json \
  --root-group /private/normalized-root-b.jsonl \
  --effect-gate /private/root-b-effect-gate.json \
  --oracle /private/root-b-oracle.json \
  --out /private/aggregate.json
```

The output contains typed aggregate counts only. It excludes event bodies,
paths, identifiers, timestamps, and per-root rows. An atomic event pattern
becomes a candidate only when it appears in at least two independent roots.
The runner does not infer causal, temporal, or identity relationships from
aggregate counts. This threshold supports investigation. It does not prove
causation or justify a new skill.

The runner groups evidence at file level. Poor normalization or correlated
roots can create false patterns. Review the private sources before changing
shipped guidance. Do not publish the normalized inputs or sidecars.

Credential-free checks:

```bash
go run evals/studies/session-observability/run.go --selftest
bash evals/studies/tests/session-observability-contract.test.sh
```
