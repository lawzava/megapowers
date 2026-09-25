# Install megapowers

megapowers supports current Claude Code and Codex. Choose one native channel per
harness so each skill registers once. Register the marketplace at the `release`
branch: it only fast-forwards to signed release tags, so the automatic
marketplace refresh that both harnesses run never installs unreleased `main`
state.

## Requirements

- A current Claude Code or Codex CLI, already authenticated.
- Git, for marketplace snapshots.
- Go 1.25 or newer, for hooks and deterministic tools. Hooks compile once into
  a local cache, outside the installed plugin. Later calls reuse that executable.
  Missing Go or an unusable cache produces an explicit hook error.

## Claude Code

```bash
claude plugin marketplace add lawzava/megapowers@release
claude plugin install megapowers@megapowers
```

Verify registration:

```bash
claude plugin list --json
```

Start a fresh session. Ask Claude Code to load `humanizing-prose` and summarize
its preservation rules. This checks discovery and full skill loading without
claiming broader behavioral quality.

The output style is optional and off until selected. Select the
plugin-qualified value `megapowers:Megapowers` in `/config` under Output style,
or run `/output-style megapowers:Megapowers`. The resulting setting is
`"outputStyle": "megapowers:Megapowers"`. The bare value `Megapowers` does not
resolve to the plugin style. The style preserves built-in coding instructions.
Select another style to opt out while keeping the plugin enabled. Start a new
session after changing style, then run `/megapowers:megapowers-doctor` to
confirm the value in effect.

For a project-scoped install, add `--scope project` to both marketplace and
install commands. For a local user-only install, use `--scope local`.

## Codex

```bash
codex plugin marketplace add lawzava/megapowers --ref release
codex plugin add megapowers@megapowers
```

Verify registration:

```bash
codex plugin marketplace list --json
codex plugin list --json
```

Start a fresh session, then review and trust the plugin hooks when Codex asks.
Codex skips every plugin hook until it is trusted, and records trust against
the hook's current hash, so a release that changes the hooks asks again. Once
trusted, the startup hook applies the shared direct, concise style as developer
context without editing user config. Set `MEGAPOWERS_OUTPUT_STYLE=off` in the
environment before launching Codex to omit the startup style while keeping the
destructive-command guard and the skill reminders enabled.

Ask Codex to load `humanizing-prose` and summarize its preservation rules. This
checks skill discovery separately from the startup style.

## Developing this repository

The repository's `.agents/skills` links and an installed plugin can expose the
same skills twice. Use the repository links for source development, or disable
that discovery channel when testing the installed plugin in an isolated home.
Keep the links as the canonical development entrypoints. Do not edit installed
caches or global configuration to hide a duplicate during an unrelated task.

For a deliberate Codex setup using the installed release in this checkout,
disable each repository skill path through native configuration:

```toml
[[skills.config]]
path = "/absolute/path/to/megapowers/.agents/skills/orchestrating/SKILL.md"
enabled = false
```

Use one entry per repository skill. Keep the installed plugin enabled so its
hooks remain available. Verify the exact path with native `skills/list` before
expanding the exclusions: the repository entry must be disabled and the installed
entry enabled. Verify hook trust with `hooks/list`. Exclude only the intended
checkout; a candidate run must load source through its own isolated plugin
installation. Removing these entries restores repository discovery.

## Pin a release

A local immutable checkout makes the selected source explicit for either
harness:

Replace `vX.Y.Z` with a reviewed published tag. Do not run the placeholder
literally.

```bash
release_tag=vX.Y.Z
git clone --branch "$release_tag" --depth 1 \
  https://github.com/lawzava/megapowers.git "megapowers-$release_tag"
```

Pass that checkout path instead of `lawzava/megapowers@release` (Claude) or
`lawzava/megapowers --ref release` (Codex) to the marketplace add command. Verify the selected commit before trusting it:

```bash
git -C "megapowers-$release_tag" rev-parse HEAD
git -C "megapowers-$release_tag" status --short
```

## Update

Use `upgrading-megapowers` for an agent-driven update. It inventories the
installed version, enabled state, source, scope, pins, local edits, duplicates,
and active caches before asking once for the exact writes. A current install is
a valid no-op. Preserve the channel already in use and read
[CHANGELOG.md](../CHANGELOG.md) before changing it.

The exact per-harness refresh and registration commands, the `HEAD` comparison
against the approved release tag, and the runtime markers to ignore live in one
place, the skill's
[channels reference](../plugins/megapowers/skills/upgrading-megapowers/references/channels.md).
Two rules from it apply to any manual update: a registration without a ref
tracks `main` and receives unreleased commits, so re-register with `--ref
release` (Codex) or `@release` (Claude) if the marketplace list shows no ref;
and after a marketplace refresh, require the reported `HEAD` to equal the
approved release commit before registering the new snapshot. Restart before
expecting new guidance. Do not delete an older cache while a live session may
still use it. A pinned local checkout changes only when you deliberately replace
or update that checkout.

## Uninstall

Claude Code:

```bash
claude plugin uninstall megapowers@megapowers
claude plugin marketplace remove megapowers
```

Codex:

```bash
codex plugin remove megapowers@megapowers
codex plugin marketplace remove megapowers
```

The `autonomous-run` skill may create ignored `.megapowers/run/<id>/` state in a
project. Plugin removal does not delete that project history.

## Validate a checkout

```bash
scripts/validate.sh
bash evals/run-all.sh --json results.jsonl
claude plugin validate --strict .claude-plugin/marketplace.json
claude plugin validate --strict plugins/megapowers
```

These checks validate structure and deterministic regressions. They do not
measure agent behavior. Optional behavioral studies are described in
[advanced/evals.md](./advanced/evals.md).

## Diagnose an install

Ask for `megapowers-doctor` (`/megapowers:megapowers-doctor` on Claude Code, or
the skill name after `$` as the Codex skills list shows it). It runs the
plugin's deterministic `doctor` command and reports the plugin version and
root, the harness, the Go toolchain version against the cached runner, the
Claude `outputStyle` value in effect, whether the hooks are registered, and how
to check your transcripts for skill loads.
