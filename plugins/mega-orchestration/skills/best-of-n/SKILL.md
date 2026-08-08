---
name: best-of-n
description: >-
  Use when a hard implementation or design task needs independent candidates
  and one winner chosen by tests or blind comparison. Unlike a council, it
  selects a work product.
license: MIT
---

# Best-of-N

Use this for a wide, costly solution space where selecting one work product is better
than iterating a single attempt. Do not use it for routine work or to manufacture a
consensus.

## Inputs and output

Input: one precise brief, acceptance criteria, an executable oracle where possible,
candidate count, and a stopping budget. Output: one selected candidate, its oracle or
blind-comparison evidence, and a record of why alternatives lost.

## Method

1. Define the oracle before candidates start. A test suite, property, type check,
   benchmark threshold, or reproducible check selects correctness. State a tie-breaker
   for multiple passing candidates and a stop condition for diminishing diversity.
2. Produce candidates independently. Each candidate has isolated write space and no
   visibility into another candidate's artifacts, reasoning, or scratch state.
3. Run the oracle independently for every candidate. A self-reported result is not
   selection evidence. A sole full pass wins; otherwise prefer the simplest complete
   pass or proceed to blind comparison.
4. If judgment is necessary, anonymize copies, not source candidates. Refuse to publish
   a set if copying, stripping, or verification fails. Source candidates remain
   immutable; marker-stripped copies are the content judges see.
   `scripts/anonymize-candidates` performs that publication gate; its
   `scripts/tests/anonymize-candidates.test.sh` characterizes copy and scan failures.
   Use a writer-controlled output parent. Its `--out` path is the ordinary directory
   atomically published after validation. After judgment, remove that directory:

   ```bash
   out="<exact --out path passed to anonymize-candidates>"
   [ -d "$out" ] && [ ! -L "$out" ] || { echo "anonymous set missing: $out" >&2; exit 1; }
   rm -rf -- "$out"
   ```

   Give the judge only that set and the criteria.
5. Integrate one winner through the designated single writer. A runner-up idea becomes
   a separately reviewed change, never a blended candidate diff.

## Routing contract

Hard candidates require a routing request that states candidate stakes, required
capabilities, isolation, and independence constraints. The router must choose a route
whose policy admits that work. It must not silently reuse a route reserved for cheap,
ordinary implementation merely because the route is available. If no qualifying route
exists, reduce the task, obtain authorization for a suitable route, or use one explicit
implementation path. Route the blind judge away from every candidate author; otherwise
label the result as a non-independent comparison.

## Safety and oracle

Selection is not synthesis: never merge candidates to create a compromise. Preserve
candidate provenance and test output until selection is reviewable. An executable
oracle outranks model preference; blind comparison is a fallback, not proof of runtime
correctness.
