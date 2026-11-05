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

For a scorecard, record skill loads and corrections before the next human turn,
historical gate blocks, VERIFIED and NOT VERIFIED reports, foreground sleeps,
and autonomous-run journal events. A skill load is a successful trace record,
not a catalog mention. A foreground sleep waits more than five seconds outside
a condition loop; background waits do not count. Gate denials must appear as
error results, not quoted source text. Count oracle, continuation, blocked,
holdout and high-risk decision events from project journals separately.
See [learning loop](learning-loop.md).

Keep transcript bodies local. Redact emails, tokens, home paths and URL
credentials from excerpts, then inspect for missed secrets. Only user-reviewed,
sanitized scorecards may leave the machine. Hash workspace paths, omit run IDs
and fold unrelated skill names into other. Never share raw transcripts or
unreviewed excerpts in reviews, issues or commits.
