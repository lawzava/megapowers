# Install megapowers

megapowers supports current Claude Code and Codex. Choose one native channel per
harness so each skill registers once.

## Requirements

- A current Claude Code or Codex CLI, already authenticated.
- Git, for marketplace snapshots.
- `jq`, for the destructive-command hook.
- Go 1.24 or newer, only when using the independent-review tool or repository
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

Start a fresh session. Ask Codex to load `humanizing-prose` and summarize its
preservation rules.

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

Read [CHANGELOG.md](../CHANGELOG.md), then update the channel already in use.

Claude Code:

```bash
claude plugin marketplace update megapowers
claude plugin update megapowers@megapowers
```

Codex:

```bash
codex plugin marketplace upgrade megapowers
```

Restart the harness and repeat the registration and skill-loading checks. A
pinned local checkout changes only when you deliberately replace or update that
checkout.

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
