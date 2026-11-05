---
name: megapowers-doctor
description: "Use when Megapowers seems inactive or broken: a skill did not fire, the output style is missing, or the user asks to check the Megapowers setup."
when_to_use: "Trigger phrases: megapowers not working, skill did not fire, output style missing, check megapowers setup, is megapowers installed, megapowers doctor."
metadata:
  short-description: Diagnose a Megapowers installation and give the exact fix
---

# Megapowers Doctor

The plugin root is two directories above this skill. Use your normal file
tools to follow the [manual checks](references/directory-edition.md): read the
plugin manifest version, `skills/catalog.json` and each listed skill's
`SKILL.md`, and the shipped agent registry template. Report missing or unreadable
files and give one fix per finding from [fixes by check](references/fixes.md).

On Claude Code, check the optional output style with `/config` or ask the user which style
is selected. Report checks you could not perform as skipped.

From a megapowers checkout, maintainers can run `go run ./scripts/cmd/doctor --root <plugin-root>`.

## Did a skill fire?

Ask for the skill explicitly and confirm its body loads. A catalog entry or
prose mention alone does not establish a load. If an explicit request fails,
check discovery. If a matching task never loads a discoverable skill, use
`writing-agent-instructions` to inspect its trigger wording. If the skill
loads but the outcome does not change, report that observation separately.
