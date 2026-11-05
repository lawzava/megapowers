# Session mining

Transcript mining with session-mine is available as a repository maintainer tool; ask the user for it, or use it when a megapowers checkout is present.

Past sessions can show repeated mistakes and earlier decisions. Read the
original turns and command results before drawing a conclusion. When no
report is available, ask the user for the corrections or sessions to review.

## Classification

Correction reports group candidate turns into these classes:

- `stopped-unnecessarily`: the user had to tell the agent to continue.
- `verification`: the agent claimed success without checking it.
- `shallow`: the user asked for more depth or effort.
- `verbosity`: the answer was too long.
- `overreach`: the agent changed something nobody asked it to change.
- `frustration`: the user repeated a complaint or showed anger.

Keyword classes are candidates, not findings. "Continue" may be a plain
instruction, and a real correction may use none of the keywords. Read each
example before counting it; a repeated class needs two occurrences.

## Reading a report

For recall, default to the last 14 days and this workspace; widen only when
the user asks, and state the scope. Read the relevant sessions for decisions,
open questions, and last state, then check live repository and forge state
before reporting. Give each thread a status and cite the source file and time.

A scorecard counts skill loads, later corrections, historical gate blocks,
verification reports, foreground sleep calls, and run journal events. A
correction after a skill load does not show that the skill caused it. Use the
[learning loop](learning-loop.md) to assess lessons from the evidence.

Review excerpts before sharing them, and share only with the user's approval.
