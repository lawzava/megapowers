# Security

## Scope

Nothing in this repository is a security boundary.

- `mega-guardrails`' `deny-destructive` hook is a tripwire against accidents,
  not a sandbox. It string-matches obviously catastrophic commands and can be
  bypassed by anyone trying. Real containment comes from your runtime's
  sandbox and OS permissions.
- The `effect-broker` skill gates irreversible actions by declared action
  class; a model that misdeclares is not stopped by it.
- Hook manifests ship for Claude Code and Codex. Codex's installed
  mega-guardrails plugin dispatches PreToolUse to the destructive-command
  adapter after a `/hooks` trust decision. Nothing blocks or gates on OpenCode
  or Antigravity.

If your threat model includes a malicious or compromised model, none of these
help; use OS-level sandboxing.

## Indirect prompt injection

This marketplace ships executable instructions. A skill body or reference doc
is text your agent reads and then acts on, so it is an instruction channel, not
inert documentation. Three inputs deserve to be treated as untrusted by
default:

- **The skills and hooks you install.** They run in your agent's full
  permission context. A compromised or careless one is the dominant real-world
  failure mode of 2025-2026: public research that scanned thousands of
  published skills found a minority carrying prompt-injection payloads, and
  separately showed hook and rule files that steered agents into leaking
  environment variables and local secrets. The class matters more than the
  vendor: any file the model reads can carry an instruction.
- **Page and screenshot content the browser delegate reasons over.** A page the
  delegate visits can contain text aimed at the model rather than at you.
- **Repositories a delegate reads.** An untrusted repo's READMEs, comments, and
  config reach the model through the same channel.

What this repo does about it:

- No hook makes a network call. Every hook script reads only stdin, the file
  just written, the session transcript, or local git state, and writes only its
  decision or a local marker. Verify with
  `grep -rnE 'curl|wget|/dev/tcp' plugins/*/hooks`.
- No skill instructs the agent to fetch remote content and run it. Browser and
  delegate work routes through `playwright-cli` and named delegate CLIs, not an
  opaque `curl | sh`.
- `scripts/security-lint.sh` ships in this repo and scans its own skills,
  hooks, and templates for the documented injection markers (a fetch of remote
  content in an executable step, a base64 blob piped into a shell, `eval` of
  fetched content, unicode direction-override characters, and disable-safety
  instructions), failing on a hit. It runs in CI as part of `scripts/validate.sh`
  and can be run locally the same way, or directly with `scripts/security-lint.sh`.

What it cannot do: the harness executes what you trust it with. This repo
cannot stop a model you have told to follow a malicious instruction, and it
cannot vouch for a skill you install from anywhere else. Review before you
install.

## Before you install

These skills and hooks run in your agent's full permission context. Read them
first, the way you would read a shell script before piping it into `bash`:

- Skim each `SKILL.md` you will load and each `hooks/*` script.
- Look for the markers `scripts/security-lint.sh` checks for: a fetch of remote
  content in an executable step (`curl`/`wget` to a URL, especially piped into
  a shell), obfuscated commands (a base64 blob decoded into a shell, unusual
  unicode), and any instruction that tells the agent to disable a sandbox,
  bypass a permission prompt, or ignore its own rules.
- Prefer a pinned install (a marketplace ref to a release tag) over tracking
  `main`, so an upstream change is a version you choose rather than a silent
  update. See `docs/setup.md`, "Pinning to a release".

Capability disclosure. Every hook here is a local shell script that makes no
network call. What each plugin runs:

| Plugin | Hook (event) | Reads / writes | Skills | Network |
| --- | --- | --- | --- | --- |
| `megapowers` | `session-start` (SessionStart) | reads its own `using-megapowers` skill; writes a context string to stdout | process skills (planning, TDD, debugging, review, worktrees, memory) | none |
| `mega-guardrails` | `deny-destructive` (PreToolUse: Bash); `auto-format` (PostToolUse: Write/Edit); cross-harness dispatchers select the Codex destructive adapter and no-op formatter | deny-destructive reads the proposed command on stdin and writes an allow/ask/deny decision; the Codex adapter maps that same decision onto Codex's hook contract; auto-format reads the just-written file path and reformats that one file (`gofmt`/`goimports`/`prettier`) under Claude Code | none (hooks and an optional statusline only) | none |
| `mega-orchestration` | `run-loop`, `delegate-nudge` (both Stop) | both read stdin and the session transcript; run-loop also reads `.megapowers/run/<id>/status`; delegate-nudge also reads `git diff` and writes a one-line marker to `.git/megapowers-delegate-nudge-seen`; both write a stop decision to stdout | orchestration and delegation skills | none |
| `mega-go` | none | reads and writes nothing (skills only) | `golang-patterns`, `greenfield-go-stack` | none |
| `mega-python` | none | reads and writes nothing (skills only) | `python-patterns`, `greenfield-python-stack` | none |
| `mega-ts` | none | reads and writes nothing (skills only) | `typescript-patterns`, `greenfield-ts-stack` | none |

The optional `mega-guardrails` statusline (manual install, Linux only) reads
`/proc`, `df`, `date`, and `git`, and writes only to the statusline; it too
makes no network call.

## Release integrity

Releases are tagged. Pin your install to a tag rather than tracking `main`.
Each tag resolves to an exact commit you can verify with
`git ls-remote --tags https://github.com/lawzava/megapowers`, `git tag`, or
`gh release view <tag>`. Release tags from v0.1.3 on are GPG-signed: verify one
with `git tag -v <tag>` (the v0.1.1 and v0.1.2 tags predate signing and are
stable inspectable refs, not cryptographic provenance).

## Reporting

For anything with real blast radius (the deny-destructive suite passing a
command that wipes a disk, a hook that fails closed and locks a session, or a
skill or hook that leaks user data), the preferred channel is GitHub Private
Vulnerability Reporting: the repository's Security tab, "Report a
vulnerability". For non-sensitive tripwire gaps a GitHub issue is fine; anything
with real blast radius should go through the private channel.

None of this repo's components hold secrets or run as services, so coordinated
disclosure is often unnecessary. Please include the exact command/input, the
expected vs actual behavior, and your harness (Claude Code / Codex / OpenCode /
Antigravity + version).
