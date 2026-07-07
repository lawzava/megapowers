---
name: cross-model-verification
description: >-
  Use to verify risky or high-stakes work with an independent, different-vendor
  model that tries to REFUTE it — for billing/auth/concurrency/security logic, a
  critical claim, or any artifact where a single model's blind spots are costly.
  Triggers on "get an independent review", "find the bug in this", "verify this
  before I trust it", "adversarial pass". Prefer an executable oracle over model
  opinion wherever one exists.
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

1. **Resolve a verifier from a different vendor than the author** via
   multi-agent-delegation's `scripts/delegate-resolve verify`. If the routed
   provider matches the author's vendor, re-route until it differs. A second
   instance of the same model shares the same blind spots, and self-preference
   bias is largest when a model judges its own family's output
   (arXiv 2410.21819).

2. **Hand over the artifact and the claim, nothing else.** The verifier gets the
   diff, code, or document plus a crisp statement of what it is supposed to do
   or guarantee. Withhold the author's chain-of-thought, self-review, and
   justification. Information restriction is the point.

3. **Prompt it to refute, with the burden of proof on "verified".** Ask for the
   bug, the counterexample, the missed case, and a default of not verified
   under any real doubt. For a Codex verifier, the adversarial template and
   output schema in multi-agent-delegation's references/prompting-codex.md make
   the verdict machine-checkable.

4. **Escalate to a perspective-diverse panel for high stakes.** Run several
   independent verifiers, each with a distinct lens: correctness, security,
   concurrency, reproduction. The panel exists for coverage, not voting: one
   credible refutation from any lens kills the claim, no matter how many other
   passes said fine.

5. **Act on the verdict as single writer.** The verifier reports; it never
   merges its own fix. The lead applies changes and re-verifies material ones.
   Never trust a self-reported pass; re-run the oracle.

## Guardrails

- Different vendor, or it isn't independent. Same-model "review" is a consistency
  check, not verification.
- Record which model verified what, against which claim, so the result is
  replayable.

## Relationship to other skills

- Unsure whether verification, selection, or a council fits? Start at
  mega-orchestration:orchestrating, the decision root.
- Serves as the blind judge in mega-orchestration:best-of-n when no oracle can
  rank candidates, and as the per-answer scrutiny step in
  mega-orchestration:council-adjudication.
