# Session mining

Past sessions show how work really goes. Places where the user stepped in
to correct the agent are the best evidence for a new check, rule, or skill.
`scripts/session-mine.go` reads local Claude Code and Codex transcripts,
read-only, so a retrospective can cover many sessions, not one.

Use it when:

- a retrospective needs to know if a mistake repeats across sessions;
- the user asks what was decided or built earlier on a topic.

## Commands

`SKILL_DIR` is this skill's directory; `--workspace` names the project:

```bash
go run "$SKILL_DIR/scripts/session-mine.go" corrections --workspace "$PWD" --since 2026-10-01
go run "$SKILL_DIR/scripts/session-mine.go" recall --topic "release script" --workspace "$PWD"
```

Shared flags: `--harness claude|codex|both` (default `both`), `--format
text|json`, `--limit N` (examples per class, default 5; sessions for recall,
default 10), `--all-workspaces` instead of `--workspace`, and `--claude-root`
or `--codex-root` to override the transcript locations.
By default, Codex scans both `sessions` and `archived_sessions` under
`CODEX_HOME` (or `~/.codex`), counting each rollout filename once;
`--codex-root` scans only the given directory, including archives only when
pointed there.

`corrections` puts each user turn next to the agent text that came before it
and sorts the turn into a class by keywords:

- `stopped-unnecessarily`: the user had to tell the agent to continue.
- `verification`: the agent claimed success without checking it.
- `shallow`: the user asked for more depth or effort.
- `verbosity`: the answer was too long.
- `overreach`: the agent changed something nobody asked it to change.
- `frustration`: the user repeated a complaint or showed anger.

A class that appears only once is listed as a one-off. A class counts after
two occurrences.

`recall` lists the sessions where one message contains every topic word, most
recent first, with the first prompt and the last reply. When rebuilding context
for a topic from past sessions, default to the last 14 days and this workspace;
widen only when the user asks, and state which scope you used. Open the
relevant transcripts to read their decisions, open questions, and last state,
then check live repository and forge state (recent commits, branches, and any
named pull request or issue) before reporting. Report a short brief: what the
work was; each thread with a status tag such as `[merged #12]`, `[open PR #14]`,
`[abandoned]`, or `[unknown]`; at most five open problems; and the next move.

## Reading the results

Keyword classes are candidates, not findings. "Continue" may be a plain
instruction, and a real correction may use none of the keywords. Read each
example, opening the transcript if the excerpt is unclear, before counting
it toward a retrospective class.

## Privacy

The tool runs locally and never writes files. Excerpts are short and redacted:
emails, tokens, home paths, and URL credentials are removed. Redaction can
miss things. Never paste raw transcripts or unreviewed excerpts into
a review package, issue, commit, or other public text.
