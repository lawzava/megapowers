---
name: humanizing-prose
description: Use to draft or edit user-facing docs, release notes, PRs, announcements, or errors. Triggers on "humanize", "sounds like AI", "slop", or "read naturally".
license: MIT
---

# Humanizing Prose

## Overview

AI-generated prose has recognizable tells, and they cluster: one em dash means
nothing; em dashes plus a sales punchline plus a triple of adjectives reads as
generated. Rewrite the tells and keep the claims: every fact, number, command,
and name must survive the edit untouched.

## Dashes

No em or en dashes. Before returning final text, scan it for `—` and `–` (and
` -- ` doing dash work); any hit means the draft is not done. Replace each
with a period and a new sentence, a comma, a colon, or parentheses, or
restructure the sentence.

## Deflate the sales register

Marketing reflexes read as generated even when the facts are right. Replace
sizzle with the measurement: "you'll feel the difference immediately" carries
less than "full builds finished 3x faster on a 400-package repo". Cut
punchlines that address the reader ("this release is for you", "if X has been
the slow part of your day"). State what changed and let the numbers do the
selling.

## Structure defaults

Threes are the model's default rhythm: adjectives, clauses, and bullets come
in threes whether or not the content is three things. Break the triple unless
the content really is three things. The same goes for one-short-sentence
drama: one is fine, a run is engineered.

## Editing text that arrived AI-flavored

When the job is to de-slop existing text, also sweep the classic tells that
show up in older or heavier drafts: vocabulary (delve, tapestry, landscape,
testament, seamless, robust, vibrant, leverage, journey, empower), elaborate
copulas (serves as, stands as, boasts, represents; prefer is, has), filler
("In order to" for "To", "Due to the fact that" for "Because", "It is
important to note that X" for "X"), stacked hedges, signposting that announces
instead of doing ("Let's dive in"), and a closing paragraph of generic
optimism.

## False positives

Tells count only in prose you are writing or rewriting. Never rewrite inside
quotations, code, command names, or text that discusses a phrase rather than
using it. Dry writing without the tells is just dry writing; leave it alone.

## Example

Before: "Parcel delivers a seamless, robust build experience — and with
`--parallel`, you'll feel the difference immediately."

After: "Parcel builds independent packages concurrently with `--parallel`:
3x faster on a 400-package repo."

Origin: Adapted from humanizer (MIT, (c) 2025 Siqi Chen),
https://github.com/blader/humanizer, itself derived from Wikipedia's "Signs
of AI writing" (CC BY-SA); re-derived against a measured frontier baseline
for the technical register.
