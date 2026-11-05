# Optional helpers

The v0.52.0 directory-review probe keeps `skills/mcp-setup/` here unchanged
for repository-only use. It is not shipped with the plugin.

The v0.51.0 directory-review probe moves nine Go helpers from the plugin into
`extras/scripts/<skill>/`, unchanged. Nested paths within each former skill scripts
folder stay intact. These helpers are not installed with the plugin.

Use Go 1.25 or newer. Invoke a helper from the repository checkout, for example:

```bash
go run ./extras/scripts/megapowers-doctor/doctor.go --root ./plugins/megapowers --harness codex
```

Each helper retains its existing command interface. Read its source before
running it. Contract tests remain under `scripts/contracts` and
`scripts/internal/maintain`. To reverse the probe, revert the probe commit;
this restores helper locations and the corresponding guidance together.
