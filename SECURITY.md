# Security

megapowers is not a security boundary. It installs model instructions and bundled Go
tools into the agent's permission context.
Review the exact revision before installing it.

## Installed capabilities

| Component | Reads | Writes | Network |
|---|---|---|---|
| Skills | Repository and task context selected by the harness | Only what the active agent is authorized to change | No direct network client |
| Doctor script | Plugin manifests and catalog, local marketplace manifests, Claude settings, optional agent registry, workspace Git root, local Claude and Codex session records | Summary on standard output | Script: none; `go run` may download a selected Go toolchain |
| Memory-audit tool | One explicit audit manifest | Standard output only | None |
| Independent-review guidance | Context selected by the active agent | Review brief and findings through existing harness tools | Selected review route under user permissions; no bundled launcher |

There is no daemon, model router, formatter, status line, or background
scheduler.

The doctor reads `~/.claude/settings.json` and project `.claude/settings.json`
and `.claude/settings.local.json` (parsing only `outputStyle`), the optional
agent registry to check its expiry, and local marketplace manifests to compare
versions. With a skill catalog, it reads recent local Claude transcripts in
the workspace's project directory, including subagents. Under
`~/.codex/sessions`, it scans rollout headers from today and the previous 14
date directories, examining at most 2,000 headers and collecting at most 500
transcripts across both harnesses. Environment overrides can select other
local settings and session directories. It counts sessions modified in the
last seven days that contain a successful skill-body load and prints counts,
paths, versions, style values, and warnings; it does not print transcript
bodies. The directory edition omits the catalog, so it skips load counting. It
runs `git rev-parse --show-toplevel` locally to resolve the workspace root.

The script does not edit settings or transcripts and has no network client.
Running it with `go run` uses Go's build caches; with `GOTOOLCHAIN=auto`, Go
may download a newer toolchain when selected by the active module or
workspace. Use `GOTOOLCHAIN=local` with an installed suitable Go version when
a local toolchain is required.

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

Independent review is guidance only. No bundled review script launches provider
CLIs or accesses their saved logins. The active agent chooses another available
model through existing harness review, native subagents, or installed agent
CLIs. Cross-vendor review is preferred but optional. The user's global
CLAUDE.md or AGENTS.md defines independence and reviewer preferences.

Before sending anything off-machine, inspect the brief and accessible files
for secrets and disclosure scope. Follow existing user authorization and
harness permissions. Native review can also send data off-machine. The skill
provides no automated secret scan, package hashing, or filesystem sandbox.

See [docs/advanced/independent-review.md](./docs/advanced/independent-review.md)
for reviewer selection, the return contract, and correction rounds.

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
