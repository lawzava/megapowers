---
name: independent-review
description: Use when security, auth, billing, concurrency, data integrity, or another high-stakes artifact needs adversarial review by a provider other than its author.
---

# Independent Review

Use model review for residual uncertainty after tests, types, reproductions,
and measurements. State the artifact intent and acceptance boundary first. A
same-provider second look is context separation, not independence. The lead
owns remediation and reruns the real oracle.

## Trusted review path

Review a file or immutable commit range. Inspect the disclosure
before an external call. Set `review_tool` to the absolute installed
`scripts/megapowers-review.go` beside this `SKILL.md`, not a project copy.

```bash
go run "$review_tool" inspect --file path/to/file --provider claude
go run "$review_tool" inspect --base <base-revision> --head <head-revision> --provider codex
```

The inspection prints the provider path, binary hash, package hash, and an
`approval_token`. Check them, copy that token, declare the
different artifact author, and approve only that exact package and binary:

```bash
go run "$review_tool" review --file path/to/file \
  --provider claude --author codex --approve-external "$approval_token"

go run "$review_tool" review --base <base-revision> --head <head-revision> \
  --provider codex --author claude --approve-external "$approval_token"
```

The tool refuses an author and provider that do not differ. It does not infer a
dirty worktree, include untracked files, or accept routing overrides.
It rejects secret-like paths and content, binary data, symlinks, submodules,
and oversized packages; uses fixed Claude and Codex adapters; rejects provider
binaries inside the repository; and forwards only an explicit environment
allowlist.

Review recaptures input and provider, then constant-time compares the token
before forwarding credentials or source. It copies the approved provider bytes
into a private read-only execution path and executes that copy, so later pathname
replacement cannot change the reviewed binary. Any file or provider-binary change
requires a new inspection. The pre-dispatch disclosure lists paths, byte count,
source identity, and hashes. Pass `--approve-external` only after checking those
fields.
Receipts are private and advisory, not an approval gate or tamper-proof
attestation. The transcript is not retained by default. Use
`--retain-transcript` only after a sensitive-data decision. Treat the
review verdict as a claim. Record credible findings, and explain dismissed
findings against the artifact intent. Fix as the single writer, then rerun the
artifact's acceptance tests after every material change.

The default receipt stays under private Git metadata. An explicit `--out` must
already exist as an absolute canonical directory outside the repository and may
not contain it. Receipt writes stay anchored to an opened directory handle.
