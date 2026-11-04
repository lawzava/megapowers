# Session mining

Past sessions show how work really goes. Places where the user stepped in
to correct the agent are the best evidence for a new check, rule, or skill.
`scripts/session-mine.go` reads Claude Code and Codex transcript files and
OpenCode V2 sessions through its CLI API, read-only, so a retrospective can
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

`both` remains Claude Code + Codex; `all` adds OpenCode when available.
Each report includes `by_source` session, human, machine, and subagent-prompt
counts, plus `skipped_sources`, `partial_sources`, `skipped_sessions`, and
`deduplicated_sessions`. An unavailable OpenCode CLI/service is skipped with
a stderr note, not a failed run. Failed exports, later-page failures, and the
overall deadline make OpenCode partial, retaining sessions already loaded.
A failed export also increments `skipped_sessions`. The OpenCode source's
`status` is `complete`, `partial`, or `skipped`; its `environment` records the
selected CLI environment.

### OpenCode API access

The adapter checks `opencode service status`, then uses only `opencode api get`
against `/api/session` (all cursor pages) and
`/api/experimental/session/{sessionID}/export`. It issues no service start,
stop, or restart commands, and opens no OpenCode files directly.
Use an already-running service. Status calls have a 10-second
timeout; API calls have a 30-second timeout. `WaitDelay` bounds leftover child
pipes to one additional second. Timed-out attempts are not retried.
The whole OpenCode source has a 5-minute deadline, including status, retries,
and exports. Set `--opencode-timeout 2m` or `SESSION_MINE_OPENCODE_TIMEOUT`
to a positive Go duration to change it; the flag wins. A deadline produces a
partial source even if no sessions have loaded, and scanning other sources continues.
API output goes to a private temporary file under `TMPDIR` (removed after
reading), since some CLI versions truncate piped output. Missing or malformed
API JSON is retried once, then reported without raw CLI diagnostics.
An empty list/export is also retried once for location warm-up. A valid empty
snapshot is retained if the retry fails, unless the attempt times out.
A persistently empty list may mean no sessions or the wrong server/environment.

`--workspace DIR` selects sessions whose current `location.directory` equals
the resolved directory, including children, not every session sharing a project
ID. Moved sessions are attributed to their current session directory.
`--all-workspaces` has no directory filter. `--since` filters message timestamps;
sessions last updated before that date need no export.

`--opencode-cmd FILE` or `SESSION_MINE_OPENCODE_CMD` selects an executable
(not a shell command); the flag wins. The child receives only set values of
`PATH`, `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `XDG_CONFIG_HOME`,
`XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`, `XDG_RUNTIME_DIR`, and
`TMPDIR`. The platform may also add its required system variable.
It receives `service status` or
`api get PATH` arguments. A wrapper can select the correct CLI environment or
server without changing the miner. If status reports no running service and
`XDG_STATE_HOME` is nonempty, the adapter retries status with that variable
removed from the child environment, then uses that same environment for all
API calls. The source summary reports `environment: without_xdg_state_home`
and stderr notes the fallback. Otherwise it reports `environment: inherited`.
No parent environment is changed and no state path is hardcoded.
For other environment conflicts, a wrapper or an explicit caller override
can select the intended running service:

```bash
env -u XDG_STATE_HOME go run "$SKILL_DIR/scripts/session-mine.go" corrections --workspace "$PWD" --harness opencode
```

OpenCode `user` messages are human turns unless a machine-text anchor below
matches. `synthetic` messages are machine turns; child sessions (`parentID`
set) classify user/synthetic turns as subagent prompts, never human. Assistant
text is kept; reasoning, system text, and tool outputs are not conversation
excerpts. Skill loads and shell/read/error tool events feed scorecards.
The V2 API has no standard scheduler/Paseo-origin field: unmarked scripted root
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
corrections that followed them, megapowers gate blocks (written by the plugin's hooks, so zero without them), `VERIFIED` and `NOT
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

The tool runs locally and keeps no transcript copies beyond its temporary API
output files, which it removes after reading. Only a sanitized scorecard
may leave the machine, and only after the user has seen it. Excerpts are short and redacted:
emails, tokens, home paths, and URL credentials are removed. Redaction can
miss things. Never paste raw transcripts or unreviewed excerpts into
a review package, issue, commit, or other public text.
