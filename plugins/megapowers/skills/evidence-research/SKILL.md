---
name: evidence-research
description: Use to research a decision that needs evidence beyond the repository, such as current upstream behavior, historical rationale, cross-system incidents, or contested claims. Do not use for repository-only code tracing or a summary of provided material.
when_to_use: "Trigger phrases: investigate disputed claims, compare current upstream options, why did we do X, what changed outside this repository, what evidence would settle this decision."
metadata:
  short-description: Question framing and evidence classification beyond the repo
---

# Evidence Research

Keep repository-only code tracing and summaries of already provided material in
the ordinary task flow.

Define the exact question, decision, time boundary, and stopping rule. Start
from the code or artifact anchor. Use repository and Git history before broader
search when they can answer the question.

Query tickets, docs, chat, observability, errors, and analytics only when each
source is available, authorized, and proportionate. Prefer primary sources for
external facts. Use independent sources when a claim is contested or one source
cannot establish the decision.

For each load-bearing claim, classify the support as `direct-statement`,
`direct-observation`, `source-backed`, or `history-entry-only`, and mark the
rest `inferred`, `speculative`, `unknown`, or `contested`. Only the first four
classes support a decision; the same vocabulary governs `memory-hygiene`. Keep API contracts,
observed runtime behavior, and commercial promises distinct. Record sources
consulted, dates or revisions where material, and material gaps. State what
would resolve an unknown or contested claim.

Lead with the decision and minimum sufficient evidence. Cite the source next to
the claim it supports. Save a durable research artifact only at an approved
path. Keep secrets, sensitive transcripts, raw chat, and irrelevant personal
data out of the result. Do not write to trackers or publish findings without
separate authority.

A research conclusion is not authority to implement or publish. Use
`orchestrating` for independent evidence lanes, `safe-effects` for external
writes, and `verify-and-finish` before claiming the research artifact complete.
