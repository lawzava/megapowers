---
name: grill-me
description: Use when the user asks to be grilled, or wants a plan, decision, idea, or draft stress-tested through a structured interview before any action.
when_to_use: "Trigger phrases: grill me, interview me, stress-test this plan, poke holes in this, challenge my idea, ask me everything before we start."
metadata:
  short-description: Structured interview that stress-tests a plan or idea
---

# Grill Me

Interview the user until you reach a shared understanding. The interview is the
deliverable. Do not implement, edit, or plan execution until the user confirms
shared understanding.

## Map the decision tree

Model the topic as a tree of decisions. Each answer can open new decisions
under it. The interview is topic-agnostic: designs, plans, architecture,
product ideas, processes, and prose all qualify. Track each decision as
settled, open, or blocked by an open decision.

## Ask in rounds

The frontier is every open decision whose prerequisites are settled. Ask the
whole frontier in one round. Number each question. Give each question one
recommended answer and its tradeoff. A question that depends on another open
question belongs to a later round. After each round, wait for the answers,
update the tree, and recompute the frontier. Open each round with a short
restatement of settled decisions and open branches; the restated tree, not
earlier history, carries the interview state. Rounds are requested depth: they
may exceed a standing prose cap but stay scannable.

## Resolve facts yourself

Facts are your job; decisions are the user's. Resolve repository and
environment facts before asking, as in design-and-plan. When a fact needs a
long lookup, ask the rest of the frontier now and mark dependent questions
blocked. Do not ask the user for a fact you can look up.

## Finish

Stop when the frontier is empty and no assumption remains unstated. Summarize
the settled decisions and remaining risks. Confirm shared understanding. A
finished interview does not authorize any change; execution needs its own
instruction from the user.
