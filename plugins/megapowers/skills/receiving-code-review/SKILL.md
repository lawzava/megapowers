---
name: receiving-code-review
description: Use to evaluate review feedback before changing code, especially unclear or suspect suggestions. Triggers on reviewer comments or requested changes. Not for requesting review.
license: MIT
---

# Receiving Code Review

Treat feedback as evidence to evaluate, not instructions to copy. Read each
item, establish the requirement it concerns, and verify the claim against the
code, tests, and project constraints before changing anything.

Resolve every related ambiguity before implementing any item. If feedback
conflicts with an accepted requirement or requires an architectural tradeoff,
ask the requirement owner to decide. Otherwise, respond with the technical
reason for accepting or declining it, then make and verify one focused change
at a time.

Correct a specification-compliance violation, then re-review before proceeding.
For engineering findings, fix material defects before proceeding and record
minor follow-up work separately. Do not add an unneeded feature merely because
it sounds more complete; establish an actual use or requirement first.

Push back with concrete compatibility, behavior, or test evidence when a
suggestion is wrong. The goal is a correct implementation, not performative
agreement.

When feedback lives in review threads, respond in the existing review thread
only when the response changes reviewer state. Give the current decision and
minimum evidence: `Fixed in <sha>. Test: <name>.` for an accepted finding, or
`Not changing: <one decisive technical reason>.` for a declined finding. Do not
restate the finding.

Do not post an aggregate top-level review ledger after each fix wave or repeat
thread outcomes in a final recap. The threads and checks are the record. If the
requirement owner asks for a top-level summary, limit it to the current verdict,
blocker, and next action in at most three bullets. When another review is
required, post the review trigger as its own minimal comment.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
