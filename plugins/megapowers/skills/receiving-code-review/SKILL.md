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

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
