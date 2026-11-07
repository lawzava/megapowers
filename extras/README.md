# Repository-only extras

These helpers and skills do not ship with the plugin or its HTTP skill catalog.
The plugin uses manual procedures with normal agent tools and needs no Go runtime.

## Helpers

Nine Go helpers moved unchanged into `extras/scripts/<skill>/`, preserving nested paths.
Use Go 1.25 or newer from this checkout, for example:

```bash
go run ./extras/scripts/megapowers-doctor/doctor.go --root ./plugins/megapowers --harness codex
```

The helpers retain their command interfaces. Read the source before running
them. Contract tests live under `scripts/contracts/` and
`scripts/internal/maintain/`.

## Skills

`skills/megapowers-doctor/`, `skills/upgrading-megapowers/`, and
`skills/mcp-setup/` retain their original text for repository-only use. Their
old plugin-relative helper paths are historical; run helpers from `extras/scripts/`
as shown above and supply the plugin root explicitly. These skills are not
part of the supported shipped set.

## OpenCode catalog maintenance

After changing a shipped skill or plugin version, regenerate the catalog:

```bash
go run ./scripts/cmd/maintainer opencode-index
scripts/validate.sh
```

Each entry uses the plugin version. Bump it when publishing changed skill files
so OpenCode refreshes its HTTP cache. Hidden local files and directories are
excluded from the catalog.
