# Harness support

Last reviewed: 2026-09-04 (UTC).

Current stable Claude Code and Codex are the only supported harnesses. Portable
skills may load elsewhere, but this repository does not test or document those
environments.

| Capability | Claude Code | Codex |
|---|---|---|
| Marketplace metadata | `.claude-plugin/marketplace.json` | `.agents/plugins/marketplace.json` |
| Plugin manifest | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` |
| Task skills | Supported | Supported |
| Communication style | Selectable plugin output style | Trusted startup hook with environment opt-out |
| Destructive-command guard | High-confidence denies only | High-confidence denies only |
| Native agents and parallel work | Direct agents; native team/task coordination when available | Direct agents; native team/task coordination when available |
| Personal capability registry | Advisory, read on demand | Advisory, read on demand |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Any operator-named reviewer CLI, different vendor family from author | Any operator-named reviewer CLI, different vendor family from author |
| Credentialed installed A/B | Optional diagnostic arm | Optional diagnostic arm |
| Exact-tag install smoke | Post-publish oracle | Post-publish oracle |

## Shared contract

Skills use `name`, `description`, `when_to_use`, and `metadata.short-description`
frontmatter; `memory-hygiene` sets `disable-model-invocation` for Claude Code.
Codex honors `name`, `description`, and the short description. Its explicit-only
policy lives in `memory-hygiene/agents/openai.yaml` as
`policy.allow_implicit_invocation: false`. Trigger evaluation reads the native
policy for each harness and tests explicit invocation separately. `SKILL.md` bodies
remain harness-neutral; `independent-review` runs whichever reviewer command
the operator names and forces no vendor. Channel-specific upgrade
commands live in a directly linked reference loaded only after channel
detection.

Install smoke verifies the cached invocation-policy file and native skill
discovery. Codex 0.153.3 does not expose invocation policy in `skills/list`,
so that response cannot prove effective implicit-selection behavior. Treat
model-based policy behavior as a separate evaluation.

Repository instructions, existing code, and configured project tools are
authoritative. megapowers supplies workflow defaults only where the repository
is silent.

Orchestration is a portable decision contract, not a scheduler. The lead
preflights tools, authentication, ownership, permissions, and write authority
before each dispatch. Native agent, team/task, and wait semantics remain
harness-owned. Verify current support against the exact CLI and plugin revision.

## Communication style

Both harness adapters use `output-styles/megapowers.md` as the source of truth.
Claude Code offers it through the `/config` output-style picker with
`force-for-plugin: false`. Selecting `Megapowers` preserves the built-in coding
instructions. Another selected style remains effective.

Codex has no output-style component. Its bundled `SessionStart` hook adds the
same body as developer context on startup, resume, clear, and compaction. Codex
requires the user to review and trust non-managed plugin hooks before it runs.
The adapter does not edit `~/.codex/config.toml` or `AGENTS.md`.
Set `MEGAPOWERS_OUTPUT_STYLE=off` before launching Codex to suppress style
injection without disabling the guard.

The same `SessionStart` command gives both harnesses a short reminder to load
applicable skills. This workflow guidance does not depend on the selected style
or `MEGAPOWERS_OUTPUT_STYLE`. Disabling the plugin removes its hooks and skills.

Both paths affect the main conversation only. They do not change ordinary
subagent prompts or hide tool results. `humanizing-prose` remains available for
tasks that need more specific editing judgment.

The plugin ships no model or role configuration and does not create model,
agent, context, sandbox, or permission availability. An optional user-owned
registry can describe relative capabilities and native bindings for the lead's
selection. It is advisory input: harness and repository policy still decide
what exists and what is authorized.

## Hook behavior

Both adapters forward high-confidence denials only. Reversible risk remains
with each harness's native permission system.

On either harness the hook is a narrow accident tripwire. It does not replace
the sandbox, OS permissions, review, or `safe-effects` approval for external
mutations.
Hook policy and JSON handling run in Go. The entrypoint only launches a cached
executable or builds it from the shipped source. Compilation uses the local Go
toolchain without fetching modules. The plugin contains no prebuilt binaries.

## Freshness policy

A support claim must be checked against a current CLI, the checked-in native
manifest, a fresh config home, and the exact plugin revision. Structural
validation runs without credentials. Behavioral claims require the
installed-plugin A/B protocol for both harnesses. See
[advanced/evals.md](./advanced/evals.md).

The Go freshness checker requires a dated source, exact tested CLI version,
and existing oracle paths. It checks evidence metadata and age, not whether
upstream documentation changed or a model follows the instructions.

On each scheduled review, retrieve the relevant official pages and inspect
changes against the supported CLI versions. Use the
[OpenAI Docs MCP](https://developers.openai.com/mcp) for official OpenAI lookup,
and the [Anthropic documentation index](https://code.claude.com/docs/llms.txt)
to locate current pages. Re-run the named oracle before updating its date or
version. A documentation change starts a compatibility review; it does not
authorize automatic policy adoption. Keep claims uncertain until tested.

<!-- freshness-sources -->
```json
[
  {
    "source": "https://code.claude.com/docs/en/skills",
    "reviewed": "2026-09-04",
    "cli": "claude",
    "version": "2.1.258",
    "oracle": ["scripts/validate.sh", "evals/studies/trigger-recall/policy_test.go"]
  },
  {
    "source": "https://code.claude.com/docs/en/output-styles",
    "reviewed": "2026-09-04",
    "cli": "claude",
    "version": "2.1.258",
    "oracle": ["scripts/validate.sh", "plugins/megapowers/output-styles/megapowers.md"]
  },
  {
    "source": "https://learn.chatgpt.com/docs/build-skills",
    "reviewed": "2026-09-04",
    "cli": "codex",
    "version": "0.153.3",
    "oracle": ["scripts/codex-install-smoke.sh", "evals/studies/trigger-recall/policy_test.go"]
  },
  {
    "source": "https://learn.chatgpt.com/docs/hooks",
    "reviewed": "2026-09-04",
    "cli": "codex",
    "version": "0.153.3",
    "oracle": ["plugins/megapowers/hooks/hook_runner_test.go", "plugins/megapowers/hooks/output_style_test.go"]
  }
]
```
