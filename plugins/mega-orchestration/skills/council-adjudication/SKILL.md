---
name: council-adjudication
description: >-
  Use when a hard architecture, design, strategy, or tradeoff decision lacks an
  executable oracle and needs independent model judgments. Unlike best-of-n, it
  selects a recommendation.
license: MIT
---

# Council Adjudication

Use a council for one consequential decision with several defensible answers and no
executable oracle. Use best-of-n when a test, measurement, or other oracle can select
a work product.

## Inputs and output

Input: a decision question, constraints, decision criteria, and the authority that may
accept the recommendation. Output: a recommendation with its selected rationale,
specific adopted points, dissent, and a reviewable record of the question, anonymous
answers, rankings, and final rationale.

## Method

1. Ask members for independent answers. They receive the same question and criteria,
   but not one another's answers or reasoning. Answer generation is not review: it has
   no artifact author, and must not require one.
2. Remove authorship and self-advocacy before ranking. Publish an anonymous set only
   after every copy and removal succeeds; on any error, publish nothing. Keep the
   label-to-author mapping private from judges. Use best-of-n's
   `scripts/anonymize-candidates` in a writer-controlled output parent; it publishes
   an ordinary directory atomically, which the lead removes after ranking.
3. Rank the same anonymous set against the stated criteria. Prefer judges independent
   of the answer authors. Exclude self-rankings if a member ranks its own answer, and
   counter order bias with randomized or reversed presentation.
4. Select the strongest answer as the recommendation's spine. Add only concrete points
   that strengthen it. Record material conflicts and dissent. Do not average positions.

## Dispatch and accounting boundary

Dispatch each answer through the authorless `council_member` generation role: record the
member identity and question, but supply no artifact author. Give the anonymized set to
the `judge` role with every answer author excluded. The lead records stable panel, scope,
and member identities, then closes one panel cycle only after every member reports rather
than treating it as serial review rounds. `scripts/delegate-run` records one artifact
review only; it does not supply panel receipts or accounting. If the lead cannot preserve those
boundaries, use a single accountable decision maker instead of claiming independent
panel behavior.

## Safety and oracle

The council is advisory. A human or delegated authority decides actions with significant
external impact. The oracle is criterion-traceability: every conclusion and graft maps
back to a ranked anonymous answer or a recorded decision reason. A confident majority
is not an oracle.
