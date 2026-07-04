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

A second model catches what the first is blind to — but only if it is genuinely
independent. Two things make it independent: it is a **different vendor** than the
author, and it does **not see the author's reasoning or conclusion**. A verifier
that sees the prior conclusion anchors to it and sycophantically confirms; a blind
verifier keeps its edge.

## Prefer an oracle to an opinion

Before asking a model, ask whether an executable check can decide it: tests,
types, a compile, a property test, a formal check, a reproduction. An oracle is
cheaper, deterministic, and not fooled by a confident argument. Use model
verification for what no oracle can cover — design soundness, subtle logic,
security reasoning, "does this actually do what it claims".

## Procedure

1. **Pick a verifier from a different vendor than the author.** The value is
   diversity of failure modes; a second instance of the same model shares the same
   blind spots, and self-preference bias is largest when a model judges its own
   family's output; anonymity alone does not remove it (arXiv 2410.21819). Resolve
   the `verify` role via `multi-agent-delegation`'s
   `scripts/delegate-resolve verify` (e.g. if the author was Claude, send the pass
   to Codex; if the author was Codex, send it to Claude or another model — the
   routed provider must differ from the author's vendor, so re-route when they
   collide).

2. **Give the verifier the artifact and the claim — not the author's reasoning.**
   Hand over the diff/code/document and a crisp statement of what it is supposed to
   do or guarantee. Withhold the author's chain-of-thought, self-review, and
   "here's why it's correct". Information restriction is the point.

3. **Prompt it to REFUTE, with the burden of proof on "verified".** Ask it to find
   the bug, the counterexample, the missed case — and to default to *not verified*
   under any real doubt. "Prove this wrong" surfaces more than "check this".

4. **Escalate to a perspective-diverse panel for high stakes.** When a defect could
   fail in more than one way, run several independent verifiers, each with a
   distinct lens — correctness, security, concurrency/races, "does it actually
   reproduce" — rather than N identical reviewers. The panel is for **coverage, not
   voting**: any credible refutation — a concrete counterexample or a reproduced
   bug from *any* single lens — kills the claim. A real defect is not outvoted by
   reviewers who happened not to look where it hides; one strong refutation
   outweighs any number of "looks fine" passes. The same perspective-diverse panel
   also drives multi-aspect *selection* (aggregated approvals) in
   mega-orchestration:best-of-n: there it approves to pick a winner; here one
   refutation from any lens kills the claim.

5. **Act on the verdict as single writer.** The verifier does not co-author or land
   changes; it reports. The lead (or the author) applies fixes and, for a material
   change, re-verifies. Never trust a self-reported pass — re-run the oracle.

## Guardrails

- Different vendor, or it isn't independent. Same-model "review" is a consistency
  check, not verification.
- Blind to prior conclusions. If the verifier can see the author's argument, you've
  lost the benefit.
- The verifier refutes; it does not merge its own rewrite in as the fix. Keep the
  single-writer path (see mega-orchestration:multi-agent-delegation).
- Record which model verified what, against which claim, so the result is
  replayable.

## Relationship to other skills

- Unsure whether verification, selection, or a council fits? Start at
  mega-orchestration:orchestrating, the decision root.
- Use inside mega-orchestration:best-of-n as the blind judge when no oracle can rank
  candidates.
- Use inside mega-orchestration:council-adjudication as the per-answer scrutiny step.
- The `delegate-nudge` hook (Claude Code only) is a backstop that reminds you to
  run this on risky diffs; the discipline lives here, not in the hook.
