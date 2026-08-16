---
name: independent-review
description: Use when security, auth, billing, concurrency, data integrity, or another high-stakes artifact needs adversarial review by a provider other than its author.
---

# Independent Review

Use model review for residual uncertainty after tests, types, reproductions,
and measurements. A same-provider second look is context separation, not
independence. The lead owns remediation and reruns the real oracle.

## Trusted review path

Review one explicit repository file or an immutable commit range. Inspect the
disclosure before authorizing an external call. First set `review_tool` to the
absolute path of `scripts/megapowers-review.go` beside this `SKILL.md`; use the
skill's installed location, not a project-supplied copy.

```bash
go run "$review_tool" inspect --file path/to/file --provider claude
go run "$review_tool" inspect --base <base-revision> --head <head-revision> --provider codex
```

The inspection prints the resolved provider binary path, binary hash, package
hash, and an `approval_token`. Check them, copy that token, declare the
different artifact author, and approve only that exact package and binary:

```bash
go run "$review_tool" review --file path/to/file \
  --provider claude --author codex --approve-external "$approval_token"

go run "$review_tool" review --base <base-revision> --head <head-revision> \
  --provider codex --author claude --approve-external "$approval_token"
```

The tool refuses an author and provider that do not differ. It does not infer a
dirty worktree, include untracked files, or accept project routing overrides.
It rejects secret-like paths and content, binary data, symlinks, submodules,
and oversized packages; uses fixed Claude and Codex adapters; rejects provider
binaries inside the repository; and forwards only an explicit environment
allowlist.

Review recaptures the input and provider, then constant-time compares the token
before forwarding credentials or source. It copies the approved provider bytes
into a private read-only execution path and executes that copy, so later pathname
replacement cannot change the reviewed binary. Any file or provider-binary change
requires a new inspection. The pre-dispatch disclosure lists paths, byte count,
source identity, and hashes. Pass `--approve-external` only after checking those
fields.
Receipts are private and advisory, not an approval gate or tamper-proof
attestation. The transcript is not retained by default; use
`--retain-transcript` only with a deliberate sensitive-data decision. Treat the
review verdict as a claim, fix credible findings as the single writer, and
rerun the artifact's acceptance tests after every material change.

The default receipt stays under private Git metadata. An explicit `--out` must
already exist as an absolute canonical directory outside the repository and may
not contain it. Receipt writes stay anchored to an opened directory handle.
