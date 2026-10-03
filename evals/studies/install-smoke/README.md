# Exact-tag install smoke

This post-publish check installs exactly one `megapowers` plugin into fresh,
unauthenticated Claude Code and Codex homes. It verifies registration JSON,
harness-reported cache paths, both cached manifests and versions, and the exact
installed bytes of `evidence-research`. It needs no credentials, invokes no
model, and proves delivery only, not behavioral quality.

Runner check, no CLI required:

```bash
bash evals/studies/install-smoke/run-smoke.sh --selftest
```

Local diagnostic mode, which tolerates a missing CLI only when the other
harness passes:

```bash
evals/studies/install-smoke/run-smoke.sh \
  --out "${TMPDIR:-/tmp}/megapowers-install-smoke" \
  --harnesses claude,codex
```

After an authorized tag and publication, fetch and prove the exact public ref:

```bash
tag=vX.Y.Z
evals/studies/install-smoke/run-smoke.sh \
  --out "${TMPDIR:-/tmp}/megapowers-install-smoke-$tag" \
  --source lawzava/megapowers --ref "$tag" --version "${tag#v}" \
  --harnesses claude,codex
```

Exact-ref mode fails on every skip, verifies the fetched tag points at `HEAD`,
and checks that the source and cached manifests carry the requested version.
