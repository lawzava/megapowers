# mega-guardrails

A small cross-harness safety and convenience plugin. Codex and Claude Code get
a tripwire for a short list of destructive shell commands. Claude Code also
gets a formatter after file writes. A Linux statusline script is included for
Claude Code and must be enabled by hand.

## deny-destructive (PreToolUse, Bash)

A high-confidence accident tripwire that classifies instead of
blanket-blocking:

- Denies a short list of catastrophic, unrecoverable commands: recursive `rm`
  of `/`, `~`, `$HOME`, or a top-level system dir; `mkfs`; `dd` to a block
  device; `chmod 777 /`; redirect to a raw disk; fork bomb.
- Asks (surfaces a confirmation) for reversible-but-risky local ones:
  `git reset --hard` / `clean -f` / `branch -D` / `push --force`; a remote
  download piped into a shell (`curl ... | bash`). Remote destructive ops
  (cloud deletes, `terraform destroy`, `kubectl delete --all`) are not
  pattern-matched: real-world effects belong to the effect-broker skill.
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

### Relationship to native protections

Claude Code gained its own destructive-command blocking in v2.1.183: in auto
mode it blocks destructive git operations (`git reset --hard`, `git checkout
-- .`, `git clean -fd`, `git stash drop`, and `git commit --amend` on commits
it did not create) and infrastructure teardown (`terraform`/`pulumi`/`cdk
destroy`) unless you asked for them. `deny-destructive` does not replace or
duplicate that. It adds two things on top:

- **Non-git, non-infrastructure destructive families** the native checks do not
  cover: recursive `rm` of `/`, `~`, `$HOME`, or a top-level system dir; `mkfs`;
  `dd` to a block device; `chmod 777 /`; a redirect to a raw disk; a fork bomb.
- **A portable, mode-independent pattern.** It fires on every Bash call, not
  only in auto mode, and it is a plain stdin-to-stdout script, so the same
  classification can run wherever a harness lets a hook see the command.

Real containment is still the runtime sandbox, not this hook. Claude Code's
`/sandbox` filters network egress through a local proxy and confines Bash child
processes; its own documentation is explicit that this reduces risk rather than
delivering complete isolation (a broad domain allowlist can become an
exfiltration path, and `Read`/`Edit`/`Write` are governed by the permission
system, not the sandbox). Treat `/sandbox` plus OS permissions as the boundary
and `deny-destructive` as an accident tripwire in front of it.

The default hook manifest dispatches to `codex-deny-destructive.sh` when Codex
loads the plugin and to the full guard under Claude Code. Codex cannot surface
the guard's `ask` result, so its adapter preserves catastrophic denies and lets
Codex's normal approval flow handle the reversible-risk tier. OpenCode and
Antigravity still receive no hook behavior from this plugin.

## auto-format (PostToolUse, Write/Edit)

Formats the file that was just touched: `gofmt`/`goimports` for Go, and a
project-local `prettier` for JS/JSX, TS/TSX, JSON, CSS/SCSS, Markdown, and
YAML. Runs
synchronously (so it can't race a follow-up edit) and only rewrites the single
file just written.

## Prerequisites

- `jq` is required by both hooks.
- `goimports` and `gofmt` are used for Go formatting.
- A project-local `prettier` is used for JS/JSX and TS/TSX (and JSON, CSS/SCSS,
  Markdown, YAML).

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

Claude Code: `/plugin install mega-guardrails@megapowers`.

Codex: `codex plugin add mega-guardrails@megapowers`.
