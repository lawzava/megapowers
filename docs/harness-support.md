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
| Communication style | No plugin style; optional manual style in repository-only extras | No session-wide plugin style; `humanizing-prose` available on demand |
| Native agents and parallel work | Direct agents; native team/task coordination when available | Direct agents; native team/task coordination when available |
| Personal capability registry | Advisory, read on demand | Advisory, read on demand |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Another available model under user reviewer instructions | Another available model under user reviewer instructions |
| Credentialed installed A/B | Optional diagnostic | Optional diagnostic |
| Exact-tag install smoke | Post-publish check | Post-publish check |

## Shared contract

Skills use only `name` and a task-triggering `description` of at most 1024
characters, as plain text. Trigger phrases live in `description`, not
harness-only fields. `grill-me` and `close-off` require an explicit user request
in their descriptions. Claude Code and OpenCode rely on that model guidance,
not an invocation-policy frontmatter flag.
Codex and OpenCode use `description` for discovery.

OpenCode accepts HTTP skill catalogs, local source paths in its `skills` array,
or copied skills. This is not a plugin installation. See
[install.md](./install.md#opencode) and the
[OpenCode skills documentation](https://opencode.ai/docs/skills/).
The release catalog lists every shipped skill's files and uses the plugin
version for cache refresh. This documents the source format, not a behavioral
test result.

Codex's skill documentation, retrieved 2026-10-08, names `agents/openai.yaml`
`interface.short_description` and the plugin manifest
`interface.shortDescription` as short-description carriers. The directory
edition uses the existing plugin manifest field when adding agent interfaces;
`SKILL.md` frontmatter has no nested metadata.

Codex's explicit-only policy lives in each such skill's `agents/openai.yaml` as
`policy.allow_implicit_invocation: false`. The trigger evaluation distinguishes
native policy from description-only guidance and tests explicit invocation
and implicit non-selection separately.

`SKILL.md` bodies do not depend on which harness is running.
`independent-review` supplies guidance for another available model, using
native subagents or native review. Cross-vendor review
is preferred but optional. Global CLAUDE.md or AGENTS.md instructions define
independence and reviewer choice. Inspect secrets and disclosure scope before
sending off-machine. No review launcher ships with the skill.

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

The plugin ships no output style. The unchanged Claude Code style lives at
`extras/output-styles/megapowers.md` for manual installation into
`~/.claude/output-styles/`; see [extras](../extras/README.md#output-style).
Select the manually installed `Megapowers` in `/config` or `/output-style`.
The former plugin-qualified `megapowers:Megapowers` no longer resolves from
the plugin. Selecting the manual style preserves built-in coding instructions.
Another selected style stays in effect. The style permits one short line of intent before long work and
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

The plugin ships no executable code. Optional doctor and setup guidance live
in repository-only [extras](../extras/README.md), outside installed skills.

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
    "oracle": ["scripts/validate.sh", "extras/output-styles/megapowers.md"]
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
