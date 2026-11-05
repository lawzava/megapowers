---
name: megapowers-doctor
description: "Use when Megapowers seems inactive or broken: a skill did not fire, the output style is missing, or the user asks to check the Megapowers setup."
when_to_use: "Trigger phrases: megapowers not working, skill did not fire, output style missing, check megapowers setup, is megapowers installed, megapowers doctor."
metadata:
  short-description: Diagnose a Megapowers installation and give the exact fix
---

# Megapowers Doctor

The plugin root is two directories above this skill. Use normal file and
session tools for the [manual checks](references/directory-edition.md).
Identify the harness with [harness invocation](references/invocation.md).
Inspect the plugin version and root, marketplace copy, skill discovery,
applicable output-style setting, recent successful skill loads, and optional
agent registry. Report a problem as WARN with its evidence and one fix from
[fixes by check](references/fixes.md). Report inaccessible inputs as unchecked.

## Did a skill fire?

A skill fired only if the session loaded its body; a catalog entry or a
prose mention is not a load. Inspect the session trace to find successful load records. Report
counts and session identifiers, not contents. Count sessions modified in the last seven days that contain a successful load.

If a matching task never loaded the skill, the defect is trigger wording or
skill discovery; use `writing-agent-instructions`
for the trigger. If the skill loaded but the outcome did not change, say so; a
load is not evidence of behavior.
