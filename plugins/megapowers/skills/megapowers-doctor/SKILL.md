---
name: megapowers-doctor
description: "Use when Megapowers seems inactive or broken: a skill did not fire, the output style is missing, or the user asks to check the Megapowers setup."
when_to_use: "Trigger phrases: megapowers not working, skill did not fire, output style missing, check megapowers setup, is megapowers installed, megapowers doctor."
metadata:
  short-description: Diagnose a Megapowers installation and give the exact fix
---

# Megapowers Doctor

The plugin root is two directories above this skill. With Go 1.25 or newer, run:

```bash
go run "<plugin root>/skills/megapowers-doctor/scripts/doctor.go" --root "<plugin root>" --harness "<harness>"
```

Set `<harness>` using [harness invocation](references/invocation.md).
The optional harness flag labels the report.

The script checks only the plugin's own files: manifests, the optional catalog
and listed skill bodies, and the shipped agent registry template. Problems
start with `WARN`. Quote those lines and give one fix per line from
[fixes by check](references/fixes.md). If the command fails, report its exit
status and raw error; that failure is the diagnosis.

Without a local shell or Go, follow [manual checks](references/directory-edition.md).

Check the optional output style using [manual checks](references/directory-edition.md),
or ask the user which style is selected.

## Did a skill fire?

Ask for the skill explicitly and confirm its body loads. A catalog entry or
prose mention alone does not establish a load. If an explicit request fails,
check discovery. If a matching task never loads a discoverable skill, use
`writing-agent-instructions` to inspect its trigger wording. If the skill
loads but the outcome does not change, report that observation separately.
