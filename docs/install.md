# Install megapowers

megapowers supports current Claude Code and Codex. Choose one native channel per
harness so each skill registers once.

## Requirements

- A current Claude Code or Codex CLI, already authenticated.
- Git, for marketplace snapshots.
- `jq`, for the destructive-command hook.
- Go 1.25 or newer, only when using the independent-review tool or repository
  validation.

## Claude Code

```bash
claude plugin marketplace add lawzava/megapowers
claude plugin install megapowers@megapowers
```

Verify registration:

```bash
claude plugin list --json
```

Start a fresh session. Ask Claude Code to load `humanizing-prose` and summarize
its preservation rules. This checks discovery and full skill loading without
claiming broader behavioral quality.

While megapowers is enabled, Claude Code automatically applies its direct,
concise output style. The style preserves built-in coding instructions and
overrides another selected output style. Disable the plugin before starting a
session that needs a different style.

For a project-scoped install, add `--scope project` to both marketplace and
install commands. For a local user-only install, use `--scope local`.

## Codex

```bash
codex plugin marketplace add lawzava/megapowers
codex plugin add megapowers@megapowers
```

Verify registration:

```bash
codex plugin marketplace list --json
codex plugin list --json
```

Start a fresh session, then review and trust the plugin hooks when Codex asks.
Codex skips both hook features until they are trusted. Once trusted, the startup
hook applies the shared direct, concise style without editing user config.

Ask Codex to load `humanizing-prose` and summarize its preservation rules. This
checks skill discovery separately from the startup style.

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

Pass that checkout path instead of `lawzava/megapowers` to the marketplace add
command. Verify the selected commit before trusting it:

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

Before approving a floating update, resolve the latest stable tag to its commit
and compare it with the observed marketplace repository's default-branch head.
Stop if they differ; marketplace refresh follows the branch snapshot and must
not install unreleased branch state as a stable upgrade.

Claude Code:

```bash
claude plugin marketplace update megapowers
git -C <marketplace-install-location> rev-parse HEAD
claude plugin update megapowers@megapowers --scope <scope>
```

Replace `<scope>` with the scope reported by `claude plugin list --json`; do not
infer or change it during an update. Get `<marketplace-install-location>` from
`claude plugin marketplace list --json`.

Codex:

```bash
codex plugin marketplace upgrade megapowers --json
git -C <marketplace-install-location> rev-parse HEAD
codex plugin add megapowers@megapowers --json
```

Marketplace refresh updates Codex's source snapshot; `plugin add` registers the
new snapshot as the installed cache. Get its marketplace root from
`codex plugin marketplace list --json`. After either marketplace refresh,
require the reported `HEAD` to still equal the approved stable commit before
running `plugin update/add`; otherwise stop with the installed plugin untouched.

Claude's `plugin list --json` reports scope and install path; Codex's reports
source, enabled state, and version, while its `installedPath` comes from the
`plugin add --json` result. Compare that exact cache with the target ref. Ignore
harness-owned `.codex-marketplace-install.json` and `.in_use` markers during
source-edit and byte-parity checks. Restart before expecting new guidance. Do
not delete an older cache while a live session may still use it. A pinned local
checkout changes only when you deliberately replace or update that checkout.

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
