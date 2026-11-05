# Security

megapowers is not a security boundary. It installs model instructions, bundled Go
tools, and an optional external-review tool into the agent's permission context.
Review the exact revision before installing it.

## Installed capabilities

| Component | Reads | Writes | Network |
|---|---|---|---|
| Skills | Repository and task context selected by the harness | Only what the active agent is authorized to change | No direct network client |
| Doctor script | Plugin manifests, optional skill catalog and listed skill bodies, shipped agent registry template | Summary on standard output | Script: none; `go run` may download a selected Go toolchain |
| Session-mine tool (repository-only) | Local Claude Code and Codex transcripts | Reports on standard output | None |
| Memory-audit tool | One explicit audit manifest | Standard output only | None |
| Independent-review tool | One explicit repository file or immutable commit range | Private advisory receipt, plus transcript only when requested | One operator-named reviewer command after token approval |

There is no daemon, model router, formatter, status line, or background
scheduler.

The doctor reads only files inside the supplied plugin root: the Claude and
Codex manifests, the optional skill catalog and listed skill bodies, and the
shipped agent registry template. It prints versions, catalog counts, paths, and
warnings. Root environment variables only locate that tree.

The script has no network client. `go run` uses Go's build caches; with
`GOTOOLCHAIN=auto`, Go may download a selected toolchain. Set
`GOTOOLCHAIN=local` to require a suitable locally installed toolchain.

The memory-audit tool checks provenance, dates, retention decisions, and
common credential patterns. It rejects symlink inputs and never edits provider
memory. Pattern checks cannot tell whether a claim is true or current, so the
memory-hygiene reference in `writing-agent-instructions` requires inspecting
the source and one exact approval. After approval, the active agent applies
that patch through the harness memory boundary and reads every target back to
verify it.

## Enforcement boundary

The plugin supplies no destructive-command guard or command gates. Use the
harness sandbox, OS permissions, least-privilege credentials, backups, and
explicit review as the controls.

`safe-effects` covers deploys, messages, charges, migrations, destructive
queries, DNS changes, and other changes outside the machine. It is model
guidance, not enforcement.

## Independent-review disclosure

The review tool accepts only one explicit file or one immutable commit range.
It does not infer the dirty worktree or include untracked files. Before an
external call it prints the provider, source identity, paths, file count, byte
count, package hash, and any `--intent` text, then requires
`--approve-external`. The intent is sent verbatim and bound to the token.

It rejects:

- author and reviewer providers that match;
- provider binaries resolved inside the repository;
- symlinks, submodules, non-text files, and oversized packages;
- secret-like paths and common credential patterns;
- project routing configuration and unrestricted environment forwarding.

The `independent-review` skill treats a reviewer bound in the operator's
capability registry, or named in user or repository instructions, as standing
approval: the agent dispatches with `--approve-external configured` or inspects
and passes the token itself. With no configured reviewer, it asks before
sending. Delete the binding to require approval for every review.

Pattern matching cannot identify every secret. Inspect the disclosure and the
source itself before dispatch. Raw transcripts are not retained by default.
Receipts are advisory records, not signatures or tamper-proof attestations.

After approval validation, the tool runs a private read-only copy of a native
reviewer binary whose bytes match the approved hash, or verifies the SHA-256 of
a script launcher outside the repository before and after execution. An
explicit receipt output directory must already exist at an absolute canonical
path that neither overlaps nor contains the repository. Writes are rooted at an
opened directory handle; the default stays under Git metadata.

See [docs/advanced/independent-review.md](./docs/advanced/independent-review.md)
for the exact workflow.

## Prompt injection and supply chain

Every `SKILL.md`, reference, and repository instruction is an instruction
channel. A malicious repository or dependency can place text in the agent's
context. Treat installed plugin revisions, reviewed repositories, browser
content, issue text, and generated artifacts as untrusted input.

This repository's security lint scans the full installable tracked and
nonignored untracked tree. It rejects documented executable-fetch, obfuscation,
directional-control, and disable-safety patterns unless a narrow file-level
allowlist explains a necessary fixture. That scan reduces accidental exposure;
it does not prove an instruction safe.

Before installation:

1. Inspect both plugin manifests and bundled scripts.
2. Read every skill likely to run in your environment.
3. Run `scripts/security-lint.sh` and `scripts/validate.sh`.
4. Verify the selected Git revision and use a pinned checkout when update
   timing matters.

## Credentials and artifacts

The installed-plugin A/B study (the credentialed evaluation in `evals/`) never
copies credentials into home directories visible to the evaluated agent (the
actor) or launches a provider directly. Real runs require a reviewed,
hash-pinned broker that handles authentication outside an attested OS
isolation boundary. A missing, mismatched, or overbroad attestation fails
closed. Publish bundles contain only sanitized result rows and manifests, not
credentials, raw prompts, responses, transcripts, repositories, or absolute
paths. Inspect them before sharing.

Broker paths must be absolute, canonical, free of symlinks, outside
actor-visible and output trees, and point to a self-contained executable. Each
invocation runs a private read-only copy whose bytes match the pinned hash.

Do not store credentials in repository configuration, fixtures, eval case
manifests, or review receipts.

## Report a vulnerability

Use GitHub Private Vulnerability Reporting for a secret disclosure, unsafe
package capture, or another finding with real blast radius. A public issue is
appropriate for a documentation defect.

Include the exact revision, Claude Code or Codex version, input, expected
decision, actual decision, and the smallest safe reproduction.
