---
name: independent-review
description: "Use when security, auth, billing, concurrency, data integrity, migrations, or another high-stakes artifact needs an independent opinion from another model before merge. Covers get a second opinion, external review, another vendor's review, and adversarial review."
metadata:
  short-description: Independent model review of a high-stakes artifact
---

# Independent Review

Use another model for residual uncertainty after executable checks when the
stakes warrant it. The lead owns remediation and reruns the oracle.

## Choose the review route

Follow the user's global instructions (CLAUDE.md or AGENTS.md) for what counts
as independent and which reviewer to use, including any local reviewer binding.
Otherwise choose from what is actually available: native subagents with a
different model or the harness's native review.
Cross-vendor review is preferred but optional, depending on installed options.
Do not assume a provider or model is available. Report unavailable required
review without silently replacing the user's choice.

This skill supplies guidance only. Use the selected system's existing review
interface and permissions. Give the reviewer artifact identity, requirements,
acceptance boundary, scope, and check evidence. Keep review access read-only.

## Dispatch authority

Check briefs and accessible files for secrets and disclosure scope before
sending anything off-machine, including through native review. Exclude secrets
and unrelated local content. Follow user instructions and existing permission
rules for sending to the selected reviewer. A bound or instruction-named
reviewer is pre-approved within its stated scope. Dispatch without asking
again when that authority covers the review. Ask when required authority is
missing or disclosure exceeds the approved scope. Honor denials and report
permission or provider stalls.

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
review.
