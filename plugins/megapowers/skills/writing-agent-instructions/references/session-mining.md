# Session mining

Use the harness's normal read-only search and file tools to inspect local
Claude Code and Codex transcripts. Default to this workspace and the last
14 days for recall. Widen only when the user asks and state the scope.
Without accessible transcripts, ask for the relevant corrections or sessions;
do not reconstruct them from memory.

For recall, locate messages containing the topic words, most recent first.
Read decisions, open questions and last state. Check live repository and forge
state before reporting. Give a short brief: work, thread statuses, at most five
open problems and the next move. Mark unknown status explicitly.

For corrections, read each user turn with the preceding agent text. Candidate
classes are stopped-unnecessarily, verification, shallow, verbosity, overreach,
and frustration. Count a class after two occurrences; label one-offs.
Keywords alone are not findings. Open the transcript when context is unclear.

Separate human, machine and subagent prompts. Exclude compaction summaries,
producer notifications, Codex exec prompts and scheduled continuation prompts
from human corrections. A scripted prompt without a machine marker can be
indistinguishable from a human turn; report that limit. Deduplicate rollouts
when inspecting active and archived Codex sessions. State counts by source.

For a manual scorecard, count inspected sessions, direct user corrections,
explicit verification reports, and recorded project-journal outcomes. Cite each
count's source and date window. Skill-load associations, waits, and cross-session
event totals are best-effort: count only records whose meaning is clear from
the surrounding context, and label incomplete coverage. Do not infer timing,
causation, or missing behavior from keyword matches. See
[learning loop](learning-loop.md).

Keep transcript bodies local. Redact emails, private access data, home paths,
and sensitive URL components from excerpts, then inspect for missed secrets.
Keep transcripts and excerpts out of review packages, issues, commits, and
other public text.

Keep output only to the local terminal or local files. Mined text is never
sent anywhere.
