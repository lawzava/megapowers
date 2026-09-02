# Independent review

Independent review is an explicit external disclosure path for residual risk in
security, authentication, billing, concurrency, data integrity, and similar
work. It supplements tests and measurements. It does not replace them.

The tool is vendor-neutral. It runs whatever reviewer command the operator
names, verifies that binary, and binds approval to the exact package, binary,
and command. megapowers ships no provider list, model choice, or credential
handling; those live in the operator's registry and instructions.

## Requirements

- Run inside the Git repository that owns the artifact.
- Go 1.25 or newer available for `go run` and rooted receipt writes.
- A reviewer CLI installed and authenticated, from a vendor family different
  from the artifact author's.
- Human approval for the disclosed package.

Resolve the tool from the installed skill, not from the project being reviewed:

```bash
review_tool=/absolute/path/to/installed/independent-review/scripts/megapowers-review.go
```

## Name the reviewer

`--provider` is an opaque family label such as `anthropic`, `openai`, or
`zai`. `--provider-command` is the reviewer command as one quoted string; it is
split into arguments without a shell. `{prompt_file}` expands to a private
prompt file and `{scratch_dir}` to the private scratch directory; without
`{prompt_file}` the prompt arrives on stdin. Credentials pass only through
`--provider-env NAME`. Example shapes, to be replaced by whatever the operator
runs:

```bash
reviewer_cmd='claude -p --safe-mode --no-session-persistence --permission-mode plan --tools ""'
reviewer_cmd='codex exec --ephemeral --ignore-user-config --skip-git-repo-check -C {scratch_dir} --sandbox read-only -'
reviewer_cmd='<reviewer-cli> --model <model-id> --prompt-file {prompt_file}'
```

## Inspect first

Review one repository file:

```bash
go run "$review_tool" inspect --file path/to/file --provider <reviewer-family> \
  --provider-command "$reviewer_cmd"
```

Or review an immutable commit range:

```bash
go run "$review_tool" inspect --base <base-revision> --head <head-revision> \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
```

The disclosure shows source identity, per-chunk paths, bytes, and package
hashes, the resolved binary path and hash, the command, and an
`approval_token`. Stop if a path, byte count, provider, binary, or command is
not the one intended. Copy the token only after reviewing those fields:

```bash
approval_token='<approval_token from the inspected JSON>'
```

## Approve and dispatch

```bash
go run "$review_tool" review --file path/to/file \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external "$approval_token"
```

The tool refuses matching author and reviewer labels. It also refuses
project-selected binaries, shell metacharacters in the command, symlinks,
submodules, binary data, oversized packages, secret-like paths, and common
secret patterns. Commit ranges use immutable revisions and exclude unrelated
worktree changes. Reviewer processes receive a small environment allowlist plus
the named `--provider-env` variables.
Review recaptures the source and resolves the reviewer again before dispatch.
Any package, binary, or command change invalidates the token and requires a new
inspection. The approved binary bytes are copied into a private read-only
execution path, so later pathname replacement cannot change what runs.

## Receipts and transcripts

Receipts are private advisory records. They bind the source identity, package
hash, provider label, command, author, and command outcome. A range above one
package splits into up to 16 directory-grouped chunks under one approval
token; an index `receipt.json` lists one `chunk-NN/receipt.json` per dispatched
package. A preflight probe fails fast, before any artifact bytes leave the
machine, when the reviewer reports a login or usage limit or stalls past
`--preflight-timeout`. Receipts are not tamper-proof attestations and do not
gate a release by themselves.

Raw prompts and reviewer output are not retained by default. Use
`--retain-transcript` only after deciding that storing the disclosed content and
review response is acceptable. An explicit `--out` must already exist as an
absolute, canonical directory outside the repository and may not contain it.
Writes are anchored to an opened directory handle. Otherwise the default stays
under private Git metadata.

## Interpret the result

A model verdict is a claim. The lead evaluates each finding, makes any change as
the single writer, and reruns the artifact's acceptance tests. If the reviewer
is unavailable or the disclosure cannot be approved, report that independent
review did not run.
