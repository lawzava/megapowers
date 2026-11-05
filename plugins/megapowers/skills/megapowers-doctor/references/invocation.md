# Harness invocation

Run the doctor with the installed plugin root:

```bash
go run "<plugin root>/skills/megapowers-doctor/scripts/doctor.go" --root "<plugin root>"
```

Optionally pass `--harness claude` or `--harness codex` to label the report.
The label does not change the checks. Without `--root`, the script tries
`MEGAPOWERS_PLUGIN_ROOT`, `PLUGIN_ROOT`, then `CLAUDE_PLUGIN_ROOT`.

On Claude Code, check the optional output style with `/config` or ask the user
which style is selected. Confirm skill discovery by asking for a skill explicitly.
