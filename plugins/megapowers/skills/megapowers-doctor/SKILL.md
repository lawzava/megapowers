---
name: megapowers-doctor
description: "Use when Megapowers seems inactive or broken: a skill did not fire, the output style is missing, a hook errors or times out, or the user asks to check the Megapowers setup."
when_to_use: "Trigger phrases: megapowers not working, skill did not fire, output style missing, hook error, check megapowers setup, is megapowers installed, megapowers doctor."
metadata:
  short-description: Diagnose a Megapowers installation and give the exact fix
---

# Megapowers Doctor

Run the deterministic check first. The plugin root is two directories above
this skill's directory:

```bash
"<plugin root>/hooks/run-hook.cmd" doctor
```

It prints the plugin version and root, the detected harness, the Go toolchain
against the cached hook runner, the output-style setting, and the registered
hook events. Problems start with `WARN`. Quote only those lines and give one
fix per line from [fixes by check](references/fixes.md). If the command itself
fails, report its exit status and raw error; that failure is the diagnosis.

## Did a skill fire?

A skill fired only if the session loaded its body; a catalog entry or a
prose mention is not a load. Search the harness session store without
printing transcripts and count only load records: a skill tool call naming
the skill, an injected skill body, or a tool call that reads
`skills/<name>/SKILL.md`.
The [session-store entry](references/fixes.md) gives the exact search per
harness. Report counts and session identifiers, not contents.

If a matching task never loaded the skill, the defect is trigger wording or
the startup reminder, not the installation; use `writing-agent-instructions`
for the trigger. If the skill loaded but the outcome did not change, say so; a
load is not evidence of behavior.
