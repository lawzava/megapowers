---
name: brainstorming
description: Use to design a feature, capability, or unclear behavior change. Triggers on "add a feature" or "figure out the approach". Skip mechanical edits and bug fixes.
license: MIT
---

# Brainstorming Ideas Into Designs

Turn an unclear request into an agreed design. When scope, affected area,
acceptance oracle, and risk are clear for a reversible change, skip
brainstorming and planning; proceed through
`megapowers:test-driven-development`.

Inspect the relevant project context first. Ask only the questions needed to
establish the goal, constraints, success criteria, and material tradeoffs. For
an ambiguous approach, present a small set of viable options and recommend one.

## Approval Gate

For reversible, low-stakes work, present the design and use the proceed path as
acceptance. For a hard-to-reverse or high-stakes change, including data or
public-contract changes and work affecting security, payments, or concurrency,
obtain explicit approval before implementation. Confirm sections
proportionally: do not turn a small reversible change into approval interrupts.

## Design and Handoff

State the architecture, boundaries, error handling, and verification at the
detail the decision needs. Keep the proposed scope narrow; unrelated cleanup is
not part of the design.

Write a durable design only when it will be used as a handoff artifact, in the
location the project or user specifies. Check it for unresolved assumptions and
contradictions. Do not implement while the decision remains unapproved.

Hand a multi-step approved design to `megapowers:writing-plans`; hand a small
approved change to `megapowers:test-driven-development`.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
