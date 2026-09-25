---
name: humanizing-prose
description: Use when the task itself is to draft, rewrite, edit, or preserve human-facing prose for clarity, attribution, or proportion; do not select merely because other work ends with a response.
when_to_use: "Trigger phrases: rewrite this, make it read human, remove AI slop, no em dashes, tighten this doc, draft the announcement, edit the blog post, polish the PR description, sounds robotic."
metadata:
  short-description: Draft or edit human-facing prose without AI-slop markers
---

# Humanizing Prose

Use the available context to identify the speaker, recipient, purpose, and
what the conversation has settled. Write a message that belongs in that
exchange. Ask for missing context only when it materially changes the message.
Follow explicit user requirements for voice and format.
Remove padding, sales language, generic optimism, and irrelevant session history.

Preserve the requested substance, including every recommendation the user
asked to share. Keep facts, reasons, caveats, and uncertainty needed to assess
it. Summarize only within the requested scope; do not omit required detail to
fit a preferred length or structure. Preserve identifiers, numbers, commands,
and decisions accurately. Do not rewrite quotations, code, or command names. Never invent
facts, agreement, ownership, commitments, or experience.

Keep proposals, questions, decisions, and commitments distinct. Use the author's
perspective when established, without turning a proposed assignment into a
promise. Choose paragraphs, lists, and headings for the reader's task; do not
copy the assistant's analysis structure by default. Leave already-effective
prose alone.

For examples of discussion and review structure, see
[workplace comments](references/workplace-comments.md).

## Machine-prose markers

Apply the plugin [output style](../../output-styles/megapowers.md) rules for
attribution and specific evaluation; load it if it is not active. Do not use
em dashes or spaced hyphens as em dashes, even when the surrounding archive
uses them. Remove triads of adjectives, "not X but Y" framing, rhetorical
questions, "it's worth noting", "in today's landscape", summary headers that
restate the body, and generic closing offers or recaps. Vary sentence length
instead of stacking parallel clauses. Keep one register across the piece.
Replace unmeasured intensifiers with a number, bounded scope, or source.
Collapse stacked hedges to one confidence level while preserving material
uncertainty.

For a status update or review summary, give the current verdict, material
impact, and minimum evidence. For a discussion reply, include the requested
recommendations, useful reasons, and unresolved questions. Check that the
recipient can understand what the message adds and any response needed.
Do not publish routine progress narration, session handoffs, repeated findings,
or test transcripts as tracker, issue, or PR comments. When publication is
authorized, reply in the existing thread when possible.
