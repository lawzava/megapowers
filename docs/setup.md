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
session's context), ask the agent to "load the test-driven-development skill and
quote its core principle". This doc and `agent-install.md` quote the sentence
too, so probe from a session that has neither in context; there the expected
sentence lives only in the skill body:

> if you didn't watch the test fail, you don't know whether it tests the
> right thing

A correct quote proves skills are discoverable and loadable. The probe needs
the `megapowers` bundle (that skill ships in it); for other plugins, confirm
the `/plugin` listing or ask for one of their skills instead. This is exactly
what the install-smoke study asserts; see `evals/studies/install-smoke/`.
What visibly changes in day-to-day sessions is listed in the
[README quickstart](../README.md#quickstart-claude-code).

## Per-plugin prerequisites

Each plugin installs and runs on its own; install only the tools for the parts
you use. Cross-plugin references are soft: where a skill names a skill from
another plugin it says "if installed" and works without it. The pairing that
adds the most is `megapowers` plus `mega-orchestration`: the process pipeline
escalates into delegation, verification, and autonomous runs when both are
present.

- mega-orchestration: each role needs the CLI of the provider it resolves to
  (`delegate-resolve <role>` prints BINARY). The Codex routes need Codex
  native subagents when running in Codex. From Claude Code, prefer OpenAI's
  first-party `codex-plugin-cc`; other harnesses use the Codex CLI, SDK, or MCP
  server as documented in the Codex provider reference. The Claude routes (plan review, and the cross-vendor
  review/verify chains under a non-Anthropic lead) need the Claude CLI. The
  visual/browser role needs `playwright-cli` plus a vision-capable model to
  read the screenshots: `npm i -g @playwright/cli`, then `playwright-cli
  install --skills` installs Microsoft's own playwright-cli skill into
  `.claude/skills/`. megapowers does not vendor that skill: Playwright
  distributes and updates it, and a shipped copy would register twice. Roles
  you don't use don't need their tools installed.
- mega-go: `greenfield-go-stack` optionally uses the context7 MCP server to
  fetch current library docs while scaffolding; it degrades gracefully without it.
- mega-guardrails: the hooks require `jq`. The auto-format hook additionally
  uses gofmt/goimports (Go) and a project-local prettier (JS/TS/etc.) when
  present, and skips them quietly otherwise.

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
Under Codex, mega-guardrails supplies the destructive-command adapter only;
its formatter and statusline remain Claude Code features.

### Contributor or fork variant

To track a local checkout or a fork instead of the published tag, add the
working directory as a local marketplace:

```
git clone https://github.com/lawzava/megapowers && cd megapowers
codex plugin marketplace add ./
codex plugin add megapowers@megapowers
```

Update it with `git pull` in the checkout.

### Codex native agent roles

Codex native multi-agent support is stable and enabled by default. This repo's
baseline deliberately opts into the under-development `multi_agent_v2`
surface. `mega-orchestration` packages two
profiles under `assets/codex-agents/`: `builder.toml` and `reviewer.toml` both
pin `gpt-5.6-terra`, so fan-out does not inherit the Sol lead model. Find the
installed plugin directory with `codex plugin list`, review the files, then
copy the profiles you want into `~/.codex/agents/` (global) or
`<repo>/.codex/agents/` (project). Codex loads them on the next session.
Before dispatching `builder`, the lead must create a dedicated linked worktree
and include its path in the brief. The profile checks `git rev-parse --git-dir`
against `--git-common-dir` and refuses to edit the primary checkout.

A v2 global baseline with up to ten concurrent subagents is:

```toml
[features.multi_agent_v2]
enabled = true
max_concurrent_threads_per_session = 11
multi_agent_mode_hint_text = """
Use subagents only when delegation is explicitly authorized. Treat the canonical
task path as the nesting counter. At five components beneath /root, do not spawn.
"""
```

The v2 cap includes the root thread, so 11 permits ten subagents. Remove the
v1 `agents.max_threads` key when enabling v2; Codex rejects that combination.
As of Codex 0.144.3, v2 does not enforce `agents.max_depth`, so the depth-five
limit is a model-visible system policy, not a hard runtime cap.

Keep the normal lead on `gpt-5.6-sol` at `xhigh`. The current bundled Sol model
also supports `ultra`, which adds automatic task delegation. Named profiles
live in separate `$CODEX_HOME/<name>.config.toml` files and are selected with
`--profile`; do not put `[profiles.*]` tables in the main config. Copy
`templates/codex-complex.config.toml` to `$CODEX_HOME/complex.config.toml` for
deliberate complex work, then start it with `codex --profile complex`. Complex
plan/spec review can still route independently to Fable. A Codex lead should
not register `codex mcp-server` under `[mcp_servers.codex]`: that channel is
for another harness delegating into Codex, while native subagents are the
direct path inside Codex.

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
hook runs, Codex asks you to trust its exact definition; trust is hash-bound, so
an upgraded hook is skipped until reviewed again. Use `/hooks` in Codex to
review and trust the installed definitions. Do not use
`--dangerously-bypass-hook-trust` for an interactive installation.

Migrating from the v0.3.1 manual pilot: remove only the three megapowers pilot
entries you previously added to `~/.codex/hooks.json` or a project's
`.codex/hooks.json`, preserving unrelated hooks. Install/upgrade the three
plugins above, start Codex, and trust their plugin-provided hooks in `/hooks`.
Leaving the manual entries in place runs SessionStart, Stop, or PreToolUse
twice.

After an upgrade, restart the app server so the live process and CLI load the
same plugin snapshot:

```bash
codex app-server daemon restart
codex app-server daemon version
codex --version
```

The app-server and CLI versions should match. In a fresh session, confirm the
rendered model-catalog block appears and `/hooks` lists five hook handlers across three plugins:
one SessionStart, two Stop, one PreToolUse, and one PostToolUse. The run-loop
Stop handler and formatter PostToolUse handler intentionally no-op under Codex;
the other three are active. Confirm `codex plugin list` reports one source for
each megapowers plugin. If a skill appears twice, remove the older
shared-directory or legacy standalone install. Install language
plugins only where needed; loading every language bundle globally can exceed
the initial skill-description budget even though each plugin is valid alone.

## Pinning to a release

By default a marketplace `add` tracks the default branch, so you receive new
plugin versions automatically as the maintainer publishes them. Adopters who
want change-controlled updates can pin instead. Two facts govern what a pin
does:

- Marketplace source: `add` supports a ref (branch or tag), not a commit sha.
  Pin to a published tag with
  `codex plugin marketplace add lawzava/megapowers@v0.3.6`, or, for Claude Code,
  add `"ref": "v0.3.6"` to the `extraKnownMarketplaces` source (see
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
integrity). Tags `v0.1.1` through `v0.3.6` are the release pin range once this
version is published.

## Every other harness: the skills CLI

For harnesses without a native plugin marketplace (OpenCode, Antigravity,
Cursor, Copilot, and the rest of the Agent Skills ecosystem), use the open
[skills CLI](https://github.com/vercel-labs/skills) (published at skills.sh):

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

The CLI reads this repo's `.claude-plugin/marketplace.json` and discovers
every plugin's skills, grouped by plugin. A skill's `scripts/` and
`references/` install with it: the whole skill directory is copied. Installs
are recorded in `skills-lock.json`, which makes the same skill set
reproducible on another machine (restore is `npx skills
experimental_install`, still marked experimental upstream).

Two rules:

- Skills only. Hooks and delegate agents do not travel this channel. On
  Claude Code and Codex, prefer the native marketplaces above, which ship the
  full bundle. On other harnesses these hook scripts are not ported anyway (see
  [`docs/harness-support.md`](./harness-support.md)), so nothing real is lost.
- One channel per agent per machine. Never install the same skill via a
  native marketplace and the skills CLI: a skill registered twice fires
  twice.

The second rule has a trap on mixed machines: the skills CLI installs several
agents (OpenCode, Antigravity, Codex among them) into the SHARED
`~/.agents/skills/` directory, and Claude Code scans that directory too. If
the Claude Code plugins are installed on the same machine, a global skills-CLI
install into `~/.agents/skills/` double-registers every skill for Claude Code.
Found the hard way on this project's own machine. On a machine that runs
Claude Code with the plugins, give other harnesses a tool-specific path
instead: the symlink fallback below into e.g. `~/.config/opencode/skills/`,
or project-level installs (`npx skills add` without `-g`).

### Manual fallback: symlinks from a checkout

Where you'd rather track a checkout (or a fork) directly, symlink the
canonical skill directories you want from `plugins/*/skills/*`:

```bash
# from the checkout root; adjust the target to your runtime's skill path
ln -s "$(pwd)"/plugins/megapowers/skills/* ~/.config/opencode/skills/
```

Symlinks track the checkout: `git pull` updates them in place. Do not load
every skill body through `instructions`: bodies are meant to load only when a
skill is invoked, and inlining them keeps every word in context permanently.

Antigravity root plugin manifests are present as `plugins/*/plugin.json`, and
the nested `skills/<name>/SKILL.md` shape is Antigravity's native skill layout,
so a plugin install imports these skills as-is with no conversion. The
maintainer has not tested the `agy plugin` install path directly; the supported
lane for Antigravity is the skills CLI above. Whichever you use, verify with the
first-task probe: in a fresh Antigravity session, ask the agent to load
`test-driven-development` and quote its core-principle sentence. See
[`docs/harness-support.md`](./harness-support.md) for the current support matrix.

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

- Codex: add the remote marketplace in your dotfiles bootstrap
  (`codex plugin marketplace add lawzava/megapowers` +
  `codex plugin add <plugin>@megapowers` per plugin) and update with
  `codex plugin marketplace upgrade megapowers`.
- Everything else: commit `skills-lock.json` where your dotfiles bootstrap
  runs, and restore from it with `npx skills experimental_install` (the verb
  that consumes the lockfile, still marked experimental upstream). To bootstrap
  without a committed lockfile, install fresh instead:
  `npx skills add lawzava/megapowers -s '*' -y`.

Whatever the channel, follow [Updating](#updating) below before rolling a
fleet forward.

## Optional templates

`templates/` holds copyable examples, not files to install wholesale:

- `templates/CLAUDE.md` and `templates/CODEX.md` are starter instruction files
  for other projects (Claude Code lead, Codex delegate); `templates/CODEX-LEAD.md`
  is the variant for running Codex as the lead.
- `templates/codex-config.toml` is a minimal Codex baseline with no
  user-configured MCP bridge requirement.
- `templates/codex-complex.config.toml` is the optional named Sol ultra layer;
  save it as `$CODEX_HOME/complex.config.toml` and select it with
  `codex --profile complex`.
- `templates/playwright-mcp-settings.json` is a starter MCP registration for the
  Playwright browser server, for harnesses that drive the browser through an MCP
  rather than `playwright-cli` directly.
- `templates/codex-mcp-settings.json` is a starter MCP registration for
  `codex mcp-server` for a non-Codex lead. Register the server as `codex` so its
  tools resolve as `mcp__codex__codex` / `mcp__codex__codex-reply`. Full auth,
  sandbox, and thread mechanics live in mega-orchestration's Codex provider
  reference.
- `templates/settings.example.json` holds conservative, generic Claude Code
  defaults (no attribution trailers, secret-path denies, sandbox credential
  blocks). It does not set a `defaultMode`, so it never loosens your permission
  posture just by being copied. Copy the keys you want into your own
  `~/.claude/settings.json`; do not replace your file wholesale. For more
  autonomy, opt in explicitly by adding `"defaultMode": "acceptEdits"`
  (auto-approves file edits) under `permissions` yourself; understand that it
  removes the per-edit prompt before you do.
- The `autoMode` block in the same file teaches the permission classifier
  your environment instead of leaving it to guess: which hosts are
  production (write statements get a confirm), what is routine here (fewer
  prompts). Replace the three REPLACE lines with facts about your machine;
  the `$defaults` entries keep the built-in rules. Copied verbatim it is
  harmless, just useless.
- `templates/agent-notify/` pushes a notification (Telegram by default,
  transport swappable) when an agent needs input or finishes, with a
  noise-filtering wrapper for Claude Code hooks and a Codex notify program.
  See its [README](../templates/agent-notify/README.md).
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

If the core plugin is installed, ask the agent to use `upgrading-megapowers`.
It inspects the active channel, preserves pins and scopes, proposes installed
updates plus relevant optional additions, asks once before writes, and verifies
the result. The exact native commands live in the skill's
[`channels.md`](../plugins/megapowers/skills/upgrading-megapowers/references/channels.md)
reference.

Without the skill, refresh only the channel already in use, update the existing
installed set, and verify it before adding anything. Marketplace installs use
the harness plugin manager; skills CLI installs name the approved skills and
preserve detected scope with `npx skills update <names> -p -y` or
`npx skills update <names> -g -y`; symlinked checkouts use `git pull --ff-only`
only on a clean floating branch with an upstream. Explicit pins fetch and
select only an approved ref. Forks integrate upstream under their existing
merge policy and run `scripts/validate.sh` plus `bash evals/run-all.sh`.
Managed plugin copies can overwrite local edits, so preserve customizations in
a fork.

## Uninstalling

- Claude Code: `/plugin` → installed → remove the plugin. Removal
  unregisters its skills, hooks, and agents; confirm no `megapowers`-named
  entries remain under `/plugin`. To drop the marketplace registration too:
  `claude plugin marketplace remove megapowers`. If you enabled the
  statusline or the Fleet settings block, also delete the `statusLine`,
  `extraKnownMarketplaces`, and `enabledPlugins` keys you added to
  `settings.json`.
- Codex: remove the plugin in `/plugins`; remove the marketplace with
  `codex plugin marketplace remove megapowers` if you no longer want the repo
  listed.
- skills CLI installs: `npx skills remove` (interactive) removes the
  skill from every agent directory it was installed to and updates
  `skills-lock.json`.
- OpenCode / Antigravity: delete the symlinks or copied skill directories
  you created.
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
