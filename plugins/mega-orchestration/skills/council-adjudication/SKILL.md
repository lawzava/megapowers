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

For a decision no test can settle, a panel of models beats one model, but only when
it is adjudicated correctly. The failure mode is a panel that talks until it averages
into a bland compromise; measured, that regresses toward the weaker members. The
council works on one principle: **answer independently, rank blind, synthesize from
the best.** It never averages.

## When to use it

- A real decision (design, architecture, tradeoff, strategy) with more than one
  defensible answer and no executable oracle.
- Stakes high enough to justify several models' time. A decision one competent pass
  settles does not need a council.
- If an executable oracle exists, use mega-orchestration:best-of-n instead;
  selection by oracle beats any panel of opinions.

## Procedure

1. **One sharp question.** State the decision criteria with it: constraints,
   priorities, what must not break. Everything downstream is judged against them.

2. **Independent answers.** Ask N members for a full answer with its reasoning and
   tradeoffs. Use different models or vendors where available (resolve the
   `council_member` role via `multi-agent-delegation`'s
   `scripts/delegate-resolve council_member`), or one model from deliberately
   different starting angles. Members answer blind to each other: no shared thread,
   no building on a previous answer. Independence gives the panel its range.

3. **An anonymized set.** Blind the answers with mega-orchestration:best-of-n's
   `scripts/anonymize-candidates`, naming the exact authorship markers to strip, and
   keep the label→author manifest private. Strip any self-advocacy as well. Every
   reviewer sees the same de-identified set.

4. **Blind ranking.** Each member, or one independent judge, ranks the anonymized
   answers against the stated criteria without seeing authorship or the others'
   rankings. Blindness does not remove self-preference: models favor their own
   generations even without author labels (arXiv 2410.21819). Prefer a non-author
   independent judge; when members rank a set containing their own answer, exclude
   each member's score of its own answer from the aggregate. Counter position bias
   by randomizing answer order, or by ranking twice with the order swapped and
   treating an order-flipped verdict as a tie.

5. **Synthesis from the best, never an average.** The top-ranked answer is the spine
   of the decision. Graft in specific, concrete points from runner-ups where they
   clearly strengthen it, as deliberate additions, never by splitting the difference
   between conflicting positions. Where answers conflict, choose the better-argued
   one against the criteria; record why, and record the dissent.

## Guardrails

- Synthesis is selection plus grafting, never averaging. A recommendation that reads
  as "a bit of everyone's view" is the anti-pattern.
- Independence at answer time, blindness at ranking time. Lose either and the panel
  collapses toward one anchored view.
- Keep the trail: the question, the anonymized answers, the ranking, and the
  reasoning for the final call, so the decision is reviewable.
- The council advises; a human stays in the loop for high-blast-radius or
  irreversible decisions.

## Cheap reference

| You have | Use |
|---|---|
| An executable oracle | mega-orchestration:best-of-n (select by oracle) |
| One artifact + a claim to check | mega-orchestration:cross-model-verification |
| A decision, no oracle, wide space | this skill (answer → rank blind → synthesize from best) |
| Tempted to "average the panel's advice" | Don't: pick the strongest answer, graft the best points |
| Unsure a council is the right structure | mega-orchestration:orchestrating routes task shapes to structures |
