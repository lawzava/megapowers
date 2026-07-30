---
name: cross-model-verification
description: >-
  Use when risky billing, auth, concurrency, security, or other high-stakes work
  needs another vendor to independently refute or adversarially verify it.
license: MIT
---

# Cross-Model Verification

A second model catches what the first is blind to only if it is independent.
Independence has two parts: the verifier comes from a different vendor than the
author, and it never sees the author's reasoning or conclusion. A verifier that
sees the prior conclusion anchors to it and confirms; a blind verifier keeps its
edge.

## Prefer an oracle to an opinion

Before asking a model, ask whether an executable check can decide it: tests,
types, a compile, a property test, a reproduction. An oracle is deterministic
and not fooled by a confident argument. Reserve model verification for what no
oracle covers: design soundness, subtle logic, security reasoning, "does this
actually do what it claims".

## Procedure

1. **Launch a verifier with the author vendor declared** via
   multi-agent-delegation's `scripts/delegate-run --role verify
   --author-vendor VENDOR --artifact ... --claim ...`. The resolver fails if it
   cannot route away from every declared author vendor. A second
   instance of the same model shares the same blind spots, and self-preference
   bias is largest when a model judges its own family's output
   (arXiv 2410.21819).

2. **Hand over the artifact and the claim, nothing else.** The verifier gets the
   diff, code, or document plus a crisp statement of what it is supposed to do
   or guarantee. Withhold the author's chain-of-thought, self-review, and
   justification. Information restriction is the point.

3. **Prompt it to refute, with the burden of proof on "verified".** Ask for the
   bug, the counterexample, the missed case, and a default of not verified
   under any real doubt. The resolved provider's reference file
   (multi-agent-delegation, `references/providers/`) carries the adversarial
   template and output schema that make the verdict machine-checkable.

4. **Escalate to a perspective-diverse panel for high stakes.** Run several
   independent verifiers, each with a distinct lens: correctness, security,
   concurrency, reproduction. The panel exists for coverage, not voting: one
   credible refutation from any lens kills the claim, no matter how many other
   passes said fine.

   A panel is one round, not one per lens. The counter in step 5 cannot express
   that: it resets on any single `approve`, so a lens that passes while its
   siblings are still running clears a count the others are measuring against.
   Read the panel's round number from its first dispatch and treat the panel as
   resolved only when every lens has reported. Do not dispatch panel members
   concurrently with a serial retry of the same role on the same branch.

5. **Act on the verdict as single writer, and stop at three rounds.** The
   verifier reports; it never merges its own fix. The lead applies changes and
   re-verifies material ones. Never trust a self-reported pass; re-run the
   oracle. Three rounds is the cap: after a third round that still returns
   findings, stop and hand the open findings to the human instead of
   dispatching a fourth. Read the count off the receipt's `round` field, which
   counts consecutive rounds for this role on this branch that did not reach
   `approve` and resets on an approve. That field is the authority, not
   recollection, because a lead that compacted cannot recall the count. Same
   cap and same reason as megapowers:subagent-driven-development, step 3.

   Dispatch every round of one loop from the same checkout. The ledger resolves
   per checkout, so a linked worktree keeps its own counter under
   `.git/worktrees/<name>/`. A loop that moves between a delegate's worktree and
   the main checkout restarts at one, which undercounts exactly the churn this
   cap exists to catch.

   When consecutive rounds return the same finding, the loop is not converging
   and another dispatch will not make it. The next move is a different approach
   or a human decision, never a resubmission of the same claim. A fourth round
   is legitimate when the human authorizes it and something has changed that
   makes the extra round decidable rather than arguable, such as a labelled
   oracle the next verdict can be scored against. Journal the decision and the
   reason.

## Guardrails

- Different vendor, or it isn't independent. Same-model "review" is a consistency
  check, not verification.
- Keep the launcher's receipt. It records which model verified what claim,
  against the exact artifact identity, and becomes stale after any change.
- Exit status 8 is a refusal to dispatch, not a failed review: the review
  package was empty. Regenerate the artifact, then dispatch; retrying the same
  dispatch reproduces the refusal.

## Relationship to other skills

- Unsure whether verification, selection, or a council fits? Start at
  mega-orchestration:orchestrating, the decision root.
- Serves as the blind judge in mega-orchestration:best-of-n when no oracle can
  rank candidates, and as the per-answer scrutiny step in
  mega-orchestration:council-adjudication.
