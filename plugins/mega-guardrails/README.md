# mega-guardrails

A small safety and convenience plugin for Claude Code. It wires two hooks that run
automatically: one is a high-confidence tripwire for a handful of shell commands, and
one formats files right after they are written. A Linux statusline script is also
included, but it is optional and must be enabled by hand.

## Auto-wired hooks

| Hook | Event | Matcher | What it does |
| --- | --- | --- | --- |
| deny-destructive | PreToolUse | Bash | A small, high-confidence accident tripwire. It **denies** a short list of catastrophic, unrecoverable commands (recursive `rm` of `/`, `~`, `$HOME`, or a top-level system dir; `mkfs`; `dd` to a block device; `chmod 777 /`; redirect to a raw disk; fork bomb) and **asks** (surfaces a confirmation) for reversible-but-risky ones (`git reset --hard` / `clean -f` / `branch -D` / `push --force`; `aws s3 rm --recursive` / `rb --force`; `docker prune -f`; `terraform destroy -auto-approve`; `kubectl delete --all`; a remote download piped into a shell, `curl … \| bash`). It **allows** ordinary scoped work (`rm -rf ./dist`, `rm -rf "$TMPDIR/x"`, API-key `curl`, dry-runs). It is **not** a sandbox or security boundary — determined obfuscation gets past it, it does not try to catch secret exfiltration (that's the sandbox's job), and real containment comes from the sandbox and permission system. Fails open on any internal error. |
| auto-format | PostToolUse | Write, Edit | Formats the file that was just touched: `gofmt`/`goimports` for Go, and a project-local `prettier` for JS, TS, CSS, Markdown, and YAML. Runs synchronously (so it can't race a follow-up edit) and only rewrites the single file just written. |

## Prerequisites

- `jq` is required by both hooks.
- `goimports` and `gofmt` are used for Go formatting.
- A project-local `prettier` is used for JS and TS (and CSS, Markdown, YAML).

Missing formatters are skipped quietly rather than treated as errors.

## Statusline (optional, manual)

The statusline is not auto-wired by the plugin. To enable it, copy `statusline.sh` to a
stable path and reference it from `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "<path>/statusline.sh"
  }
}
```

It is Linux-only, since it reads `/proc`, and uses `df -P` and `date -d`. It renders the
current folder, git branch, git diff shortstat, model and effort, context percentage,
memory/cpu/disk usage, and 5-hour and weekly rate-limit usage.

## Install

```
/plugin install mega-guardrails@megapowers
```

### Standalone skills

None. mega-guardrails ships only hooks and the statusline, so there are no
standalone skill entries to install separately.
