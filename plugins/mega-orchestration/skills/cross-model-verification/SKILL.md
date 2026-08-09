---
name: cross-model-verification
description: >-
  Use when risky billing, auth, concurrency, security, or other high-stakes work
  needs another vendor to independently refute or adversarially verify it.
license: MIT
---

# Cross-Model Verification

Use an independent verifier for a risky claim an executable oracle does not fully
settle. Prefer tests, types, reproductions, and measurements for what they can decide.

## Inputs and output

Input: a frozen artifact, the precise claim being verified, declared artifact authors,
scope, and verification criteria. Output: a refutation or bounded verification result
with evidence, uncertainty, and the exact artifact identity reviewed.

## Method

1. Give the verifier the artifact and claim, not the author's reasoning or conclusion.
   The verifier actively seeks counterexamples, missed cases, and evidence against the
   claim.
2. Route verification independently from every declared artifact author. If independence
   is unavailable, report a same-family consistency check, not independent verification.
   `--allow-context-separation` makes that fallback explicit: a fresh same-vendor session
   is the condition the controlled evidence actually measured, and it is worth running
   when the alternative is nothing. It is not worth substituting here. This skill exists
   for the risk class where correlated blind spots are the specific concern, so a
   context-separation receipt records `independent: false` and does not clear the
   risky-logic gate. Say the cross-vendor check did not run.
3. Treat a credible refutation as actionable. The single writer fixes or narrows the
   claim, then verifies the changed artifact again. Keep the receipt bound to the
   artifact identity; it expires after a material change.
4. For high-stakes coverage gaps, use a panel with distinct lenses such as correctness,
   security, concurrency, or reproduction. One credible refutation prevents approval.

For a routed artifact review, use `scripts/delegate-run` with the verification role,
declared authors, artifact, and claim. Its receipt binds the reviewed artifact and
records a consecutive round. The configured round cap stops an unresolved serial loop;
preserve receipts and do not treat a changed artifact as covered by an older receipt.

## Panel accounting contract

The launcher has no panel lifecycle. Manage panel, scope, and member identities
at the lead, retain every receipt, and close the panel only after every member
reports. One approval cannot negate another member's pending or adverse result.

## Safety and oracle

Do not let a verifier merge its own remediation. Preserve the artifact, claim, verdict,
and evidence so another person can reproduce the conclusion. The strongest available
oracle remains authoritative; model verification covers the residual uncertainty.
