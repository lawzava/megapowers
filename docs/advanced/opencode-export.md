# OpenCode session export

The repository tools export and mine OpenCode sessions outside the shipped plugin:

```sh
export_dir="$(mktemp -d "${TMPDIR:?set TMPDIR to a disk-backed directory}/opencode-export-XXXXXX")"
go run ./scripts/cmd/opencode-export --all-workspaces --output "$export_dir"
go run ./scripts/cmd/opencode-mine corrections --workspace "$PWD" --opencode-export "$export_dir"
go run ./scripts/cmd/opencode-mine scorecard --workspace "$PWD" --opencode-export "$export_dir"
```

Run session mining only when the user asks for it. Keep reports local, and
delete the export directory after the last report or when a run is
interrupted; nothing removes it for you.

Run the exporter from a repository checkout. It is outside the shipped plugin.
The output directory must be an absolute path outside the checkout, new or empty to avoid mixing snapshots. Files use
mode 0600 and contain unredacted transcripts; the miner redacts report excerpts.
After accepting the output directory, the exporter writes `export-status`
(JSON in a non-.json file) with status, skipped_sessions, and environment.
An initial partial marker survives interruption; the final manifest records
complete, partial, or skipped. The miner reads this manifest for directory
inputs and for individual files beside it. Handwritten exports without a
manifest remain supported and report environment offline.

Exporter exit 1 = partial result: retain files already written and inspect
export-status to distinguish partial from skipped. Exit 2 means invalid
options. A complete snapshot, including an empty one, exits 0. The JSON stdout
summary is printed after export and manifest completion; option, directory,
or manifest-write failures may return without a summary. Diagnostics use stderr.

Select exactly one of --workspace DIR and --all-workspaces. --since YYYY-MM-DD
skips sessions whose list-data update time precedes the date before fetching
exports. The miner still applies its own message-time filter.

The shell entrypoint is equivalent to the Go command. Invoke its absolute path
when running from another directory:

```sh
scripts/opencode-export.sh --all-workspaces --output /path/to/exports
```

The tool checks service status, paginates `/api/session`, and fetches
`/api/experimental/session/{id}/export` using the configured OpenCode CLI.
It never starts a service. Status calls have a 10-second timeout, API calls
30 seconds, and leftover child pipes one additional second. The overall
deadline defaults to 5 minutes. Set `--opencode-timeout DURATION` or
`SESSION_MINE_OPENCODE_TIMEOUT` to a positive Go duration; the flag wins.
Timed-out attempts are not retried. Malformed and empty results are retried
once; a valid empty snapshot survives a failed retry unless it times out.

When the OpenCode server is not the CLI-managed background service, service
status reports `stopped` and the exporter finds nothing. Pass
`--opencode-server URL` to name the server: the exporter then skips the status
check and passes `--server URL` on every CLI call. It still never starts a
service. The URL must use http or https, carry no credentials, and name a
loopback host (`127.0.0.1`, `[::1]`, or `localhost`); any other value exits 2,
because exports hold private transcripts. Example:

```sh
go run ./scripts/cmd/opencode-export --workspace "$PWD" --opencode-server http://127.0.0.1:4096 --output "$export_dir"
```

If the server requires a password, export `OPENCODE_PASSWORD` for this one
command only. With `--opencode-server`, the tool forwards that one variable,
through the environment, to the CLI that talks to the loopback server it was
given. It never puts the password in argv, output, or `export-status`, and
without `--opencode-server` it does not forward it. When the password is
missing or wrong, the skip reason reads "server requires a password; set
OPENCODE_PASSWORD".

`--opencode-cmd FILE` or `SESSION_MINE_OPENCODE_CMD` selects an executable;
the flag wins. Children receive only set PATH, HOME, USERPROFILE, APPDATA,
LOCALAPPDATA, XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME, XDG_CACHE_HOME,
XDG_RUNTIME_DIR, and TMPDIR values. The platform can add a required system
variable. If status reports no running service and XDG_STATE_HOME is set,
the tool retries without that variable and uses the same environment for API
calls. Its summary reports `environment: without_xdg_state_home`; otherwise
it reports `inherited`. The parent environment is unchanged.

