---
name: independent-review
description: Use when security, auth, billing, concurrency, data integrity, or another high-stakes artifact needs adversarial review by a provider other than its author.
when_to_use: "Trigger phrases: get a second opinion from another model, external review, have another vendor review this, adversarial review of auth, billing, or migration code before merge."
metadata:
  short-description: Cross-provider adversarial review of a high-stakes artifact
---

# Independent Review

Use model review for residual uncertainty after executable checks. Same-provider
review gives only context separation, not independence. The lead owns
remediation and reruns the oracle.

## Trusted review path

Take the reviewer family and exact command from the capability registry or
user instruction; never guess. Inspect a file or immutable commit range before
any external call. Set `review_tool` to the installed
`scripts/megapowers-review.go` beside this `SKILL.md`, not a project copy.

```bash
reviewer_cmd='<reviewer CLI and arguments>'   # {prompt_file} and {scratch_dir} expand
intent='<intended behavior and acceptance boundary>'
go run "$review_tool" inspect --file path/to/file --intent "$intent" \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
go run "$review_tool" inspect --base <base-revision> --head <head-revision> \
  --intent "$intent" --provider <reviewer-family> --provider-command "$reviewer_cmd"
```

The inspection prints the intent, binary hash, command, chunk package hashes,
and one `approval_token`. Check that disclosure, then pass the token for that
exact package, intent, binary, and command:

```bash
go run "$review_tool" review --file path/to/file --intent "$intent" \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external "$approval_token"
```

Author and provider labels must differ and name real vendor families.
`--file` also accepts a `.diff` or `.patch` file under `$TMPDIR`. Credentials
pass only through `--provider-env NAME`. A preflight probe fails fast on login,
usage-limit, or stall before any artifact bytes leave the machine. Any file,
intent, binary, or command change requires a new inspection and token.

## Dispatch authority

External dispatch sends artifact bytes to another vendor. A reviewer bound to
`independent-review` in the capability registry, or named by a user or
repository instruction, is pre-approved: decide when to review and dispatch
without asking, passing each new token yourself. Ask first only when no
configured reviewer exists, the disclosure exceeds the intended scope or holds
content an instruction keeps local, or an instruction requires approval. Never
silently substitute a same-provider review. If the harness's own reviewer
denies the send, report its reason and stop; do not retry with a different
payload. Surface a permission prompt or provider stall instead of waiting
silently.

Receipts are advisory, not an approval gate; `--out`, when given, must name an
existing absolute directory outside the repository. If the tool fails, report
its error; a direct provider call skips its secret scan. Use
`--retain-transcript` only after a sensitive-data decision. Treat the review
verdict as a claim. Record credible findings and explain dismissals against the
artifact intent. Rerun acceptance tests after every material change. Bound correction rounds:
re-review only fixes, affected boundaries, and new evidence. Join every
requested review before completion; approval cannot settle a queued review.
