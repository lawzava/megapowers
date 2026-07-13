# Upgrade Channels

Use one channel per harness. Replace angle-bracket values with values observed during inspection. Commands under **Apply after approval** write outside the repository.

## Claude Code marketplace

### Inspect: read only

```bash
claude plugin marketplace list --json
claude plugin list --available --json
```

Record each installed plugin's `id`, `version`, `scope`, `enabled`, and `installPath`. Compare it with available records from the same `marketplaceName`. Inspect the marketplace source and changelog before selecting a target. An installed plugin is a managed copy, so local edits can be overwritten.

### Apply after approval

For a floating marketplace source:

```bash
claude plugin marketplace update <marketplace>
claude plugin update <plugin>@<marketplace> --scope <user|project|local|managed>
```

Repeat the update command only for the approved installed set. Install approved additions separately:

```bash
claude plugin install <plugin>@<marketplace> --scope <user|project|local>
```

An update requires a restart to load. After restart, rerun `claude plugin list --available --json` and verify versions, scope, enabled state, and component discovery.

Do not use the floating update path to move a pinned marketplace ref. Preserve its pin and present the exact old and new refs. Ref replacement is not an atomic generic CLI operation. Inspect `claude plugin marketplace add --help` for the installed CLI, then include the source transition and recovery path in the approval plan.

## Codex marketplace

### Inspect: read only

```bash
codex plugin marketplace list --json
codex plugin list --available --json
codex --version
codex app-server daemon version
```

Record `pluginId`, `version`, `enabled`, `source`, `marketplaceName`, `marketplaceSource`, and `installPolicy`. A configured Git ref is a pin even when the plugin selector itself has no version.

### Apply after approval

For a floating Git marketplace:

```bash
codex plugin marketplace upgrade <marketplace> --json
codex plugin add <plugin>@<marketplace> --json
```

Re-add only the approved installed set, then approved additions. A ref-pinned marketplace upgrade refreshes that ref; it does not authorize changing the ref. Moving to a newer tag while staying pinned requires an approved marketplace source transition. Snapshot the installed set and old source first, verify current `codex plugin marketplace add --help` syntax, and include restoration of the old source or ref in the recovery plan.

Start a fresh Codex session after changes. Rerun the four inspection commands, confirm expected skills load, inspect `/hooks`, and leave changed hook hashes untrusted until separately approved. If CLI and app-server versions differ, diagnose the running process and restart it before claiming the plugin loaded.

## Skills CLI

### Inspect: read only

Inspect the project `skills-lock.json` and the relevant installed skill directories. Identify whether the install is project-local or global and whether shared directories would cause duplicate registration.

### Apply after approval

```bash
npx skills update <approved-skill>... -p -y
npx skills update <approved-skill>... -g -y
```

Use `-p` only for the observed project install and `-g` only for the observed global install. Name every approved skill. Bare `npx skills update` prompts for scope and updates all skills in that scope; `-y` without `-p` or `-g` auto-detects scope. Verify the lock file and installed directories. Treat newly available skills as optional additions. Never widen the target agents or switch scope implicitly.

## Symlinked checkout

### Inspect: read only

```bash
git -C <checkout> status --short --branch
git -C <checkout> remote -v
git -C <checkout> tag --points-at HEAD
```

Confirm every symlink resolves into that checkout. A dirty tree or ambiguous upstream is a stop condition.

### Apply after approval

For a clean floating branch with an upstream:

```bash
git -C <checkout> pull --ff-only
```

For an explicit pin, fetch and select only the approved tag or ref while remaining pinned. Verify the checkout ref and every symlink target. Copied skills are not symlinks. Update only the approved copied directories and verify them separately.

## Fork

Inspect status, remotes, current branch, divergence, and local changes. Propose merge or rebase based on the fork's existing policy. After approval, work on a feature branch, fetch the named upstream, integrate the approved stable tag or branch, and run the fork's validators. Never reset, overwrite, or replace the fork with the upstream tree.

## Partial failure

After any failed write, stop the sequence and rerun the channel's inspection commands. Report observed applied, failed, and not-attempted actions. Do not proceed to optional additions and do not claim rollback unless the old state was restored and verified.
