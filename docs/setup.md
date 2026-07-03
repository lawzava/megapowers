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
```

Or run `/plugin` and browse Discover.

Verify the install: run `/plugin` and confirm the plugin is listed as
installed. Then, from a fresh session (the session-start rule and hooks only
appear in sessions started after the install), ask the agent to "load the
test-driven-development skill and quote its core principle". The expected
sentence, which exists nowhere outside the skill body:

> if you didn't watch the test fail, you don't know whether it tests the
> right thing

A correct quote proves skills are discoverable and loadable. The probe needs
the `megapowers` bundle (that skill ships in it); for other plugins, confirm
the `/plugin` listing or ask for one of their skills instead. This is exactly
what the install-smoke study asserts; see `evals/studies/install-smoke/`.
What visibly changes in day-to-day sessions is listed in the
[README quickstart](../README.md#quickstart-claude-code).

Nine skills are also published as standalone marketplace entries for
cherry-pickers: `brainstorming`, `systematic-debugging`,
`test-driven-development`, `writing-plans`, `writing-skills`,
`multi-agent-delegation`, `golang-patterns`, `python-patterns`, and
`typescript-patterns`. Default to the bundles. Install a bundle or its
standalone skill, not both: a skill installed twice registers twice.

## Per-plugin prerequisites

Each plugin installs and runs on its own; install only the tools for the parts
you use. Cross-plugin references are soft: where a skill names a skill from
another plugin it says "if installed" and works without it. The pairing that
adds the most is `megapowers` plus `mega-orchestration`: the process pipeline
escalates into delegation, verification, and autonomous runs when both are
present.

- mega-orchestration: the Codex roles (plan/code review, small impl) need
  Codex native subagents when running in Codex, or the Codex CLI/SDK from
  other harnesses. The visual/browser role needs `playwright-cli` plus a
  vision-capable model to read the screenshots: `npm i -g @playwright/cli`,
  then `playwright-cli install --skills` installs Microsoft's own
  playwright-cli skill into `.claude/skills/`. megapowers does not vendor
  that skill: Playwright distributes and updates it, and a shipped copy would
  register twice. Antigravity is documented but disabled; see the
  [mega-orchestration README](../plugins/mega-orchestration/README.md). Roles
  you don't use don't need their tools installed.
- mega-go: `greenfield-go-stack` optionally uses the context7 MCP server to
  fetch current library docs while scaffolding; it degrades gracefully without it.
- mega-guardrails: the hooks require `jq`. The auto-format hook additionally
  uses gofmt/goimports (Go) and a project-local prettier (JS/TS/etc.) when
  present, and skips them quietly otherwise.

## Codex marketplace

Codex installs from a local checkout (there is no remote marketplace add):

```
git clone https://github.com/lawzava/megapowers && cd megapowers
codex plugin marketplace add ./
codex
/plugins
```

Or install non-interactively (the verb is `add`, not `install`):

```
codex plugin add megapowers@megapowers
codex plugin add mega-orchestration@megapowers
```

Verify: `codex plugin marketplace list` shows `megapowers`. After installing a
plugin, the same first-task probe applies (ask the agent to quote the
test-driven-development core principle).

Install `megapowers`, `mega-go`, `mega-python`, `mega-ts`, or
`mega-orchestration` from the `megapowers` marketplace. `mega-guardrails` is
not listed for Codex because its hook wiring is Claude-specific.

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

The CLI reads this repo's `.claude-plugin/marketplace.json` and discovers
every plugin's skills, grouped by plugin. A skill's `scripts/` and
`references/` install with it: the whole skill directory is copied. Installs
are recorded in `skills-lock.json`, which makes the same skill set
reproducible on another machine (restore is `npx skills
experimental_install`, still marked experimental upstream).

Two rules:

- Skills only. Hooks and delegate agents do not travel this channel. On
  Claude Code and Codex, prefer the native marketplaces above, which ship the
  full bundle. On other harnesses the hooks would not run anyway (see
  [`docs/tool-support.md`](./tool-support.md)), so nothing real is lost.
- One channel per agent per machine. Never install the same skill via a
  native marketplace and the skills CLI: a skill registered twice fires
  twice, same as the bundle-vs-standalone rule above.

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

Antigravity root plugin manifests are present as `plugins/*/plugin.json`. Before
installing them with `agy plugin install`, confirm your local Antigravity CLI
accepts the nested `skills/<name>/SKILL.md` shape. See
[`docs/tool-support.md`](./tool-support.md) for the current support matrix.

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

- Codex: the marketplace is a local checkout, so put the clone in your
  dotfiles bootstrap (`git clone` + `codex plugin marketplace add ./` +
  `codex plugin add <plugin>@megapowers` per plugin) and update with
  `git pull`.
- Everything else: commit `skills-lock.json` where your dotfiles bootstrap
  runs, and install non-interactively:
  `npx skills add lawzava/megapowers -s '*' -y`.

Whatever the channel, follow [Updating](#updating) below before rolling a
fleet forward.

## Optional templates

`templates/` holds copyable examples, not files to install wholesale:

- `templates/CLAUDE.md` and `templates/CODEX.md` are starter instruction files
  for other projects.
- `templates/codex-config.toml` is a minimal Codex baseline with no private MCP
  bridge requirement.
- `templates/playwright-mcp-settings.json` is a starter MCP registration for the
  Playwright browser server, for harnesses that drive the browser through an MCP
  rather than `playwright-cli` directly.
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

## Updating

Plugins are versioned (see each `.claude-plugin/plugin.json` /
`.codex-plugin/plugin.json` and the root `CHANGELOG.md`). Read the changelog
before updating; behavioral guidance can change between versions.

- Claude Code: `/plugin marketplace update megapowers` refreshes the
  marketplace, then update the plugin from the `/plugin` installed list.
  Installed plugins are copies: local edits you made inside an installed
  plugin are overwritten on update. Keep customizations in a fork instead.
- Codex: update the local checkout (`git pull`), then refresh each installed
  plugin: re-run `codex plugin add <plugin>@megapowers`, or use `/plugins`
  inside Codex.
- skills CLI installs: `npx skills update` (all skills, interactive) or
  `npx skills update <name> -y`.
- OpenCode / Antigravity (symlinked skills): `git pull` the checkout;
  symlinks pick the change up immediately. If you copied instead of
  symlinking, re-copy the skill directories you use.
- Forks: merge upstream when you want the changes; `scripts/validate.sh`
  and `bash evals/run-all.sh` tell you whether your local edits still hold.

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
lists every entry in `.claude-plugin/marketplace.json` (currently 15: 6 plugin
bundles plus 9 standalone skills):

```
/plugin marketplace add ./
/plugin
```

For Codex, confirm the repo marketplace lists every entry in
`.agents/plugins/marketplace.json` (currently 5):

```
codex plugin marketplace add ./
codex plugin marketplace list
```
