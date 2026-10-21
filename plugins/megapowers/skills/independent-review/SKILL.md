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

Take the reviewer family and exact command from the capability registry or user
instruction; never guess. Set `review_tool` to the installed
`scripts/megapowers-review.go` beside this `SKILL.md`, not a project copy.

For a pre-approved reviewer, dispatch in one step with
`--approve-external configured` or inspect first and pass the emitted
`approval_token`:

```bash
reviewer_cmd='<reviewer CLI and arguments>'   # {prompt_file} and {scratch_dir} expand
intent='<intended behavior and acceptance boundary>'
go run "$review_tool" review --file path/to/file --intent "$intent" \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external configured
```

When an instruction requires human inspection before dispatch or no configured
reviewer exists, run `inspect` first (`--file path/to/file` or
`--base <base-revision> --head <head-revision>`), check the disclosed intent,
binary hash, command, and chunk package hashes, and pass the emitted token to
`review --approve-external "$approval_token"`. Any file, intent, binary, or
command change requires a new inspection and token.

Author and provider labels must differ and name real vendor families (such as
`anthropic`, `openai`, or `google`). `--file` takes a file in the
repository, in the current directory outside Git, or under `$TMPDIR`; keep
review packages and receipts under `$TMPDIR`, never in the deliverable.
Credentials pass only through `--provider-env NAME`. `--preflight` probes
liveness before artifact bytes leave the machine. `--help` prints every path
rule.

## Dispatch authority

External dispatch sends artifact bytes to another vendor. A reviewer bound to
`independent-review` in the capability registry, or named by a user or
repository instruction, is pre-approved: decide when to review and dispatch
without asking. Ask first only when no configured reviewer exists, the
disclosure exceeds the intended scope or holds content an instruction keeps
local, or an instruction requires approval. Never silently substitute a
same-provider review. If the harness's own reviewer denies the send, report its
reason and stop. Surface a permission prompt or provider stall instead of
waiting silently.

Receipts are advisory, not an approval gate; `--out` must be an existing
absolute directory outside the review root. If the tool fails, report
its error; a direct provider call skips its secret scan. Use
`--retain-transcript` only after a sensitive-data decision. Treat the review
verdict as a claim. Record credible findings and explain dismissals against the
artifact intent. Bound correction rounds: focused tests per fix, the full suite
once after the last round; re-review only
[fixes since the prior round](references/correction-rounds.md), affected
boundaries, and new evidence. Join every requested review before completion;
approval cannot settle a queued review.
