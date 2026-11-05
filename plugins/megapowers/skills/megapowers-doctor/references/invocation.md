# Harness invocation

Claude Code:

```bash
go run "<plugin root>/skills/megapowers-doctor/scripts/doctor.go" --root "<plugin root>" --harness claude
```

Codex:

```bash
go run "<plugin root>/skills/megapowers-doctor/scripts/doctor.go" --root "<plugin root>" --harness codex
```

An explicit harness overrides environment detection. Without the flag, the
script uses the harness environment markers; outside a harness it reports
unknown and checks the local Claude output-style setting.
