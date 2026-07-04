---
name: best-of-n
description: >-
  Use for hard or high-stakes work where the solution space is wide and one
  attempt (even iterated) is risky — generate N independent candidate solutions,
  then SELECT the best by an executable oracle first and a blind judge second.
  Triggers on "try a few approaches and pick the best", "best-of-n", "generate
  several solutions and choose", high-uncertainty implementation or design.
  Distinct from consensus/averaging: you pick ONE winner, you do not blend views.
license: MIT
---

# Best-of-N

Generate several candidate solutions independently, then select the strongest.
This beats one-attempt-iterated when the solution space is wide and the cost of a
wrong path is high. The evidence is specific: **selection with an independent
oracle wins; blending or averaging candidates loses**: a deliberating team
regresses toward its weaker members: self-organizing teams average expert and
non-expert views, losing up to 41% against their best member (arXiv 2602.01011),
and single agents match multi-agent systems at equal compute (arXiv 2604.02460).
The failure mode is interactive *deliberation*, not answer-voting: non-interactive
majority vote over discrete answers stays valid (step 5 builds on it). So this skill
SELECTS a single winner: it never merges opinions into a compromise.

## When to use it (spend by stakes × uncertainty)

Scale N to how much is at stake and how uncertain you are, and set a stopping rule
up front:

- Routine, low-uncertainty work: N = 1 (don't use this skill).
- Genuinely uncertain approach, moderate stakes: N = 2–3.
- High stakes and wide solution space (a tricky algorithm, a public API shape, a
  security- or money-touching change): N = 3–5.

Stop early when an executable oracle is satisfied by a candidate and no other
candidate can beat it on the oracle. Don't scale N past the point where new
candidates stop differing — log it if you cap.

## Procedure

1. **Write ONE precise brief and, if one can exist, an executable acceptance
   oracle.** The oracle is the whole game: a test suite, a type/compile check, a
   property test, a benchmark threshold — anything that decides "correct" without a
   human or a model opinion. Spend effort here; a good oracle makes selection
   objective.

2. **Generate N candidates independently.** Dispatch N implementers from the *same*
   brief, each in its own isolated worktree (see megapowers:using-git-worktrees),
   as single writers (they return patches / work only in their worktree — see
   mega-orchestration:multi-agent-delegation). Vary the angle to get real diversity:
   different models where available (resolve the `small_impl` role via
   `multi-agent-delegation`'s `scripts/delegate-resolve small_impl`), or the same
   model with different framings (MVP-first, risk-first, performance-first).
   - **Information restriction is mandatory:** each candidate is produced blind to
     the others. A worker that sees another's attempt anchors to it, collapsing
     your N into 1. No shared scratch, no "here's what the last one did".

3. **Select — oracle first.** Run every candidate through the executable oracle
   (in its own worktree). Discard the ones that fail. If exactly one passes, it
   wins — you're done. If several pass, prefer the simplest/smallest that fully
   passes, or go to step 4 to break the tie.

4. **Select — blind judge second (only when the oracle can't decide).** When no
   oracle exists, or several candidates pass it, blind the set with
   `scripts/anonymize-candidates`: it copies the candidates to `candidate-A..N` in
   randomized order, strips the authorship markers you name, and refuses if any
   survives; keep the label→author manifest it prints private. Hand an independent
   judge the anonymized artifacts (no author labels, no reasoning traces, no
   deliberation) and have it rank them against the brief's criteria. Mitigate
   position bias: randomize presentation order, or run the ranking twice with the
   order swapped and treat an order-flipped verdict as a tie. Rank on the criteria,
   not length: longer answers are not better answers. Blindness is load-bearing: a
   judge that sees who wrote what, or a candidate's own argument for itself, anchors
   instead of evaluating. Prefer a judge from a different vendor than the authors:
   resolve the `judge` role via `scripts/delegate-resolve judge` (see
   mega-orchestration:cross-model-verification).

5. **High-stakes and no oracle? Aggregate verifiers, not one judge.** A single judge
   is one point of failure. Run several aspect verifiers over the anonymized set
   (correctness, security, simplicity) and aggregate their approvals, reusing
   mega-orchestration:cross-model-verification's panel (route each via
   `scripts/delegate-resolve judge`). Diverse verifiers scale better than one judge or
   self-consistency (BoN-MAV, arXiv 2502.20379). Selection-by-approval, distinct from
   that skill's refutation-coverage use of the same panel.

6. **Integrate the winner as single writer.** The lead applies the winning
   candidate. Optionally graft a specific better idea from a runner-up — but only
   as a deliberate, reviewed change, never by blending diffs.

## Guardrails

- This is SELECTION, not consensus. Never average candidates or merge their diffs
  to "combine strengths" — that reintroduces the failure mode this skill avoids.
- Keep a single-writer integration path: candidates live in worktrees or as
  patches; only the lead lands the winner.
- Record what was compared and why the winner won (the oracle result or the judge's
  ranking) so the choice is reviewable — see mega-orchestration:multi-agent-delegation for
  the provenance habit.
- Never trust a candidate's self-reported pass; the lead (or the oracle) re-runs.

## Cheap reference

| Situation | Do |
|---|---|
| Executable oracle exists | Run it on every candidate; the passer wins. Judge only breaks ties. |
| No oracle possible | Blind judge on anonymized candidates; prefer a cross-vendor judge. |
| No oracle, high stakes | Aggregate aspect verifiers (correctness/security/simplicity), not one judge. |
| Several candidates pass | Prefer the simplest full-pass, or blind-judge the passers. |
| Candidates converged | Stop adding N; you've explored the space. Log the cap. |
| Want to "merge the best of both" | Don't blend diffs — pick one winner, then graft one idea as a reviewed edit. |
| Unsure best-of-n is the right structure | mega-orchestration:orchestrating routes task shapes to structures. |
