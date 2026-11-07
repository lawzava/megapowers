# OpenCode native event fixtures

Captured on 2026-11-07 with the real OpenCode v2.2.6 binary and this runner's
approved standalone environment. These are real `tool_use` records, not
reconstructed event shapes. Session, message, part and provider-call identifiers
are replaced with `<redacted-id>`. Disposable project paths use `<project>`;
the public plugin checkout uses `<checkout>`. Skill output and its matching
content text are omitted. Object keys, types, tool names, inputs and edit
metadata retain their captured shape.

Sources, with a different fresh `--out` directory for each command:

```bash
go run evals/studies/installed-ab/run.go --opencode-probe activation --credentialed --model claude-agent/opus --actor-timeout 3m --out "$TMPDIR/opencode-correction-opus-activation"
go run evals/studies/installed-ab/run.go --opencode-probe activation --credentialed --model 'openai/gpt-6.1-sol-fast#high' --actor-timeout 3m --out "$TMPDIR/opencode-correction-sol-activation"
go run evals/studies/installed-ab/run.go --opencode-probe edit --credentialed --model claude-agent/opus --actor-timeout 3m --out "$TMPDIR/opencode-correction-opus-edit"
go run evals/studies/installed-ab/run.go --opencode-probe edit --credentialed --model 'openai/gpt-6.1-sol-fast#high' --actor-timeout 3m --out "$TMPDIR/opencode-correction-sol-edit"
```

Both activation probes emitted `skill` with `state.input.id: safe-effects`,
`state.status: completed`, and nested `state.metadata.metadata.name`.
Both replied `DONE`. The Opus edit used `write` with `state.input.path`.
The Sol edit used `patch` with `state.metadata.metadata.files[].file` and
two operations on `probe.txt` (delete and add). Both files ended as `after\n`.

Top-level `state.metadata.files[].filePath` coverage is a separate synthetic
compatibility test. That shape was not emitted by these four probes.
