# Install megapowers

megapowers supports current Claude Code and Codex. Install it through one
channel per harness so each skill registers once. Register the marketplace at
the `release` branch: that branch only fast-forwards to signed release tags, so
the automatic marketplace refresh that both harnesses run never installs
unreleased `main`.

## Requirements

- A current Claude Code or Codex CLI, already authenticated.
- Git, for marketplace snapshots.
- Go 1.25 or newer, for the plugin's bundled Go tools, including the doctor.
  Loading skill guidance alone does not require Go.

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
its preservation rules. This checks that the skill is discovered and fully
loaded; it says nothing about broader behavior.

The output style is optional and off until selected. Select the
plugin-qualified value `megapowers:Megapowers` in `/config` under Output style,
or run `/output-style megapowers:Megapowers`. The resulting setting is
`"outputStyle": "megapowers:Megapowers"`. The bare value `Megapowers` does not
resolve to the plugin style. The style preserves built-in coding instructions.
To opt out while keeping the plugin enabled, select another style. Start a new
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

Start a fresh session. Ask Codex to load `humanizing-prose` and summarize its
preservation rules. This checks skill discovery. The plugin does not apply a
session-wide Codex output style.

## Developing this repository

The checkout exposes no skills of its own, so working in it never lists a
skill twice. To try source changes, load the plugin from the checkout for one
session or install it into an isolated home.

Claude Code:

```bash
claude --plugin-dir plugins/megapowers
```

Codex, with a separate `CODEX_HOME` so the release install stays untouched:

```bash
export CODEX_HOME="$TMPDIR/codex-megapowers-dev"
codex plugin marketplace add "$PWD"
codex plugin add megapowers@megapowers
```

The local marketplace is `.agents/plugins/marketplace.json`. The new home
needs its own `codex login`.

## Directory edition

The directory edition is a skills-only ZIP for the OpenAI Plugins
Directory, shared by ChatGPT and Codex. It is generated from
`plugins/megapowers`; the plugin installed from the marketplace is unchanged.

```bash
go run ./scripts/cmd/maintainer directory-edition --out "$TMPDIR/megapowers-directory"
```

The command writes `megapowers-<version>.zip` and prints its size and sha256.
The same input gives the same bytes. `--check` builds and validates without
keeping the output, and `scripts/validate.sh` runs it.

The edition ships the skills and their bundled scripts, an icon, and a
`.codex-plugin/plugin.json` for the directory listing. It omits the Claude Code
output style and manifest, the source README, and `skills/catalog.json`.

Each packaged `SKILL.md` ends with one line that ranks explicit user
instructions above skill guidance. Skills that run bundled Go scripts need a
local shell and Go; without them each skill states a manual fallback or
stops without sending anything. The skill `upgrading-megapowers` drives the
Claude Code and Codex plugin CLIs and does not apply to directory installs.

Install from the self-hosted marketplace for Claude Code's optional native
output style. Submission to the directory is a manual step in the OpenAI portal.

## Pin a release

A local immutable checkout makes the selected source explicit for either
harness. Replace `vX.Y.Z` with a reviewed published tag; do not run the
placeholder literally.

```bash
release_tag=vX.Y.Z
git clone --branch "$release_tag" --depth 1 \
  https://github.com/lawzava/megapowers.git "megapowers-$release_tag"
```

Pass that checkout path instead of `lawzava/megapowers@release` (Claude) or
`lawzava/megapowers --ref release` (Codex) to the marketplace add command.
Check the selected commit before trusting it:

```bash
git -C "megapowers-$release_tag" rev-parse HEAD
git -C "megapowers-$release_tag" status --short
```

## Update

Use `upgrading-megapowers` for an agent-driven update. It inventories the
installed version, enabled state, source, scope, pins, local edits, duplicates,
and active caches before asking once for the exact writes. A current install is
a valid no-op. Keep the channel already in use, and read
[CHANGELOG.md](../CHANGELOG.md) before changing it.

The exact per-harness refresh and registration commands, the `HEAD` comparison
against the approved release tag, and the runtime markers to ignore are in the
skill's
[channels reference](../plugins/megapowers/skills/upgrading-megapowers/references/channels.md).
Two rules from it apply to any manual update:

- A registration without a ref tracks `main` and receives unreleased commits.
  If the marketplace list shows no ref, re-register with `--ref release`
  (Codex) or `@release` (Claude).
- After a marketplace refresh, require the reported `HEAD` to equal the
  approved release commit before registering the new snapshot.

After upgrading from v0.45.x and closing old sessions, the compiled runner cache under `<user cache dir>/megapowers-hooks` and Codex hook-trust entries are unused and safe to delete.

Restart before expecting new guidance. Do not delete an older cache while a
live session may still use it. A pinned local checkout changes only when you
deliberately replace or update that checkout.

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
plugin's standalone `skills/megapowers-doctor/scripts/doctor.go` script and reports the plugin version and root, the
harness, the Claude `outputStyle` value in effect, the local marketplace copy,
recent workspace skill-load counts, and the optional agent registry. The
directory edition omits the catalog and skips load counting. The doctor reads
local settings and session records; see [SECURITY.md](../SECURITY.md) for its
read and execution boundaries.
