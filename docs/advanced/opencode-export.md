# OpenCode session export

The repository tools export and mine OpenCode sessions outside the shipped plugin:

```sh
go run ./scripts/cmd/opencode-export --all-workspaces --output /path/to/exports
go run ./scripts/cmd/opencode-mine corrections --workspace "$PWD" --opencode-export /path/to/exports
```

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

OpenCode user messages count as human unless a machine-text anchor matches;
synthetic messages count as machine. Child-session user and synthetic messages
count as subagent prompts. Unmarked scripted root prompts cannot be distinguished
from typed prompts. Assistant text, skill loads, and shell/read/error tool events
feed the reports; reasoning, system text, and tool outputs are not excerpts.
Workspace matching uses location.directory; --since filters message timestamps.
