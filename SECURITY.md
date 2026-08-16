# Security

megapowers is not a security boundary. It installs model instructions, shell
hooks, and an optional external-review tool into the agent's permission context.
Review the exact revision before installing it.

## Installed capabilities

| Component | Reads | Writes | Network |
|---|---|---|---|
| Ten skills | Repository and task context selected by the harness | Only what the active agent is authorized to change | No direct network client |
| Destructive-command hook | Proposed shell command from hook input | Hook decision on standard output | None |
| Independent-review tool | One explicit repository file or immutable commit range | Private advisory receipt, plus transcript only when requested | Selected Claude or Codex provider call after approval |

There is no daemon, model router, formatter, status line, or background
scheduler.

## Destructive-command guard

The hook catches a narrow set of obvious catastrophic commands. It uses
command-string parsing for precision, not evasion resistance.

- Claude Code and Codex receive the same high-confidence denials.
- Reversible risk stays with each harness's native permission system.
- A hook evaluation error is visible and nonzero. Do not treat a broken hook as
  protection.

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
