# Upgrade Channels

Use one channel per harness. Replace angle-bracket values with values observed
during inspection. Commands under **Apply after approval** write outside the
repository.

## Claude Code marketplace

### Inspect: read only

```bash
claude plugin marketplace list --json
claude plugin list --available --json
```

Record each installed plugin's `id`, `version`, `scope`, `enabled`, and
`installPath`. Compare it with available records from the same
`marketplaceName`. Inspect the marketplace source and changelog before
selecting a target. An installed plugin is a managed copy, so local edits can
be overwritten.

### Apply after approval

For a floating marketplace source:

```bash
claude plugin marketplace update <marketplace>
claude plugin update <plugin>@<marketplace> --scope <user|project|local|managed>
```

Repeat the update command only for the approved installed set. Install approved
additions separately:

```bash
claude plugin install <plugin>@<marketplace> --scope <user|project|local>
```

An update requires a restart to load. After restart, rerun `claude plugin list
--available --json` and verify versions, scope, enabled state, and component
discovery.

Do not use the floating update path to move a pinned marketplace ref. Preserve
its pin and present the exact old and new refs. Ref replacement is not an
atomic generic CLI operation. Inspect `claude plugin marketplace add --help`
for the installed CLI, then include the source transition and recovery path in
the approval plan.

## Codex marketplace

### Inspect: read only

```bash
codex plugin marketplace list --json
codex plugin list --available --json
codex --version
codex app-server daemon version
```

Record `pluginId`, `version`, `enabled`, `source`, `marketplaceName`,
`marketplaceSource`, and `installPolicy`. A configured Git ref is a pin even
when the plugin selector itself has no version.

### Apply after approval

For a floating Git marketplace:

```bash
codex plugin marketplace upgrade <marketplace> --json
codex plugin add <plugin>@<marketplace> --json
```

Re-add only the approved installed set, then approved additions. A ref-pinned
marketplace upgrade refreshes that ref; it does not authorize changing the ref.
Moving to a newer tag while staying pinned requires an approved marketplace
source transition. Snapshot the installed set and old source first, verify
current `codex plugin marketplace add --help` syntax, and include restoration
of the old source or ref in the recovery plan.

Start a fresh Codex session after changes. Rerun the four inspection commands,
confirm expected skills load, inspect `/hooks`, and leave changed hook hashes
untrusted until separately approved. If CLI and app-server versions differ,
diagnose the running process and restart it before claiming the plugin loaded.

## Skills CLI

### Inspect: read only

Inspect the project `skills-lock.json` and the relevant installed skill
directories. Identify whether the install is project-local or global and
whether shared directories would cause duplicate registration.

### Apply after approval

```bash
npx skills update <approved-skill>... -p -y
npx skills update <approved-skill>... -g -y
```

Use `-p` only for the observed project install and `-g` only for the observed
global install. Name every approved skill. Bare `npx skills update` prompts for
scope and updates all skills in that scope; `-y` without `-p` or `-g`
auto-detects scope. Verify the lock file and installed directories. Treat newly
available skills as optional additions. Never widen the target agents or switch
scope implicitly.

## Symlinked checkout

### Inspect: read only

```bash
git -C <checkout> status --short --branch
git -C <checkout> remote -v
git -C <checkout> tag --points-at HEAD
```

Confirm every symlink resolves into that checkout. A dirty tree or ambiguous
upstream is a stop condition.

### Apply after approval

For a clean floating branch with an upstream:

```bash
git -C <checkout> pull --ff-only
```

For an explicit pin, fetch and select only the approved tag or ref while
remaining pinned. Verify the checkout ref and every symlink target. Copied
skills are not symlinks. Update only the approved copied directories and verify
them separately.

## OpenCode

OpenCode has no marketplace. An inspector that knows only the two marketplace
channels reports "not installed" for a harness that is, so check the paths.

### Inspect: read only

```bash
opencode --version
ls -l ~/.config/opencode/plugins/ ~/.config/opencode/skills/
readlink -f ~/.config/opencode/plugins/*.js ~/.config/opencode/skills/*
jq '.plugin, .instructions' ~/.config/opencode/opencode.json
```

Record where each link resolves. A clone kept for installs and a development
checkout are different classifications: a link into a working tree is the
Symlinked checkout channel above, dirty-tree stop condition included. A regular
file where a link belongs is a copy, which for the megapowers plugins is a
misinstall rather than a style choice. Files the user wrote (`AGENTS.md`, the
`opencode.json` permission block, agent role files) are user-owned baselines,
not managed copies.

### Apply after approval

Track a published tag from an install clone, so an upgrade is a ref move and
every symlink keeps pointing at the same path:

```bash
git -C <clone> fetch --tags
git -C <clone> tag -v <tag>
git -C <clone> checkout <tag>
```

Repoint links only when the approved plan moves the source directory, and then
verify every one of them, plugins and skills alike.

Symlink, never copy. Both plugins resolve sibling scripts relative to their own
realpath, and node resolves an ESM specifier to that realpath, so a copy points
at a directory that does not exist: the session catalog then injects nothing and
the guardrail refuses to load. One of those is silent.

Skills reach OpenCode through this channel or the skills CLI, never both. On a
machine that also runs the Claude Code plugins, do not take the global skills
CLI path: it installs OpenCode's skills into the shared `~/.agents/skills/`,
which Claude Code also scans, and every skill registers twice. Plugins and hooks
do not travel that channel at all.

Restart OpenCode; plugins load at session start. Verify by resolving each link
into the clone at the approved ref and importing both plugin modules, which is
what distinguishes a loadable install from a copied one:

```bash
cd ~/.config/opencode/plugins && node --input-type=module \
  -e "console.log(Object.keys(await import('./session-catalog.js')))"
```

## Fork

Inspect status, remotes, current branch, divergence, and local changes. Propose
merge or rebase based on the fork's existing policy. After approval, work on a
feature branch, fetch the named upstream, integrate the approved stable tag or
branch, and run the fork's validators. Never reset, overwrite, or replace the
fork with the upstream tree.

## Baseline drift

No plugin ships `templates/`, so the baselines come from the repository. Read
only; fetching changes nothing local.

```bash
base=https://raw.githubusercontent.com/lawzava/megapowers
for f in CLAUDE.md CODEX.md OPENCODE.md settings.example.json; do
  curl -fsS "$base/v<installed>/templates/$f" -o "<tmp>/from-$f"
  curl -fsS "$base/v<target>/templates/$f" -o "<tmp>/to-$f"
done
diff -u "<tmp>/from-CLAUDE.md" "<tmp>/to-CLAUDE.md"
```

Use `$TMPDIR`, not a hard-coded path. Fetch the pinned tag for a pinned
install, never the default branch. A fork or symlinked checkout already has
`templates/` locally: diff `git -C <checkout> show
v<installed>:templates/<file>` against the working copy instead of fetching,
and say which source you used.

Settings compare key by key, not as text:

```bash
jq -S 'keys' "<tmp>/to-settings.example.json"
jq -S --slurpfile u ~/.claude/settings.json '[keys[] | select(. as $k | $u[0] | has($k) | not)]' "<tmp>/to-settings.example.json"
```

`curl` exit 22, 6, or 28 means the check did not run. Report that, with the
reason, in place of a drift result. Do not fall back to the default branch to
make a pinned check succeed; the answer would describe a version the user is
not on.

## Partial failure

After any failed write, stop the sequence and rerun the channel's inspection
commands. Report observed applied, failed, and not-attempted actions. Do not
proceed to optional additions and do not claim rollback unless the old state
was restored and verified.
