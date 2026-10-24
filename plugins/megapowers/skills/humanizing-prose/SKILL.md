---
name: humanizing-prose
description: Use when the task itself is to draft, rewrite, edit, or preserve human-facing prose for clarity, attribution, or proportion; do not select merely because other work ends with a response.
when_to_use: "Trigger phrases: rewrite this, make it read human, remove AI slop, no em dashes, tighten this doc, draft the announcement, edit the blog post, polish the PR description, sounds robotic, write it in my style, draft the Slack message, write the Linear or Jira comment, ticket description, write it so a non-engineer understands, write normally."
metadata:
  short-description: Draft or edit human-facing prose without AI-slop markers
---

# Humanizing Prose

This skill replaces the length, sentence, and opening defaults of the
operator [output style](../../output-styles/megapowers.md). Its attribution
and specific-evaluation rules still apply.

Identify the speaker, recipient, purpose, and what is already settled. Ask for
missing context only when it changes the substance. Write from an established
speaker's perspective, never the assistant's. When the user asks for their own
voice, copy the sentence length, vocabulary, punctuation, capitalization, and
formatting of their human-facing writing, not their prompts to the agent. When
editing, keep sentences that work and change only defects.

Preserve the requested substance, including every recommendation the user
asked to share, with the facts, reasons, caveats, and open questions needed to
judge it. Do not drop required detail to fit a length. Keep quotations, code,
commands, identifiers, numbers, and decisions exact. State a verification
limit inside the sentence, not as an appended disclaimer. Never invent facts,
test runs, agreement, ownership, commitments, or experience.

Keep proposals, questions, decisions, and commitments distinct. Do not turn an
offer into a promise. Lead with the point in reader order, not the order of
investigation. Choose paragraphs, lists, and headings for the reader's task.

## Machine-prose markers

The quoted phrases are examples. Cut the move, not only the phrase.

- No em dashes or spaced hyphens as dashes, even if the source uses them.
- No `**Label:**` lead-ins unless a template requires them, no headings on
  short messages, or summary headers that restate the body.
- No "not X but Y" framing, rhetorical questions, or announcements of
  importance ("it's worth noting", "crucially").
- No padding, sales language, generic optimism, session history, opening
  praise, or closing offers and recaps.
- Vary sentence length instead of stacking parallel clauses or adjective
  triads. Keep one register. Use the same noun for the same referent.
- Collapse stacked hedges to one confidence level while preserving material
  uncertainty.

## Workplace threads and reviews

For a status update or review summary, give the verdict, material impact, and
minimum evidence in plain paragraphs without bold labels. Do not repeat
findings already posted inline. In a discussion reply, open with your answer,
stance, or question, not a restatement of the other message. Do not post
progress narration, session handoffs, repeated findings, or test transcripts
as comments. When publication is authorized, reply in the existing thread.
Examples: [workplace comments](references/workplace-comments.md).
