---
name: best-of-n
description: >-
  Use when a hard implementation or design task needs independent candidates
  and one winner chosen by tests or blind comparison. Unlike a council, it
  selects a work product.
license: MIT
---

# Best-of-N

Generate several candidate solutions independently, then select one. This beats
a single iterated attempt when the solution space is wide and a wrong path is
expensive. The evidence favors selection over blending: deliberating teams
regress toward their weaker members, losing up to 41% against their best member
(arXiv 2602.01011), while non-interactive answer voting stays valid. So this
skill picks ONE winner; it never merges opinions into a compromise.

## Sizing N

Scale N to stakes and uncertainty, and declare a stopping rule up front.
Routine, low-uncertainty work does not need this skill. An uncertain approach
at moderate stakes warrants 2 or 3 candidates. High stakes and a wide solution
space (a tricky algorithm, a public API shape, a security or money touching
change) warrant 3 to 5. Stop early once an oracle-passing candidate cannot be
beaten on the oracle, and stop adding candidates when new ones stop differing;
log it if you cap.

## Procedure

1. **One precise brief, and an executable oracle if one can exist.** The oracle
   (a test suite, a compile or type check, a property test, a benchmark
   threshold) decides correctness without a human or model opinion. Effort
   spent here makes selection objective.

2. **N candidates, isolated and blind.** Dispatch N implementers from the same
   brief, each in its own worktree, each a single writer that returns a patch
   or works only inside its worktree; candidates never write to the repo tree.
   Vary the angle for real diversity: different models where available (resolve
   the `small_impl` role via multi-agent-delegation's `scripts/delegate-resolve
   small_impl`), or same model under different framings. Information restriction
   is mandatory: each candidate is produced blind to the others, with no shared
   scratch space and no sight of another candidate's output. A worker that sees
   another attempt anchors to it, and that collapses your N into 1; this is the
   failure mode the whole structure exists to avoid.

3. **Select by oracle first.** The lead runs every candidate through the oracle
   in that candidate's worktree; a candidate's self-reported pass is never
   trusted. Failures are out. A sole passer wins. Among several passers, prefer
   the simplest full pass, or break the tie in step 4.

4. **Blind judge only for ties or when no oracle exists.** Anonymize the set
   with `scripts/anonymize-candidates`: it copies candidates to
   `candidate-A..N` in randomized order, strips the authorship markers you
   name, and refuses if any marker survives; the label to author manifest it
   prints stays private and the judge never sees it. Hand an independent judge
   the anonymized artifacts alone, with no author labels, reasoning traces, or
   deliberation, and have it rank them against the brief's criteria, not
   length. Mitigate position bias: randomize presentation order, or run the
   ranking twice with the order swapped and treat an order-flipped verdict as a
   tie. Blindness is what makes the judgment independent. Prefer a judge from a
   different vendor than the authors, resolved via `scripts/delegate-resolve
   judge` (see mega-orchestration:cross-model-verification).

5. **High stakes with no oracle: aggregate verifiers, not one judge.** A single
   judge is one point of failure. Run several aspect verifiers (correctness,
   security, simplicity) over the anonymized set and aggregate their approvals,
   reusing cross-model-verification's panel via `scripts/delegate-resolve
   judge`. Diverse verifiers scale better than one judge or self-consistency
   (BoN-MAV, arXiv 2502.20379).

6. **Integrate as single writer.** The lead applies the winning candidate. A
   specific better idea from a runner-up may be grafted as a deliberate,
   reviewed change, never by blending diffs.

## Guardrails

- This is SELECTION, not consensus. Never average candidates or merge their
  diffs to combine strengths; that reintroduces the failure mode above.
- Candidates live in worktrees or as patches; only the lead lands the winner.
- Record what was compared and why the winner won (the oracle result or the
  judge's ranking) so the choice is reviewable.
