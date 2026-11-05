# Harness support

Last reviewed: 2026-10-08 (UTC), against Claude Code 2.1.295 and Codex
0.162.0.

Current stable Claude Code and Codex remain the tested plugin install targets.
The harness-neutral skills and their supporting files may be copied into other
Agent Skills harnesses; that is not a tested plugin installation. Configure
native agent/reviewer bindings, isolation, permissions and any hooks locally.

| Capability | Claude Code | Codex |
|---|---|---|
| Marketplace metadata | `.claude-plugin/marketplace.json` | `.agents/plugins/marketplace.json` |
| Plugin manifest | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` |
| Task skills | Supported | Supported |
| Communication style | Selectable plugin output style (`megapowers:Megapowers`) | No session-wide plugin style; `humanizing-prose` available on demand |
| Self-diagnosis | `megapowers-doctor` skill over the standalone Go script | Same |
| Native agents and parallel work | Direct agents; native team/task coordination when available | Direct agents; native team/task coordination when available |
| Personal capability registry | Advisory, read on demand | Advisory, read on demand |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Native different-vendor subagent or operator-named reviewer CLI | Native different-vendor subagent or operator-named reviewer CLI |
| Credentialed installed A/B | Optional diagnostic | Optional diagnostic |
| Exact-tag install smoke | Post-publish check | Post-publish check |

## Shared contract

Skills use `name`, `description`, `when_to_use`, and `metadata.short-description`
frontmatter; `grill-me` and `close-off` set `disable-model-invocation` for Claude Code.
Claude Code concatenates `description` and `when_to_use` in its skill listing.
Codex matches implicit invocation on `description`.

Codex's skill documentation, retrieved 2026-10-08, names `agents/openai.yaml`
`interface.short_description` and the plugin manifest
`interface.shortDescription` as short-description carriers. The skills that
ship with Codex also set `metadata.short-description`, so this plugin keeps
it. Claude Code documents that it does not act on `metadata` contents.

Codex's explicit-only policy lives in each such skill's `agents/openai.yaml` as
`policy.allow_implicit_invocation: false`. The trigger evaluation reads each
harness's own policy and tests explicit invocation separately.

`SKILL.md` bodies do not depend on which harness is running.
`independent-review` uses a configured native reviewer from another vendor or an
operator-named external command with secret-scan and disclosure checks. It ships
no model bindings. Channel-specific upgrade commands live in a linked reference
loaded only after the install channel is detected.

Install smoke verifies the cached invocation-policy file and native skill
discovery. Codex 0.162.0 does not expose invocation policy in `skills/list`,
so that response cannot prove effective implicit-selection behavior. Treat
model-based policy behavior as a separate evaluation.

Repository instructions, existing code, and configured project tools are
authoritative. megapowers supplies workflow defaults only where the repository
is silent.

The orchestration guidance is a set of decision rules, not a scheduler. The
lead checks tools, authentication, ownership, permissions, and write access
before each dispatch. Agent, team/task, and wait behavior belong to the
harness. Verify current support against the exact CLI and plugin revision.

## Communication style

Claude Code uses `output-styles/megapowers.md` as its output-style source.
It offers it through the `/config` output-style picker and the
`/output-style` command with `force-for-plugin: false`, so it is off until
selected. The value that resolves is the plugin-qualified name
`megapowers:Megapowers`; the bare `Megapowers` does not resolve, and sessions
configured that way recorded no style. Selecting the style preserves the
built-in coding instructions. Another selected style stays in effect. The
style permits one short line of intent before long work and
brief progress lines during long tool chains; final answers keep the 100-word
default and 250-word ceiling and use no em dashes.

Codex has no output-style component in this plugin. Automatic style injection
is removed; the plugin does not edit `~/.codex/config.toml` or `AGENTS.md`.
`humanizing-prose` remains available for tasks that need editing judgment.

The plugin supplies no startup skill reminder or subagent report injection.
User and repository instructions still take precedence over skill guidance.
The Claude Code style affects the main conversation and cannot hide tool results.

The plugin ships no model or role configuration and does not create model,
agent, context, sandbox, or permission availability. An optional user-owned
registry can describe relative capabilities and native bindings for the lead's
selection. It is advisory input: harness and repository policy still decide
what exists and what is authorized.

## Installation diagnostics

The standalone Go doctor reports plugin version, root, harness, Claude Code
output-style settings, local marketplace versions, recent skill loads, and the optional agent
registry. `megapowers-doctor` interprets its report. It runs only when invoked
and needs Go 1.25 or newer.

The plugin supplies no destructive-command guard, completion gate, effect
gate, or git discard gate. Harness permissions and OS controls remain the
enforcement boundary; `verify-and-finish` and `safe-effects` supply guidance.

## Freshness policy

A support claim must be checked against a current CLI, the checked-in native
manifest, a fresh config home, and the exact plugin revision. Structural
validation runs without credentials. Behavioral claims require the
installed-plugin A/B protocol for both harnesses. See
[advanced/evals.md](./advanced/evals.md).

The Go freshness checker requires a dated source, an exact tested CLI version,
and existing paths in the `oracle` field (the checks that back the claim). It
checks evidence metadata and age, not whether upstream documentation changed
or a model follows the instructions. It runs as a maintainer check, not inside
the pull-request gate, so a stale review date does not block unrelated changes.

On each scheduled review, retrieve the relevant official pages and inspect
changes against the supported CLI versions. Use the
[OpenAI Docs MCP](https://developers.openai.com/mcp) for official OpenAI lookup,
and the [Anthropic documentation index](https://code.claude.com/docs/llms.txt)
to locate current pages. Re-run the named checks before updating a date or
version. A documentation change starts a compatibility review; it does not
authorize automatic policy adoption. Keep claims uncertain until tested.

<!-- freshness-sources -->
```json
[
  {
    "source": "https://code.claude.com/docs/en/skills",
    "reviewed": "2026-10-08",
    "cli": "claude",
    "version": "2.1.295",
    "oracle": ["scripts/validate.sh", "evals/studies/trigger-recall/policy_test.go"]
  },
  {
    "source": "https://code.claude.com/docs/en/output-styles",
    "reviewed": "2026-10-08",
    "cli": "claude",
    "version": "2.1.295",
    "oracle": ["scripts/validate.sh", "plugins/megapowers/output-styles/megapowers.md"]
  },
  {
    "source": "https://learn.chatgpt.com/docs/build-skills",
    "reviewed": "2026-10-08",
    "cli": "codex",
    "version": "0.162.0",
    "oracle": ["scripts/codex-install-smoke.sh", "evals/studies/trigger-recall/policy_test.go"]
  }
]
```
