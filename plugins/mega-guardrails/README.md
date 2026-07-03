# mega-guardrails

A small safety and convenience plugin for Claude Code. It wires two hooks that
run automatically: a tripwire for a short list of destructive shell commands,
and a formatter that runs right after a file is written. A Linux statusline
script is also included; it is optional and must be enabled by hand.

## deny-destructive (PreToolUse, Bash)

A high-confidence accident tripwire that classifies instead of
blanket-blocking:

- Denies a short list of catastrophic, unrecoverable commands: recursive `rm`
  of `/`, `~`, `$HOME`, or a top-level system dir; `mkfs`; `dd` to a block
  device; `chmod 777 /`; redirect to a raw disk; fork bomb.
- Asks (surfaces a confirmation) for reversible-but-risky ones:
  `git reset --hard` / `clean -f` / `branch -D` / `push --force`;
  `aws s3 rm --recursive` / `rb --force`; `docker prune -f`;
  `terraform destroy -auto-approve`; `kubectl delete --all`; a remote
  download piped into a shell (`curl ... | bash`).
- Allows ordinary scoped work: `rm -rf ./dist`, `rm -rf "$TMPDIR/x"`,
  API-key `curl`, dry-runs.

The hook is a plain stdin-to-stdout script, so you can run it by hand from a
checkout:

```console
$ echo '{"tool_input":{"command":"rm -rf /"}}' | hooks/deny-destructive.sh
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "recursive rm of a root, home, or system directory. Delete a specific subdirectory instead (e.g. rm -rf ./dist)."
  }
}
$ echo '{"tool_input":{"command":"rm -rf ./dist"}}' | hooks/deny-destructive.sh
$   # no output: allowed
```

It is not a sandbox or security boundary: determined obfuscation gets past it,
and it does not try to catch secret exfiltration. Real containment comes from
the sandbox and permission system (see the repository `SECURITY.md`). Fails
open on any internal error.

## auto-format (PostToolUse, Write/Edit)

Formats the file that was just touched: `gofmt`/`goimports` for Go, and a
project-local `prettier` for JS, TS, CSS, Markdown, and YAML. Runs
synchronously (so it can't race a follow-up edit) and only rewrites the single
file just written.

## Prerequisites

- `jq` is required by both hooks.
- `goimports` and `gofmt` are used for Go formatting.
- A project-local `prettier` is used for JS and TS (and CSS, Markdown, YAML).

Missing formatters are skipped quietly rather than treated as errors.

## Statusline (optional, manual)

The statusline is not auto-wired by the plugin. To enable it, copy
`statusline.sh` to a stable path and reference it from
`~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "<path>/statusline.sh"
  }
}
```

It is Linux-only, since it reads `/proc`, and uses `df -P` and `date -d`. It
renders the current folder, git branch, git diff shortstat, model and effort,
context percentage, memory/cpu/disk usage, and 5-hour and weekly rate-limit
usage.

## Install

```
/plugin install mega-guardrails@megapowers
```

mega-guardrails ships only hooks and the statusline, so there are no
standalone skills to install separately.
