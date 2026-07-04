---
name: council-adjudication
description: >-
  Use for a hard DECISION with a wide solution space and no executable oracle — an
  architecture or design choice, a tradeoff, a strategy call — where you want
  several models' judgment adjudicated well. Collect independent answers, rank them
  ANONYMIZED, and synthesize from the best. Triggers on "get a panel", "have the
  models debate this", "council", "which approach should we take". NOT opinion-
  averaging: you synthesize from the strongest answer, you don't blend all views.
license: MIT
---

# Council Adjudication

For a decision no test can settle, a panel of models beats one model — but only if
you adjudicate it correctly. The failure mode to avoid is a "team" that talks until
it averages into a bland compromise; measured, that regresses toward the weaker
members. So the council here works like the effective core of an LLM council:
**answer independently, rank blind, synthesize from the best.** It never averages.

## When to use it

- A genuine decision (design, architecture, tradeoff, strategy) with more than one
  defensible answer and no executable oracle to decide it.
- Stakes high enough to justify several models' time. For a decision a single
  competent pass settles, don't convene a council.
- If an executable oracle *does* exist, use mega-orchestration:best-of-n instead — selection
  by oracle beats any panel of opinions.

## Procedure

1. **Pose ONE sharp question** with the decision criteria stated (what "good" means
   here: constraints, priorities, what must not break).

2. **Collect N independent answers.** Ask N members — different models/vendors where
   available (resolve the `council_member` role via `multi-agent-delegation`'s
   `scripts/delegate-resolve council_member`),
   or one model from deliberately different starting angles — for a full answer
   *with its reasoning and tradeoffs*. Members answer **blind to each other**; no
   shared thread, no "improve on the last answer". Independence is what gives the
   panel its range.

3. **Anonymize the answers.** Blind the set with mega-orchestration:best-of-n's
   `scripts/anonymize-candidates`: it strips the authorship markers you name,
   refuses if any survives, and emits the answers in randomized order; keep the
   label→author manifest it prints private. Strip any "I'm the best" self-advocacy
   too. Give each reviewer the same de-identified set.

4. **Rank blind.** Have each member (or one independent judge) rank the anonymized
   answers against the stated criteria — not seeing who wrote what, not seeing the
   others' rankings. Blindness stops anchoring, but it does not remove
   self-preference: models still favor their own generations without author labels
   (arXiv 2410.21819). So prefer a non-author independent judge; when members rank a
   set that contains their own answer, exclude each member's score of its own answer
   from the aggregate. Mitigate position bias too: randomize answer order, or rank
   twice with the order swapped and treat an order-flipped verdict as a tie. This
   ranking follows the same independence-and-blinding discipline as
   mega-orchestration:cross-model-verification — but it is a distinct step: here
   reviewers weigh full answers *including* their reasoning and tradeoffs, whereas
   cross-model-verification checks a specific claim with the author's advocacy
   stripped out. Use each for what it's for; don't collapse them.

5. **Synthesize from the best, don't average.** Take the top-ranked answer as the
   spine of the decision. Graft in specific, concrete points from runner-ups where
   they clearly strengthen it — as deliberate additions, not by splitting the
   difference between conflicting positions. When answers genuinely conflict, choose
   the better-argued one against the criteria; record why, and record the dissent.

## Guardrails

- Synthesis is selection-plus-grafting, never averaging. A recommendation that
  reads as "a bit of everyone's view" is the anti-pattern.
- Independence at answer time, blindness at ranking time. Lose either and the panel
  collapses toward one anchored view.
- Keep the trail: the question, the anonymized answers, the ranking, and the
  reasoning for the final call — so the decision is reviewable and replayable.
- The council advises; a human stays in the loop for high-blast, irreversible
  decisions (see megapowers:brainstorming's proportional gate).

## Cheap reference

| You have | Use |
|---|---|
| An executable oracle | mega-orchestration:best-of-n (select by oracle) |
| One artifact + a claim to check | mega-orchestration:cross-model-verification |
| A decision, no oracle, wide space | this skill (answer → rank blind → synthesize from best) |
| Tempted to "average the panel's advice" | Don't — pick the strongest answer, graft the best points |
| Unsure a council is the right structure | mega-orchestration:orchestrating routes task shapes to structures |
