# Setup

Prefer to delegate this? [`agent-install.md`](./agent-install.md) is this
document rewritten as instructions for a coding agent: paste its URL into any
agent and it installs, verifies, and reports.

## Claude Code marketplace

```
/plugin marketplace add lawzava/megapowers
```

Then install what you want:

```
/plugin install megapowers@megapowers        # the workflow core
/plugin install mega-orchestration@megapowers # multi-model orchestration
/plugin install mega-guardrails@megapowers    # safety hooks + statusline
/plugin install mega-go@megapowers            # greenfield Go
/plugin install mega-python@megapowers        # greenfield Python
/plugin install mega-ts@megapowers            # greenfield TypeScript
/plugin install mega-frontend@megapowers      # frontend design guidance
```

Or run `/plugin` and browse Discover.

Verify the install: run `/plugin` and confirm the plugin is listed as
installed. Then, from a fresh session (the session-start rule and hooks only
appear in sessions started after the install, and this setup doc is not in that
session's context), ask the agent to "load the test-driven-development skill
and quote its core principle". This doc and `agent-install.md` quote the
sentence too, so probe from a session that has neither in context; there the
expected sentence lives only in the skill body:

> if you didn't watch the test fail, you don't know whether it tests the
> right thing

A correct quote proves skills are discoverable and loadable. The probe needs
the `megapowers` bundle (that skill ships in it); for other plugins, confirm
the `/plugin` listing or ask for one of their skills instead. This is exactly
what the install-smoke study asserts; see `evals/studies/install-smoke/`. What
visibly changes in day-to-day sessions is listed in the [README
quickstart](../README.md#quickstart-claude-code).

## Per-plugin prerequisites

Each plugin installs and runs on its own; install only the tools for the parts
you use. Cross-plugin references are soft: where a skill names a skill from
another plugin it says "if installed" and works without it. The pairing that
adds the most is `megapowers` plus `mega-orchestration`: the process pipeline
escalates into delegation, verification, and autonomous runs when both are
present.

- mega-orchestration: each role needs the CLI of the provider it resolves to
  (`delegate-resolve <role>` prints BINARY). The Codex routes need Codex native
  subagents when running in Codex. From Claude Code, prefer OpenAI's
  first-party `codex-plugin-cc`; other harnesses use the Codex CLI, SDK, or MCP
  server as documented in the Codex provider reference. The Claude routes (plan
  review, and the cross-vendor review/verify chains under a non-Anthropic lead)
  need the Claude CLI. The visual/browser role needs `playwright-cli` plus a
  vision-capable model to read the screenshots: `npm i -g @playwright/cli`,
  then `playwright-cli install --skills` installs Microsoft's own
  playwright-cli skill into `.claude/skills/`. megapowers does not vendor that
  skill: Playwright distributes and updates it, and a shipped copy would
  register twice. Roles you don't use don't need their tools installed.
- mega-go: `greenfield-go-stack` optionally uses the context7 MCP server to
  fetch current library docs while scaffolding; it degrades gracefully without
  it.
- mega-guardrails: the hooks require `jq`. The auto-format hook additionally
  uses gofmt/goimports (Go) and a project-local prettier (JS/TS/etc.) when
  present, and skips them quietly otherwise.

### Optional: skipping the prompt on inspection commands

`mega-guardrails` ships `hooks/allow-read-only.sh`, which is **not
registered**. It is a PreToolUse(Bash) hook that auto-approves a command when
the command string carries no write construct and names an inspection command
that does not write, optionally behind one `cd <path> &&` prefix. It never
denies and never asks; anything else is passed through untouched to the normal
permission flow.

It exists because a `permissions.allow` rule cannot do this job. `Bash(ls:*)`
matches on the command prefix, so it also approves `ls -la > ~/.bashrc` and `ls
"$(sh -c 'touch owned')"`. There is no read-only prefix, which is why the
settings template ships no allow rules at all. A hook is the only mechanism
that sees the whole string before deciding.

What it removes: across 22,201 observed Bash calls the most common shape was
`cd X && ...` at 4,348 occurrences, and the interruptions read "This Bash
command contains multiple operations. The following part requires approval:"
over parts like `ls -la` and `wc -l`. It approves `ls`, `wc`, `stat`, `file`,
`head`, and `tail`, plus a single `cd <path> && <command>` prefix.

Not every flag of those six, though, and the difference will look arbitrary the
first time you hit it. A flag is on the list only when its documented behavior
can neither write nor run a program, checked one flag at a time against the
manual and then under `strace`. So a few ordinary-looking ones still prompt:
`file -p` writes, because restoring the access time is a `utimensat` on the
file you just inspected; `file -z` runs a decompressor that the operand's own
first bytes name; `file -C` compiles a magic cache to disk; `stat
--cached=never` can make a network filesystem write data back; and `tail -f`
never returns. A flag the hook does not recognize prompts as well, rather than
being guessed at, so a flag from a future coreutils release fails closed.

To enable it, add a PreToolUse(Bash) entry pointing at
`${CLAUDE_PLUGIN_ROOT}/hooks/run-hook.cmd dispatch.sh allow-read-only.sh` in
your own settings, alongside the destructive-command hook rather than replacing
it.

Read this before enabling it. Two things it does not do.

**It does not approve git, including `git status`.** That looks like an
omission and it is not. `git status` and `git diff` rewrite `.git/index`
whenever the cached stat data is stale, which is any worktree you just edited,
and `git status`, `git diff`, and `git ls-files` run whatever program the
repository's `core.fsmonitor` names, while `git log -p`, `git show`, and `git
diff` run `diff.external`, a textconv filter, or `core.pager`. All of that
lives in `.git/config` and `.gitattributes`, which the hook never reads.
Whether a git command writes is a property of the repository, not of the
command you typed, so it is not a question this hook can answer. `git -C <path>
status` will keep prompting.

**It does not prove the command is harmless, and it does not prove which
program runs.** It approves `head -n 5 .env` and `wc -l /home/you/.ssh/id_rsa`
without a prompt, because reading a secret is not a write. Note the second one
is spelled out: a `~` makes the hook pass the command through to the normal
prompt, since tilde expansion is not in its inert byte set, but the expanded
path an agent usually writes is approved. On a machine whose sandbox denies
those paths the read still fails; what you give up is the prompt. And approval
is a statement about the command string, not about the executable: an `ls`
earlier on your `PATH`, or an exported shell function named `ls`, passes every
check. That is the same hole the prompt has, since approving `ls -la` by hand
never told you which binary answered either, but it is the reason the hook
claims a property of the string and stops there. If either trade is wrong for
your setup, leave the hook unregistered. It ships off for exactly this reason.

## Codex marketplace

Codex adds this repo as a remote Git marketplace (codex-cli 0.142.5+ accepts
`owner/repo[@ref]`; unpinned tracks the default branch so upgrades work, see
[Pinning](#pinning-to-a-release) for the tag-pinned variant):

```
codex plugin marketplace add lawzava/megapowers
codex
/plugins
```

Or install non-interactively (the verb is `add`, not `install`):

```
codex plugin add megapowers@megapowers
codex plugin add mega-orchestration@megapowers
codex plugin add mega-guardrails@megapowers
```

Update later with `codex plugin marketplace upgrade megapowers` (refreshes the
Git snapshot), then re-run `codex plugin add` for each plugin.

Verify: `codex plugin marketplace list` shows `megapowers`. After installing a
plugin, the same first-task probe applies (ask the agent to quote the
test-driven-development core principle in a fresh session).

Install `megapowers`, `mega-go`, `mega-python`, `mega-ts`, `mega-frontend`,
`mega-orchestration`, or `mega-guardrails` from the `megapowers` marketplace.
Under Codex, mega-guardrails supplies the destructive-command adapter only; its
formatter and statusline remain Claude Code features.

### Contributor or fork variant

To track a local checkout or a fork instead of the published tag, add the
working directory as a local marketplace:

```
git clone https://github.com/lawzava/megapowers && cd megapowers
codex plugin marketplace add ./
codex plugin add megapowers@megapowers
```

Update it with `git pull` in the checkout.

### Codex native agents and v2

Codex native multi-agent support is stable and enabled by default. This repo's
baseline deliberately opts into the under-development `multi_agent_v2` surface.
V2 is a same-model context-sharding surface: its native spawn call does not
expose a per-spawn role, model, or effort selector, so workers inherit the
active session model. It does not automatically select this repo's `builder`
(Terra) or `reviewer` (Sol) profiles.

`mega-orchestration` still packages those optional profiles under
`assets/codex-agents/` for Codex surfaces that support named role selection.
Find the installed plugin directory with `codex plugin list`, review the files,
then copy the profiles you want into `~/.codex/agents/` (global) or
`<repo>/.codex/agents/` (project). For a cheaper or differently configured
Codex worker outside native v2, use a bounded `codex exec` run; for another
provider, use `delegate-resolve`. For ordinary delegation, create a dedicated
linked worktree before dispatching a writer and include its path in the brief.
Recursive coordinator mode is the explicit shared-checkout exception; do not
create worktrees for it.

A v2 global baseline with up to ten concurrent subagents is:

```toml
[features.multi_agent_v2]
enabled = true
max_concurrent_threads_per_session = 11
multi_agent_mode_hint_text = """
Use subagents only when the user or applicable AGENTS.md or skill instructions explicitly authorize delegation. The root agent owns spawning by default; workers spawn only in a deliberately designed coordinator workflow with isolated artifact ownership. Keep each ordinary batch to at most six workers so the root retains capacity for integration and recovery, even though the session cap is higher.

For independent workers, pass fork_turns = "none" and make the brief self-contained. Use a small positive fork_turns count only when specific recent turns are essential. Use fork_turns = "all" only for a true same-context continuation, never as a convenience default.

The root owns integration. Do not finish gating work until all required workers have returned and their outputs have been reviewed and independently validated. Completed workers remain idle; send a follow-up only for the same assignment, start a fresh worker for a new problem, and interrupt only running workers.

Treat the canonical task path as the nesting counter. If it already has five task-name components beneath /root, do not spawn another subagent; continue locally or report the limit.
"""
```

The v2 cap includes the root thread, so 11 permits ten subagents. Remove the v1
`agents.max_threads` key when enabling v2; Codex rejects that combination. The
six-worker ordinary-batch limit preserves root capacity for integration and
recovery; ten is a ceiling, not a target. For independent work, fresh context
avoids copying the root's transcript and compactions into every worker. Use a
small positive `fork_turns` count only for essential recent turns, and reserve
`all` for a genuine continuation of the same context. Reuse an idle worker only
for the same assignment; start a fresh worker for a new problem. As observed in
Codex 0.144.4, v2 does not enforce `agents.max_depth`, so the depth-five limit
is a model-visible system policy, not a hard runtime cap.

Pin the Codex session model to `gpt-5.6-sol`. Under the shipped catalog Codex
is the critic rather than the lead. Roles that adjudicate or refute run at
`high`; roles bounded by something other than reasoning depth (scoped
implementation, visual, browser) run at `medium`, following OpenAI's GPT-5.6
guidance that medium is the balanced starting point and that `high` or `xhigh`
want eval evidence of a meaningful gain. `templates/codex-config.toml` ships
`high` as the Codex session default for the same reason; raise it per task,
with a reason, rather than standing at `xhigh`. A 2026-08-05 audit of 1,623
rollouts found `xhigh` running roughly three times as often as `high`, which is
the drift that default exists to stop. The current bundled Sol model also
supports `ultra`, which adds automatic task delegation. Named profiles live in
separate `$CODEX_HOME/<name>.config.toml` files and are selected with
`--profile`; do not put `[profiles.*]` tables in the main config. Copy
`templates/codex-complex.config.toml` to `$CODEX_HOME/complex.config.toml` for
deliberate complex work, then start it with `codex --profile complex`. Complex
plan/spec review can still route independently to Claude (Opus 5). A Codex lead
should not register `codex mcp-server` under `[mcp_servers.codex]`: that
channel is for another harness delegating into Codex, while native subagents
are the direct path inside Codex.

### Codex hooks

Installed Codex plugins now expose their hooks directly. Cross-harness
dispatchers use Codex's `PLUGIN_ROOT` environment variable to select the Codex
payload while retaining the Claude Code behavior from the same manifest:

- `megapowers` SessionStart injects the rendered model catalog.
- `mega-orchestration` Stop runs the independent-review nudge; the autonomous
  Claude run-loop becomes a no-op under Codex.
- `mega-guardrails` PreToolUse runs the destructive-command adapter; its
  PostToolUse formatter becomes a no-op under Codex. Codex does not support the
  guard's `ask` decision, so the adapter passes catastrophic `deny` decisions
  through and leaves reversible-risk approval to Codex.

No manual `~/.codex/hooks.json` wiring is needed. Before a non-managed command
hook runs, Codex asks you to trust its exact definition; trust is hash-bound,
so an upgraded hook is skipped until reviewed again. Use `/hooks` in Codex to
review and trust the installed definitions. Do not use
`--dangerously-bypass-hook-trust` for an interactive installation.

If a megapowers hook fires twice, a hand-wired entry for it is still in
`~/.codex/hooks.json` or a project's `.codex/hooks.json`. Delete that entry,
keeping unrelated hooks; the installed plugin provides it now.

After an upgrade, restart the app server so the live process and CLI load the
same plugin snapshot:

```bash
codex app-server daemon restart
codex app-server daemon version
codex --version
```

The app-server and CLI versions should match. In a fresh session, confirm the
rendered model-catalog block appears and `/hooks` lists five hook handlers
across three plugins: one SessionStart, two Stop, one PreToolUse, and one
PostToolUse. The run-loop Stop handler and formatter PostToolUse handler
intentionally no-op under Codex; the other three are active. Confirm `codex
plugin list` reports one source for each megapowers plugin. If a skill appears
twice, remove the older shared-directory or legacy standalone install. Install
language plugins only where needed; loading every language bundle globally can
exceed the initial skill-description budget even though each plugin is valid
alone.

## Pinning to a release

By default a marketplace `add` tracks the default branch, so you receive new
plugin versions automatically as the maintainer publishes them. Adopters who
want change-controlled updates can pin instead. Two facts govern what a pin
does:

- Marketplace source: `add` supports a ref (branch or tag), not a commit sha.
  Pin to a published tag with `codex plugin marketplace add
  lawzava/megapowers@v0.9.1`, or, for Claude Code, add `"ref": "v0.9.1"` to the
  `extraKnownMarketplaces` source (see
  [Fleet](#fleet-keeping-many-devices-in-sync)). A tag is immutable, so
  `marketplace upgrade` cannot move a tag-pinned source; to update under a
pin, remove the marketplace and re-add it at the new tag.
- Plugin version field: each `plugin.json` declares a `version`. That version
  pins the installed plugin until the maintainer bumps the string; new commits
  that leave it unchanged do not reach existing installs. When the maintainer
  bumps it, background auto-update applies the new version.

Neither is an integrity pin (no sha in the ref), so a pin controls when you
move, not cryptographic provenance. Release tags from `v0.1.3` on are
GPG-signed and can be verified out of band (see SECURITY.md, Release
integrity). Every tag from `v0.1.1` on is a valid pin; `git ls-remote --tags
https://github.com/lawzava/megapowers` lists the ones currently published, so
this page never has to name the newest.

## Every other harness: the skills CLI

For harnesses without a native plugin marketplace (OpenCode, plus the rest of
the Agent Skills ecosystem, which is not supported here but can still read
these skills), use the open [skills CLI](https://github.com/vercel-labs/skills)
(published at skills.sh):

```bash
npx skills add lawzava/megapowers                # pick skills interactively
npx skills add lawzava/megapowers -s '*' -y      # everything, non-interactive
npx skills update                                # update installed skills
npx skills list                                  # what's installed where
```

Without `-g` these install into the current project; `-g` installs globally,
for every project. The trap below is about global installs.

Verify: `npx skills list` shows the skills, but that only confirms files were
copied. For an end-to-end check that the harness discovers and loads them, run
the first-task probe in a fresh session: ask the agent to load
`test-driven-development` and quote its core-principle sentence (the one quoted
under [Claude Code marketplace](#claude-code-marketplace) above).

The CLI reads this repo's `.claude-plugin/marketplace.json` and discovers every
plugin's skills, grouped by plugin. A skill's `scripts/` and `references/`
install with it: the whole skill directory is copied. Installs are recorded in
`skills-lock.json`, which makes the same skill set reproducible on another
machine (restore is `npx skills experimental_install`, still marked
experimental upstream).

Two rules:

- Skills only. Hooks and delegate agents do not travel this channel. On Claude
  Code and Codex, prefer the native marketplaces above, which ship the full
  bundle. On other harnesses these hook scripts are not ported anyway (see
  [`docs/harness-support.md`](./harness-support.md)), so nothing real is lost.
- One channel per agent per machine. Never install the same skill via a native
  marketplace and the skills CLI: a skill registered twice fires twice.

The second rule has a trap on mixed machines: the skills CLI installs several
agents (OpenCode and Codex among them) into the SHARED `~/.agents/skills/`
directory, and Claude Code scans that directory too. If the Claude Code plugins
are installed on the same machine, a global skills-CLI install into
`~/.agents/skills/` double-registers every skill for Claude Code. Found the
hard way on this project's own machine. On a machine that runs Claude Code with
the plugins, give other harnesses a tool-specific path instead: the symlink
fallback below into e.g. `~/.config/opencode/skills/`, or project-level
installs (`npx skills add` without `-g`).

### Manual fallback: symlinks from a checkout

Where you'd rather track a checkout (or a fork) directly, symlink the canonical
skill directories you want from `plugins/*/skills/*`:

```bash
# from the checkout root; adjust the target to your runtime's skill path
ln -s "$(pwd)"/plugins/megapowers/skills/* ~/.config/opencode/skills/
```

Symlinks track the checkout: `git pull` updates them in place. Do not load
every skill body through `instructions`: bodies are meant to load only when a
skill is invoked, and inlining them keeps every word in context permanently.

## Fleet: keeping many devices in sync

Make the install declarative once, then every machine converges instead of
being hand-configured:

- Claude Code: declare the marketplace and plugins in a `settings.json` you
  already sync (dotfiles for your own machines, the repo's
  `.claude/settings.json` for a team). Claude Code prompts each user to trust
  and install on first run; after that, updates follow the marketplace:

  ```json
  {
    "extraKnownMarketplaces": {
      "megapowers": {
        "source": { "source": "github", "repo": "lawzava/megapowers" }
      }
    },
    "enabledPlugins": {
      "megapowers@megapowers": true,
      "mega-orchestration@megapowers": true
    }
  }
  ```

- Codex: add the remote marketplace in your dotfiles bootstrap (`codex plugin
  marketplace add lawzava/megapowers` + `codex plugin add <plugin>@megapowers`
  per plugin) and update with `codex plugin marketplace upgrade megapowers`.
- Everything else: commit `skills-lock.json` where your dotfiles bootstrap
  runs, and restore from it with `npx skills experimental_install` (the verb
  that consumes the lockfile, still marked experimental upstream). To bootstrap
  without a committed lockfile, install fresh instead: `npx skills add
  lawzava/megapowers -s '*' -y`.

Whatever the channel, follow [Updating](#updating) below before rolling a fleet
forward.

## Optional templates

`templates/` holds copyable examples, not files to install wholesale:

- `templates/CLAUDE.md` and `templates/CODEX.md` are starter instruction files,
  one per harness. Both are lead charters: each harness leads in its own
  runtime and they dispatch each other on demand, so which one is running is
  which one is in charge. Neither ships a delegate variant. A session
  dispatched with a task brief is that brief's delegate for its duration, and
  both templates carry the same paragraph saying so.
- `templates/codex-config.toml` is a minimal Codex baseline with no
  user-configured MCP bridge requirement. Its `[features]` table turns off
  `tool_suggest`, `apps`, and `image_generation`, which a lead does not use and
  which cost 1,782 tokens of context per turn on codex-cli 0.146.0. Keep that
  table above `[features.multi_agent_v2]`: a bare `[features]` header captures
  every top-level key written after it, which would silently move
  `sandbox_mode` and `approval_policy` inside it.
- `templates/codex-complex.config.toml` is the optional named Sol ultra layer;
  save it as `$CODEX_HOME/complex.config.toml` and select it with `codex
  --profile complex`.
- `templates/playwright-mcp-settings.json` is a starter MCP registration for
  the Playwright browser server, for harnesses that drive the browser through
  an MCP rather than `playwright-cli` directly.
- `templates/codex-mcp-settings.json` is a starter MCP registration for `codex
  mcp-server` for a non-Codex lead. Register the server as `codex` so its tools
  resolve as `mcp__codex__codex` / `mcp__codex__codex-reply`. Full auth,
  sandbox, and thread mechanics live in mega-orchestration's Codex provider
  reference.
- `templates/settings.example.json` holds conservative, generic Claude Code
  defaults (no attribution trailers, secret-path denies, sandbox credential
  blocks). It sets no `defaultMode` and carries no `permissions.allow` entries,
  so copying it never loosens your permission posture. A read-only inspection
  allowlist was drafted for it and then dropped: a `Bash(cmd:*)` rule matches
  on the command prefix, so `Bash(ls:*)` also matches `ls -la > ~/.bashrc`, `ls
  "$(curl -fsS URL)"`, and `ls "$(sh -c 'touch owned')"`. No prefix rule can
  express "this command, without redirection, substitution, or control
  operators", which means no shell command is read-only at the prefix level and
  the whole shape is unsafe. Copy the keys you want into your own
  `~/.claude/settings.json`; do not replace your file wholesale. For more
  autonomy, opt in explicitly by adding `"defaultMode": "acceptEdits"`
  (auto-approves file edits) under `permissions` yourself; understand that it
  removes the per-edit prompt before you do.
- The same file turns off harness surfaces the plugins already cover, which
  buys back system-prompt budget every turn. Measured on Claude Code 2.1.222
  against a 31,442-token headless baseline, one key at a time:

  | Key | Saved | What it costs you |
  | --- | --- | --- |
  | `disableWorkflows` | 6,014 | The `Workflow` tool and the `ultracode` keyword |
  | `includeGitInstructions: false` | 1,558 | Built-in commit and PR mechanics |
  | `CLAUDE_CODE_DISABLE_EXPLORE_PLAN_AGENTS` | 287 | The built-in `Explore` and `Plan` agent types |
  | `skillOverrides` | ~1,000 | Listing lines for bundled skills you do not use |
  | `disableArtifact` | ~1,500 | The `Artifact` tool for publishing web pages |

  The `skillOverrides` and `disableArtifact` figures are interactive-session
  estimates: a `claude -p` run never loads either surface, so a headless probe
  scores both at zero. The three headless rows sum to 7,859 but the whole file
  measures 7,818, because one-key probes are not additive; treat any single row
  as approximate.
- Five of those keys trade a feature away, so decide rather than copy:
  - `disableWorkflows` also disables `templates/workflows/`. Re-enable it where
    you actually fan out, with `"disableWorkflows": false` in that repo's
    `.claude/settings.json`; project scope beats user scope. Left off,
    mega-orchestration's `best-of-n` and `audit-fanout` skills still run the
    same patterns through ordinary subagents, just without the runner.
  - `includeGitInstructions: false` assumes you also took
    `templates/CLAUDE.md`, whose Git section and the
    finishing-a-development-branch skill carry the same rules. Flip it back
    first if commit or PR quality slips.
  - `skillOverrides` is per skill, not all-or-nothing: `"off"` hides a bundled
    skill from everyone, `"user-invocable-only"` keeps `/name` typable while
    hiding it from the model, `"name-only"` drops just the description. Prefer
    it over `disableBundledSkills`, which also removes `claude-api`, `loop`,
    and `schedule`. Your own and plugin skills are unaffected either way.
  - `disableClaudeAiConnectors` stops claude.ai cloud MCP connectors from being
    fetched and connected at startup. Servers you configure yourself, in
    `.mcp.json`, `~/.claude.json`, or `--mcp-config`, are untouched. Drop the
    key if you drive connectors from claude.ai rather than from local config.
  - `CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY` suppresses the session quality
    survey. Use `feedbackSurveyRate` (0 to 1) instead if you would rather see
    it occasionally than never.
- `sandbox.credentials` takes objects, not strings: `{"path": "~/.ssh", "mode":
  "deny"}` for files and `{"name": "GITHUB_TOKEN", "mode": "deny"}` for
  variables, with `"mask"` as the other mode. Bare strings do not parse, and on
  2.1.222 that takes the rest of the settings source down with them: a file
  holding `"disableWorkflows": true` alongside `"files": ["~/.ssh"]` measured
  31,446 tokens, the same file with `{"path": "~/.ssh", "mode": "deny"}`
  measured 25,432. The published reference says invalid entries are stripped
  individually, so this may be version specific; check yours against your own
  build if you copied this template before v0.9.0.
- The `autoMode` block in the same file teaches the permission classifier your
  environment instead of leaving it to guess: which hosts are production (write
  statements get a confirm), what is routine here (fewer prompts). Replace the
  three REPLACE lines with facts about your machine; the `$defaults` entries
  keep the built-in rules. Copied verbatim it is harmless, just useless.
- `templates/agent-notify/` pushes a notification (Telegram by default,
  transport swappable) when an agent needs input or finishes, with a
  noise-filtering wrapper for Claude Code hooks and a Codex notify program. See
  its [README](../templates/agent-notify/README.md).
- `templates/codex-agents/` holds the source copies of the Terra-pinned Codex
  native subagent roles. The same files ship inside mega-orchestration under
  `assets/codex-agents/`, so upstream plugin users can copy them into
  `~/.codex/agents/` or `<repo>/.codex/agents/` without cloning this repo.
- `templates/workflows/` holds Claude Code dynamic-workflow scripts
  (`best-of-n.js`, `audit-fanout.js`); copy into `.claude/workflows/` or
  `~/.claude/workflows/`. See its [README](../templates/workflows/README.md).

## Updating

Plugins are versioned (see each `.claude-plugin/plugin.json` /
`.codex-plugin/plugin.json` and the root `CHANGELOG.md`). Read the changelog
before updating; behavioral guidance can change between versions.

Plugins are only half of it. The instruction and settings baselines in
`templates/` change between releases too, and nothing on your machine records
which version of them you took, so drift there is invisible unless something
goes looking. `upgrading-megapowers` does: it fetches the baselines at your
installed version and at the target, reports what the baseline itself changed
in between, and leaves the merging to you. It infers your adoption point from
the installed plugin version, which is wrong if you adopted the templates at a
different time than you installed the plugins, so it states that assumption
rather than presenting the delta as fact. Offline, the check cannot run and the
skill says so instead of reporting nothing found.

If the core plugin is installed, ask the agent to use `upgrading-megapowers`.
It inspects the active channel, preserves pins and scopes, proposes installed
updates plus relevant optional additions, asks once before writes, and verifies
the result. The exact native commands live in the skill's
[`channels.md`](../plugins/megapowers/skills/upgrading-megapowers/references/channels.md)
reference.

Without the skill, refresh only the channel already in use, update the existing
installed set, and verify it before adding anything. Marketplace installs use
the harness plugin manager; skills CLI installs name the approved skills and
preserve detected scope with `npx skills update <names> -p -y` or `npx skills
update <names> -g -y`; symlinked checkouts use `git pull --ff-only` only on a
clean floating branch with an upstream. Explicit pins fetch and select only an
approved ref. Forks integrate upstream under their existing merge policy and
run `scripts/validate.sh` plus `bash evals/run-all.sh`. Managed plugin copies
can overwrite local edits, so preserve customizations in a fork.

## Uninstalling

- Claude Code: `/plugin` → installed → remove the plugin. Removal unregisters
  its skills, hooks, and agents; confirm no `megapowers`-named entries remain
  under `/plugin`. To drop the marketplace registration too: `claude plugin
  marketplace remove megapowers`. If you enabled the statusline or the Fleet
  settings block, also delete the `statusLine`, `extraKnownMarketplaces`, and
  `enabledPlugins` keys you added to `settings.json`.
- Codex: remove the plugin in `/plugins`; remove the marketplace with `codex
  plugin marketplace remove megapowers` if you no longer want the repo listed.
- skills CLI installs: `npx skills remove` (interactive) removes the skill from
  every agent directory it was installed to and updates `skills-lock.json`.
- OpenCode: delete the symlinks or copied skill directories you created.
- Runtime state the skills wrote lives under `.megapowers/` in each project
  (run journals, SDD ledgers, evidence). It is plain text and git-ignored;
  delete it when you no longer need the trail.

## Validate a local checkout

```
scripts/validate.sh
```

Requires `jq`; `shellcheck` is optional (hook checks are skipped without it).

## Manual marketplace smoke test

From a checkout, point Claude Code at the local dir and confirm the marketplace
lists every plugin bundle in `.claude-plugin/marketplace.json`:

```
/plugin marketplace add ./
/plugin
```

For Codex, confirm the repo marketplace lists every plugin bundle in
`.agents/plugins/marketplace.json`:

```
codex plugin marketplace add ./
codex plugin marketplace list
```

For release certification, a local marketplace is insufficient. After the
signed tag is public, run the strict exact-ref study:

```bash
tag=v0.9.1   # the tag you just signed and pushed
evals/studies/install-smoke/run-smoke.sh \
  --out "${TMPDIR:-/tmp}/megapowers-install-$tag" \
  --source lawzava/megapowers --ref "$tag" --version "${tag#v}" \
  --harnesses claude,codex
```

It fetches the public tag, records its commit, verifies every plugin manifest
version, installs all seven plugins into fresh homes, and fails on any skipped
harness or failed first task.
