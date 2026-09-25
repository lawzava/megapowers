---
name: independent-review
description: Use when security, auth, billing, concurrency, data integrity, or another high-stakes artifact needs adversarial review by a provider other than its author.
when_to_use: "Trigger phrases: get a second opinion from another model, external review, have another vendor review this, adversarial review of auth, billing, or migration code before merge."
metadata:
  short-description: Cross-provider adversarial review of a high-stakes artifact
---

# Independent Review

Use model review for residual uncertainty after executable checks. State artifact
intent and acceptance boundary. Same-provider review gives only context
separation, not independence. The lead owns remediation and reruns the oracle.

## Trusted review path

Take the reviewer family and exact command from the capability registry or
user instruction; never guess. Inspect a file or immutable commit range before
any external call. Set `review_tool` to the installed
`scripts/megapowers-review.go` beside this `SKILL.md`, not a project copy.

```bash
reviewer_cmd='<reviewer CLI and arguments>'   # {prompt_file} and {scratch_dir} expand
go run "$review_tool" inspect --file path/to/file \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
go run "$review_tool" inspect --base <base-revision> --head <head-revision> \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
```

The inspection prints the binary hash, command, chunk package hashes, and one
`approval_token`. Approve only that exact package, binary, and command:

```bash
go run "$review_tool" review --file path/to/file \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external "$approval_token"
```

Author and provider labels must differ and name real vendor families. The
prompt arrives on stdin unless the command names `{prompt_file}`. Credentials
pass only through `--provider-env NAME`. A preflight probe fails fast on login,
usage-limit, or stall before any artifact bytes leave the machine. Any file,
binary, or command change requires a new inspection and token.

## Declare the payload once

External dispatch sends artifact bytes to another vendor. Before sending, write
one declaration: the file list or immutable commit range, total bytes, and
destination provider. Ask approval with that same declaration and dispatch
that identical scope, so any harness approval reviewer sees one payload. A review mandate alone does not authorize disclosure. If the
artifact changes after approval, declare and approve again. If the harness's
own reviewer still denies the send, report its reason and stop; do not retry
with a different payload. If dispatch waits on a permission prompt or provider
stall, surface it and ask; do not wait silently.

Receipts are advisory, not an approval gate; they stay under private Git
metadata unless `--out` names an existing absolute directory outside the
repository. Use `--retain-transcript` only after a sensitive-data decision.
Treat the review verdict as a claim. Record credible findings and explain
dismissals against the artifact intent. Fix as the single writer and rerun
acceptance tests after every material change. Bound correction rounds:
re-review only fixes, affected boundaries, and new evidence. Join every
requested review before completion; approval cannot settle a queued review.
