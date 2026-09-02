---
name: independent-review
description: Use when security, auth, billing, concurrency, data integrity, or another high-stakes artifact needs adversarial review by a provider other than its author.
when_to_use: Trigger phrases: get a second opinion from another model, external review, have another vendor review this, adversarial review of auth, billing, or migration code before merge.
metadata:
  short-description: Cross-provider adversarial review of a high-stakes artifact
---

# Independent Review

Use model review for residual uncertainty after tests, types, reproductions,
and measurements. State the artifact intent and acceptance boundary first. A
same-provider second look is context separation, not independence. The lead
owns remediation and reruns the real oracle.

## Trusted review path

The tool binds to no vendor: the operator supplies a reviewer family label and
the exact command, from the capability registry or the user's instruction,
never from a guess. Review a file or immutable commit range. Inspect before any
external call.
Set `review_tool` to the installed `scripts/megapowers-review.go` beside this `SKILL.md`, not a project copy.

```bash
reviewer_cmd='<reviewer CLI and arguments>'   # {prompt_file} and {scratch_dir} expand
go run "$review_tool" inspect --file path/to/file \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
go run "$review_tool" inspect --base <base-revision> --head <head-revision> \
  --provider <reviewer-family> --provider-command "$reviewer_cmd"
```

The inspection prints the binary path and hash, the command, each chunk's
package hash, and one `approval_token`. Check them, then approve only that
exact package, binary, and command with the author's family declared:

```bash
go run "$review_tool" review --file path/to/file \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external "$approval_token"
```

Author and provider labels must differ and name real vendor families. The
prompt arrives on stdin unless the command names `{prompt_file}`. Credentials
pass only through `--provider-env NAME`. Large ranges split into at most 16
directory-grouped chunks. A preflight probe fails fast on login, usage-limit,
or stall before any artifact bytes leave the machine.

External dispatch sends artifact bytes to another vendor. Confirm the user
authorized that egress for this artifact; a review mandate alone does not
authorize disclosure. A dispatch waiting on a permission prompt or provider
stall is blocked: surface it and ask; do not wait silently.

Any file or provider-binary change or command change requires a new inspection.
Pass `--approve-external` only after checking the disclosure's paths, bytes,
source identity, command, and hashes. Receipts are private and advisory, not an approval gate or
tamper-proof attestation. The transcript is not retained by default. Use
`--retain-transcript` only after a sensitive-data decision. Treat the review
verdict as a claim. Record credible findings, and explain dismissed findings
against the artifact intent. Fix as the single writer, then rerun the
artifact's acceptance tests after every material change.

The default receipt stays under private Git metadata. An explicit `--out` must
already exist as an absolute canonical directory outside the repository and may
not contain it.
