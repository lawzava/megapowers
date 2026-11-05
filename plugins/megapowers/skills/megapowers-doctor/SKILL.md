---
name: megapowers-doctor
description: "Use when Megapowers seems inactive or broken: a skill did not fire, the output style is missing, or the user asks to check the Megapowers setup."
when_to_use: "Trigger phrases: megapowers not working, skill did not fire, output style missing, check megapowers setup, is megapowers installed, megapowers doctor."
metadata:
  short-description: Diagnose a Megapowers installation and give the exact fix
---

# Megapowers Doctor

The plugin root is two directories above this skill's directory. With a local
shell and Go 1.25 or newer, run:

```bash
go run "<plugin root>/skills/megapowers-doctor/scripts/doctor.go" --root "<plugin root>"
```

It prints the plugin version and root, harness, output-style
setting, marketplace copy, recent skill loads, and optional agent registry. Problems start with
`WARN`. Quote only those lines and give one fix per line from
[fixes by check](references/fixes.md). If the command fails, report its exit
status and raw error; that failure is the diagnosis.

Without a local shell or Go, follow [manual checks](references/directory-edition.md).

## Did a skill fire?

A skill fired only if the session loaded its body; a catalog entry or a
prose mention is not a load. Inspect the session trace, or use the session
miner in `writing-agent-instructions`, to find successful load records. Report
counts and session identifiers, not contents. The doctor's `skill loads` line counts sessions with a successful load over the last seven days.

If a matching task never loaded the skill, the defect is trigger wording or
skill discovery; use `writing-agent-instructions`
for the trigger. If the skill loaded but the outcome did not change, say so; a
load is not evidence of behavior.