The repository miner defaults to `--harness opencode`. It accepts the same
corrections, recall, scorecard, format, topic, limit, since, and sanitize options
as session-mine. It prints the existing per-source report fields, including
by_source, status, environment, skipped_sources, partial_sources,
skipped_sessions, and deduplicated_sessions. It reads offline files only.

To combine sources, run the repository miner with `--harness all` and the same
workspace/date filters used for session-mine. This also reads Claude Code and
Codex transcripts (with optional --claude-root and --codex-root overrides), and
omits Claude transcripts linked by a claude-agent export's native.sessionID
only when the export contributes in-range messages. Other providers' native IDs
do not link Claude transcripts. Without this link, no heuristic deduplication
is attempted.
Use this combined report instead of adding its counts to session-mine totals.
Separate OpenCode-only and session-mine reports can overlap; do not sum them
as unique activity without checking the linked sessions.

The miner reads the unwrapped `{info, messages}` shape of current session
exports and the API's `{data: {info, messages}}` wrapper. The exporter accepts
either response and writes the unwrapped shape.

OpenCode user messages count as human unless a machine-text anchor matches.
User text that starts with `<paseo-system`, `<subagent`, or `<shell` is a
harness notification and counts as machine, as do synthetic messages. A user
message can carry `skills` entries that the harness attached because the text
named a skill. The miner reports them as skill attachments (`attached_skills`
in the scorecard, `skill_attachments` per source), apart from skill loads
through a model tool call, and never reads the attached body. Idle, compaction,
system, and agent-switched records are ignored. Child-session user and synthetic messages
count as subagent prompts. Unmarked scripted root prompts cannot be distinguished
from typed prompts. Assistant text, skill loads, and shell/read/error tool events
feed the reports; reasoning, system text, and tool outputs are not excerpts.
Workspace matching uses location.directory; --since filters message timestamps.

## Follow-up metrics

The scorecard counts operator follow-ups in OpenCode root sessions (sessions
without a parent), overall and per workspace with `--all-workspaces`:

- `follow_ups`: human turns in a root session that come after at least one
  earlier prompt turn. A prompt turn is a human turn or a scheduled or
  unattended prompt, with or without a machine-text anchor, so the operator's
  first reply to a scheduled run is a follow-up. Notifications
  (`<paseo-system`, `<subagent`, `<shell`) and synthetic messages are never
  prompt turns or follow-ups. Child sessions count nothing. Prompt turns are
  tracked before the `--since` filter, so a session opened before the window
  still counts its later turns.
- `nudges`: follow-ups of at most 60 characters after trimming that contain a
  status-check phrase: stuck, still at it/running/doing/going, how's it going,
  how is it going, are you there, are you alive, keep going, go on, try now,
  try again, done?, status?, any update, any progress. `continue` and `retry`
  count only in a follow-up of at most three words, such as "ok continue".
- `close_off_requests`: follow-ups whose text contains `close-off` or
  `close off` as words, or that carry an attached skill with ID `close-off`.
- `nudges_per_100_follow_ups` and `close_off_requests_per_100_follow_ups`: one
  decimal. JSON reports `null` and text reports `n/a` when there are no
  follow-ups.

Text reports add one line, `opencode follow-ups: N, nudges: N (R per 100
follow-ups), close-off requests: N (R per 100 follow-ups)`, and with
`--all-workspaces` a `by workspace, opencode follow-ups` block. With
`--sanitize` the counts stay.

Known limits: the nudge match reads words, not intent. A short request such as
"I got stuck on the form, can you fill it in for me please" counts as a nudge,
and a long status check over 60 characters does not.
