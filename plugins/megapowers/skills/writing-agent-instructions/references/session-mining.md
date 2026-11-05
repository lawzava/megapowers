# Session mining

Past sessions show how work really goes. Places where the user stepped in
to correct the agent are the best evidence for a new check, rule, or skill.
`scripts/session-mine.go` reads Claude Code and Codex transcript files and
OpenCode V2 sessions from offline JSON exports, so a retrospective can
cover many sessions, not one. This does not add an OpenCode plugin install target.

It needs Go, a shell, and local transcripts. Without them, say so and ask the
user to paste the corrections or sessions to review; do not reconstruct them
from memory.

Use it when:

- a retrospective needs to know if a mistake repeats across sessions;
- the user asks what was decided or built earlier on a topic.

## Commands

`SKILL_DIR` is this skill's directory; `--workspace` names the project:

```bash
go run "$SKILL_DIR/scripts/session-mine.go" corrections --workspace "$PWD" --since 2026-10-01
go run "$SKILL_DIR/scripts/session-mine.go" recall --topic "release script" --workspace "$PWD"
go run "$SKILL_DIR/scripts/session-mine.go" scorecard --workspace "$PWD" --since 2026-10-01
```

Shared flags: `--harness claude|codex|opencode|both|all` (default `all`), `--format
text|json`, `--limit N` (examples per class, default 5; sessions for recall,
default 10), `--all-workspaces` instead of `--workspace`, and `--claude-root`
or `--codex-root` to override the transcript locations.
By default, Codex scans both `sessions` and `archived_sessions` under
`CODEX_HOME` (or `~/.codex`), counting each rollout filename once;
`--codex-root` scans only the given directory, including archives only when
pointed there.

Both `both` and `all` cover Claude Code and Codex. With
`--opencode-export PATH`, `all` also reads OpenCode exports; without it,
OpenCode is noted as not read. `both` always means Claude Code + Codex.
Each report includes `by_source` counts, `skipped_sources`,
`partial_sources`, `skipped_sessions`, and `deduplicated_sessions`.
Invalid export files make the source partial, retaining valid sessions.
Exports with an `export-status` manifest retain its status and skipped-session
count. The manifest's `environment` is a label copied as is: `offline`,
`inherited`, or `without_xdg_state_home`. Without a manifest, `environment` is
`offline`; status is `complete`, `partial`, or `skipped`.

`--opencode-export PATH` reads OpenCode session export JSON files (a file or a directory) produced outside the plugin.
Each file is one JSON object with this shape (`messages` contains message objects):

```json
{"data":{"info":{"id":"session-id","parentID":"","location":{"directory":"/path/to/workspace"}},"messages":[]}}
```

`data`, a nonempty `info.id`, and the `messages` array are required.
`parentID` is optional; workspace matching requires `location.directory`.
Session IDs are counted once. `--workspace` matches the export's current `location.directory`,
including child sessions; `--since` filters message timestamps.

OpenCode `user` messages are human turns unless a machine-text anchor below
matches. `synthetic` messages are machine turns; child sessions (`parentID`
set) classify user/synthetic turns as subagent prompts, never human. Assistant
text is kept; reasoning, system text, and tool outputs are not conversation
excerpts. Skill loads and shell/read/error tool events feed scorecards.
The V2 format has no standard scheduler/Paseo-origin field: unmarked scripted root
prompts cannot be distinguished from typed prompts and still count as human.

Claude Agent exports have `model.providerID: claude-agent` and content blocks
with `native.sessionID`, observed to match Claude JSONL `sessionId`. When both
sources are selected and the OpenCode export contributes in-range messages,
the linked Claude transcript is omitted. Other `sdk-ts` transcripts stay.
If that link is absent, or OpenCode cannot be read, no heuristic dedupe is
attempted; per-source counts can overlap. Inspect those counts before treating
combined totals as unique activity.

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

Machine-written user turns are not the user's words. The tool leaves them out of
`human_turns` and every class, and reports their count as `machine_turns`. It
excludes:

- Claude compaction summaries (`isCompactSummary`) and turns whose `origin` names
  a producer, such as a `<task-notification>`;
- Codex turns in a `codex exec` run (`source: exec`);
- in any source, a turn that starts with "This session is being continued
  from a previous conversation", "This is an approved autonomous run", or
  "Continue the approved run and finish it", or that has a line starting with
  "Unattended run:" (a schedule prompt).

Subagent prompts and meta records were already excluded. An SDK caller such as
Paseo sends typed and scripted prompts the same way, so a scripted prompt with
none of these anchors still counts as a human turn.

`recall` lists the sessions where one message contains every topic word, most
recent first, with the first prompt and the last reply. When rebuilding context
for a topic from past sessions, default to the last 14 days and this workspace;
widen only when the user asks, and state which scope you used. Open the
relevant transcripts to read their decisions, open questions, and last state,
then check live repository and forge state (recent commits, branches, and any
named pull request or issue) before reporting. Report a short brief: what the
work was; each thread with a status tag such as `[merged #12]`, `[open PR #14]`,
`[abandoned]`, or `[unknown]`; at most five open problems; and the next move.

`scorecard` counts outcome signals per project: skill loads and the
corrections that followed them, historical megapowers gate blocks (zero in hook-free versions), `VERIFIED` and `NOT
VERIFIED` reports, foreground `sleep` calls, and autonomous-run journal
events. `human_turns` and `corrections` leave out machine turns, which show as
`machine_turns`. A call counts as a foreground sleep when a shell command in it
runs `sleep` for more than 5 seconds, outside any `until`, `while`, or `for`
loop, and not backgrounded. A call with `run_in_background`, a sleep of 5
seconds or less, and the word "sleep" in an argument or string do not count.
A sleep that runs in the foreground waits on a guessed delay instead of an
event. A gate counts only when its denial opens an error result, so reading
the gate's source does not count. `--all-workspaces` adds a per-workspace
table. `--sanitize`, valid only with `scorecard`, hashes workspace paths,
drops run IDs, and folds non-megapowers skill names into `other`. The
[learning loop](learning-loop.md) uses the JSON form.

## Reading the results

Keyword classes are candidates, not findings. "Continue" may be a plain
instruction, and a real correction may use none of the keywords. Read each
example, opening the transcript if the excerpt is unclear, before counting
it toward a retrospective class.

## Privacy

The tool runs locally and never writes files. Only a sanitized scorecard
may leave the machine, and only after the user has seen it. Excerpts are short and redacted:
emails, tokens, home paths, and URL credentials are removed. Redaction can
miss things. Never paste raw transcripts or unreviewed excerpts into
a review package, issue, commit, or other public text.
