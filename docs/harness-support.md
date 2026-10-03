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
| Communication style | Selectable plugin output style (`megapowers:Megapowers`) | Trusted startup hook with environment opt-out |
| Skill reminder at session start | `SessionStart` hook | `SessionStart` hook |
| Subagent context | `SubagentStart` hook: skill reminder and compact report contract | `SubagentStart` hook: same text |
| Destructive-command guard | High-confidence denies only | High-confidence denies only |
| Completion and effect gate | `PreToolUse` denies the first commit, push, PR, publish, deploy, or outward `gh` write once per session with a reason naming the matching skill, unless it already loaded; the retry runs | Same, where Codex honors a `PreToolUse` deny |
| Git discard check | `PreToolUse` denies each distinct `git reset --hard`, forced `clean`, forced or whole-tree `checkout`, whole-tree `restore`, `branch -D`, or `stash drop` or `clear` once per session with a `git status` reminder; the retry runs | Same, where Codex honors a `PreToolUse` deny |
| Self-diagnosis | `megapowers-doctor` skill over the Go `doctor` command | Same |
| Native agents and parallel work | Direct agents; native team/task coordination when available | Direct agents; native team/task coordination when available |
| Personal capability registry | Advisory, read on demand | Advisory, read on demand |
| Durable goals | Prefer native goal support | Prefer native goal support |
| Independent review provider | Any operator-named reviewer CLI, different vendor family from author | Any operator-named reviewer CLI, different vendor family from author |
| Credentialed installed A/B | Optional diagnostic | Optional diagnostic |
| Exact-tag install smoke | Post-publish check | Post-publish check |

## Shared contract

Skills use `name`, `description`, `when_to_use`, and `metadata.short-description`
frontmatter; `memory-hygiene` sets `disable-model-invocation` for Claude Code.
Claude Code concatenates `description` and `when_to_use` in its skill listing.
Codex matches implicit invocation on `description`.

Codex's skill documentation, retrieved 2026-09-25, names `agents/openai.yaml`
`interface.short_description` and the plugin manifest
`interface.shortDescription` as short-description carriers. The skills that
ship with Codex also set `metadata.short-description`, so this plugin keeps
it. Claude Code documents that it does not act on `metadata` contents.

Codex's explicit-only policy lives in `memory-hygiene/agents/openai.yaml` as
`policy.allow_implicit_invocation: false`. The trigger evaluation reads each
harness's own policy and tests explicit invocation separately.

`SKILL.md` bodies do not depend on which harness is running.
`independent-review` runs whichever reviewer command the operator names and
forces no vendor. Channel-specific upgrade commands live in a linked reference
loaded only after the install channel is detected.

Install smoke verifies the cached invocation-policy file and native skill
discovery. Codex 0.153.3 does not expose invocation policy in `skills/list`,
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

Both harness adapters use `output-styles/megapowers.md` as the source of truth.
Claude Code offers it through the `/config` output-style picker and the
`/output-style` command with `force-for-plugin: false`, so it is off until
selected. The value that resolves is the plugin-qualified name
`megapowers:Megapowers`; the bare `Megapowers` does not resolve, and sessions
configured that way recorded no style. Selecting the style preserves the
built-in coding instructions. Another selected style stays in effect. The
style permits one short line of intent before long work and
brief progress lines during long tool chains; final answers keep the 100-word
default and 250-word ceiling and use no em dashes.

Codex has no output-style component. Its bundled `SessionStart` hook adds the
same body as developer context on startup, resume, clear, and compaction. The
Codex hooks reference retrieved 2026-09-25 lists `compact` as a `SessionStart`
`source` and states that matching hooks run before the next model request; the
plugin's hook declares no source matcher. Codex requires the user to review and
trust non-managed plugin hooks before it runs them, and asks again when a
release changes the hook definition. The adapter does not edit
`~/.codex/config.toml` or `AGENTS.md`. Set `MEGAPOWERS_OUTPUT_STYLE=off` before
launching Codex to suppress style injection without disabling the guard or the
reminders.

The same `SessionStart` command tells both harnesses to load a matching skill
before acting on a task and not to claim a skill without loading it. This is
instruction guidance, not an interpreter block, and it does not depend on the
selected style or `MEGAPOWERS_OUTPUT_STYLE`. The plugin ships no helper-code
language rule; that stays with each repository's own instructions. Disabling
the plugin removes its hooks and skills.

The style affects the main conversation only and cannot hide tool results.
Subagents receive a separate `SubagentStart` context on both harnesses: the
skill-loading reminder and a compact report contract (lead with the result,
cite `file:line` or command evidence, no padding, no em dashes).
`humanizing-prose` remains available for tasks that need more specific editing
judgment.

The plugin ships no model or role configuration and does not create model,
agent, context, sandbox, or permission availability. An optional user-owned
registry can describe relative capabilities and native bindings for the lead's
selection. It is advisory input: harness and repository policy still decide
what exists and what is authorized.

## Hook behavior

Both adapters forward high-confidence denials only. Reversible risk remains
with each harness's native permission system.

On either harness the hook is a narrow accident tripwire. It does not replace
the sandbox, OS permissions, review, or `safe-effects` approval for changes
outside the machine. The same `PreToolUse` hook also stops the first allowed
command in a session that looks like a completion step (`git commit`,
`git push`, `gh pr create` or `merge`, `gh release`) or a change outside the
machine (`npm`, `cargo`, or `pnpm publish`, `docker push`, `kubectl apply`,
`terraform apply`, `railway up`, `fly deploy`, `vercel --prod`). It denies that
command once with a reason naming `verify-and-finish` or `safe-effects`, and
the retry runs. If the skill already loaded in the transcript, there is no
stop. Without a session ID or a writable hook cache, the hook returns
non-blocking `additionalContext` instead.

Hook policy and JSON handling run in Go. The entrypoint only launches a cached
executable or builds it from the shipped source. Compilation uses the local Go
toolchain without fetching modules. The plugin contains no prebuilt binaries.
The `doctor` command in the same runner reports version, root, harness, Go
version against the cache, the Claude `outputStyle` value, and hook
registration; `megapowers-doctor` interprets it.

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
