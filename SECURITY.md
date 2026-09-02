# Security

megapowers is not a security boundary. It installs model instructions, shell
hooks, and an optional external-review tool into the agent's permission context.
Review the exact revision before installing it.

## Installed capabilities

| Component | Reads | Writes | Network |
|---|---|---|---|
| Skills | Repository and task context selected by the harness | Only what the active agent is authorized to change | No direct network client |
| Codex output-style hook | Bundled static style | Developer context on standard output | None |
| Destructive-command hook | Proposed shell command from hook input | Hook decision on standard output | None |
| Memory-audit tool | One explicit audit manifest | Standard output only | None |
| Independent-review tool | One explicit repository file or immutable commit range | Private advisory receipt, plus transcript only when requested | One operator-named reviewer command after approval |

There is no daemon, model router, formatter, status line, or background
scheduler.

The memory-audit tool validates provenance, dates, retention decisions, and
common credential patterns. It rejects symlink inputs and never edits provider
memory. Pattern checks cannot establish that a claim is true or current; the
memory-hygiene skill requires source inspection and one exact approval. After
approval, the active agent applies that patch through the harness memory
boundary and verifies every target by readback.

## Destructive-command guard

The hook catches a narrow set of obvious catastrophic commands. It uses
command-string parsing for precision, not evasion resistance.

One matcher covers the Bash and PowerShell tools, and both receive the same
high-confidence denials: the PowerShell tool hands over the same
`tool_input.command` field, so `Remove-Item -Recurse /` and the cmd.exe
`rd /s /q C:\` classify under the same rules as `rm -rf /`.

Matching PreToolUse `deny` decisions are applied by Claude Code's decision
control before the call runs, so they survive `bypassPermissions` and
`--dangerously-skip-permissions`: bypass mode removes permission prompts, not
hook evaluation. PreToolUse fires on every tool call and a `deny` cancels the
call, including in modes that skip other prompts.

- Claude Code and Codex receive the same high-confidence denials.
- Reversible risk stays with each harness's native permission system.
- A hook evaluation error is visible and nonzero. Do not treat a broken hook as
  protection.

### What stays allow by design

The guard is a tripwire for plausible accidents, not an obfuscation filter.
Everything below is deliberate.

Allowed by design (reversible, scoped, or owned by the harness permission
system):

- Scoped deletes and cleanup: `rm -rf ./dist`, `/tmp/app/*`,
  `/etc/nginx/conf.d/*`, `~/.cache/foo`, `~alice/Code/build`.
- Reversible version-control operations (`git reset --hard`, `git clean -fdx`,
  `git push --force`), cloud and cluster deletions (`terraform destroy`,
  `kubectl delete pods --all`, `aws s3 rb`), and container cleanup
  (`docker system prune`).
- Device-gated uses against plain files (`mkfs.ext4 disk.img`,
  `shred secret.txt`, `truncate -s 0 disk.img`), reads from devices, and
  writes to character devices (`dd of=/dev/null`, `cp /dev/sda ./backup.img`).
- Account and access-control changes (`userdel`, `useradd -G sudo`,
  `setfacl`, `visudo`), locally or over ssh, and other remote effects.

Known bypasses (deliberately left uncovered; chasing them with more regex is a
losing game the project does not run):

- Obfuscated spellings: encoded, aliased, or escaped commands, command
  substitution, heredoc-fed shells, double-nested `bash -c` with escaped
  inner quotes.
- Variable indirection: the guard never expands variables, so
  `DEV=/dev/sda; dd of=$DEV` is invisible to it.
- Wrapper option-values it does not model, such as long-form options with a
  separate value (`xargs --replace X rm -rf /home`), and payloads assembled at
  runtime.
- Windows paths below a drive root: `Remove-Item -Recurse C:\Users` and named
  Windows system directories stay unclassified. Only a recursive flag plus a
  POSIX root, home, or system target, or a bare drive root (`C:\`), denies.
- The brace-default spelling of a home parent (`rm -rf "${HOME:-/}/.."`) and
  a named path under a home parent (`rm -rf $HOME/../foo`).
- Payloads handed to non-shell interpreters (`find . -exec python3 -c ...`)
  and destroyers the find tier does not name.

The guard can miss obfuscated, aliased, encoded, generated, or indirect
commands. It can also reject harmless text that resembles a destructive
command. Use the harness sandbox, OS permissions, least-privilege credentials,
backups, and explicit review as the real controls.

`safe-effects` covers deploys, messages, charges, migrations, destructive
queries, DNS changes, and other external mutations. It is still model guidance,
not enforcement.

## Independent-review disclosure

The review tool accepts only one explicit file or one immutable commit range.
It does not infer the dirty worktree or include untracked files. Before an
external call it prints the provider, source identity, paths, file count, byte
count, and package hash, then requires `--approve-external`.

It rejects:

- author and reviewer providers that match;
- provider binaries resolved inside the repository;
- symlinks, submodules, non-text files, and oversized packages;
- secret-like paths and common credential patterns;
- project routing configuration and unrestricted environment forwarding.

Pattern matching cannot identify every secret. Inspect the disclosure and the
source itself before approval. Raw transcripts are not retained by default.
Receipts are advisory records, not signatures or tamper-proof attestations.
After token validation, the tool executes a private read-only copy whose bytes
match the approved provider hash rather than the mutable provider pathname.
Explicit receipt output must already exist at an absolute canonical path that
neither overlaps nor contains the repository. Writes are rooted at an opened
directory handle; the default remains under Git metadata.

See [docs/advanced/independent-review.md](./docs/advanced/independent-review.md)
for the exact workflow.

## Prompt injection and supply chain

Every `SKILL.md`, reference, hook, and repository instruction is an instruction
channel. A malicious repository or dependency can place text in the agent's
context. Treat installed plugin revisions, reviewed repositories, browser
content, issue text, and generated artifacts as untrusted input.

This repository's security lint scans the full installable tracked and
nonignored untracked tree. It rejects documented executable-fetch, obfuscation,
directional-control, and disable-safety patterns unless a narrow file-level
allowlist explains a necessary fixture. That scan reduces accidental exposure;
it does not prove an instruction safe.

Before installation:

1. Inspect both plugin manifests and `plugins/megapowers/hooks/`.
2. Read every skill likely to run in your environment.
3. Run `scripts/security-lint.sh` and `scripts/validate.sh`.
4. Verify the selected Git revision and use a pinned checkout when update
   timing matters.

## Credentials and artifacts

Installed A/B and PR replay never copy credentials into actor-visible homes or
launch a provider directly. Real runs require a reviewed, hash-pinned broker
that owns authentication outside an attested OS isolation boundary. A missing,
mismatched, or overbroad attestation fails closed. Publish bundles contain only
sanitized result rows and manifests, not credentials, raw prompts, responses,
transcripts, repositories, or absolute paths. Inspect them before sharing.

Broker paths must be absolute, canonical, free of symlinks, outside actor-visible
and output trees, and point to a self-contained executable. Each invocation runs
a private read-only copy whose bytes match the pinned hash.

Do not store credentials in repository configuration, fixtures, eval case
manifests, or review receipts.

## Report a vulnerability

Use GitHub Private Vulnerability Reporting for a command that can bypass a
high-confidence denial, a secret disclosure, unsafe package capture, or another
finding with real blast radius. A public issue is appropriate for a harmless
false positive or documentation defect.

Include the exact revision, Claude Code or Codex version, input, expected
decision, actual decision, and the smallest safe reproduction.
