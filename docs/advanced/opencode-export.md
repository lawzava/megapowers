# OpenCode session export

The repository tool creates offline JSON files for the installed session miner:

```sh
go run ./scripts/cmd/opencode-export --output /path/to/exports
go run "$SKILL_DIR/scripts/session-mine.go" corrections --workspace "$PWD" --opencode-export /path/to/exports
```

Run the exporter from a repository checkout. It is outside the shipped plugin.
The output directory must be new or empty to avoid mixing snapshots. Files use
mode 0600 and contain unredacted transcripts; the miner redacts report excerpts.
The exporter writes a JSON summary to stdout, diagnostics to stderr, and exits
1 for an incomplete export (retaining files already written), or 2 for invalid
options. An empty, complete snapshot exits 0.

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
