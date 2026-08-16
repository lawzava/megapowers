# Independent review

Independent review is an explicit external disclosure path for residual risk in
security, authentication, billing, concurrency, data integrity, and similar
work. It supplements tests and measurements. It does not replace them.

## Requirements

- Run inside the Git repository that owns the artifact.
- Go 1.24 or newer available for `go run` and rooted receipt writes.
- Claude Code or Codex installed and authenticated as the reviewer provider.
- A reviewer provider different from the artifact author.
- Human approval for the disclosed package.

Resolve the tool from the installed skill, not from the project being reviewed:

```bash
review_tool=/absolute/path/to/installed/independent-review/scripts/megapowers-review.go
```

## Inspect first

Review one repository file:

```bash
go run "$review_tool" inspect --file path/to/file --provider claude
```

Or review an immutable commit range:

```bash
go run "$review_tool" inspect --base <base-revision> --head <head-revision> \
  --provider codex
```

The disclosure shows source identity, paths, bytes, package hash, resolved
provider binary path, binary hash, and an `approval_token`. Stop if a path,
byte count, provider, or binary is not the one intended. Copy the token only
after reviewing those fields:

```bash
approval_token='<approval_token from the inspected JSON>'
```

## Approve and dispatch

File written by Codex, reviewed by Claude:

```bash
go run "$review_tool" review --file path/to/file \
  --provider claude --author codex \
  --approve-external "$approval_token"
```

Commit range written by Claude, reviewed by Codex:

```bash
go run "$review_tool" review --base <base-revision> --head <head-revision> \
  --provider codex --author claude \
  --approve-external "$approval_token"
```

The tool refuses matching author and reviewer providers. It also refuses an
implicit dirty tree, untracked files, project-selected binaries, symlinks,
submodules, binary data, oversized packages, secret-like paths, and common
secret patterns. Provider processes receive a small environment allowlist.
Review recaptures the source and resolves the provider again before dispatch.
Any package or provider-binary change invalidates the token and requires a new
inspection. The approved provider bytes are then copied into a private read-only
execution path, so later pathname replacement cannot change what runs.

## Receipts and transcripts

Receipts are private advisory records. They bind the source identity, package
hash, provider, author, and command outcome. They are not tamper-proof
attestations and do not gate a release by themselves.

Raw prompts and provider output are not retained by default. Use
`--retain-transcript` only after deciding that storing the disclosed content and
review response is acceptable. An explicit `--out` must already exist as an
absolute, canonical directory outside the repository and may not contain it.
Writes are anchored to an opened directory handle. Otherwise the default stays
under private Git metadata.

## Interpret the result

A model verdict is a claim. The lead evaluates each finding, makes any change as
the single writer, and reruns the artifact's acceptance tests. If the provider
is unavailable or the disclosure cannot be approved, report that independent
review did not run.
