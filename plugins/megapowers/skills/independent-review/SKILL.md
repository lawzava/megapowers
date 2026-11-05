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

## Choose the review route

Read the reviewer binding from the capability registry or user instruction.
Use a different vendor from each portion's author. Unavailable review remains
pending, not permission to switch providers.

- **Native subagent:** use the harness's native tool with the bound read-only
  reviewer. Supply artifact identity, requirements, acceptance boundary, scope
  and check evidence, not unrelated history. Inspect the brief and accessible
  files for secrets and disclosure scope. This route forwards no credentials
  in the brief; authentication stays with the harness. It does not run the
  external tool's secret scan, binary verification, environment filtering or
  package-hash checks, nor sandbox filesystem access. Confirm the actual vendor
  and permissions; native review can send bytes off-machine.
- **Outside the harness:** use the installed tool's secret scan and disclosure
  inspection for every package. Read
  [external review](references/external-review.md) for command shapes,
  `--approve-external`, `--intent`, and approval-token handling. The tool
  forwards no credentials. Without Go or a shell, send nothing through this
  route and report the limitation.

## Dispatch authority

A bound or instruction-named reviewer is pre-approved within the intended
scope. Ask first only when no reviewer is configured, disclosure exceeds that
scope or includes local-only content, or instructions require approval or
inspection. Both routes honor denials and report permission/provider stalls;
never bypass a failed scan with a direct send.

## Reviewer contract

Require `verdict`, `scope.creep` (unrequested changes, drive-by refactors, new
dependencies), `scope.shrink` (stubs, TODOs, weakened, skipped or deleted tests,
loosened assertions, unconfirmed acceptance items), and `findings`. Each finding
has severity (`blocker`, `major`, or `nit`), file, line, claim, and evidence.
Use explicit empty lists and state verification limits.

The lead computes **fail** when any blocker, major, scope.creep or scope.shrink
exists, regardless of the stated verdict. Nits never fail. Verify findings;
record disproof for false positives and resolve failures before approval.
Missing fields or results are gaps, never a pass.

Set the round cap before dispatch; re-review only
[fixes since the prior round](references/correction-rounds.md), affected
boundaries, and new evidence. Exhaustion leaves unresolved findings open for
the user. Join every review before completion; approval cannot settle a queued
review. Receipts record execution, not acceptance.
