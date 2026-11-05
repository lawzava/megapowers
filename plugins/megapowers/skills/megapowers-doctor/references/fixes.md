# Fixes by check

- Version or root mismatch: use `upgrading-megapowers`.
- Unreadable manifest, catalog, skill body, or registry template: confirm the
  installed plugin directory is complete. Use `upgrading-megapowers` to repair it.
- Go toolchain: bundled scripts need Go 1.25 or newer. Install it before
  running the script again, or use the [manual checks](directory-edition.md).

For manual checks:

- Output style, Claude Code: run `/config`, open Output style, and select
  `megapowers:Megapowers`, or ask the user which style is selected. The style
  is optional; its setting is `"outputStyle": "megapowers:Megapowers"`.
  Start a new session after changing it.
- Output style, Codex: invoke `humanizing-prose` for writing tasks that need
  its editing guidance.
- Skill discovery: ask for the skill explicitly and confirm its body loads.
  A load alone does not establish that the skill changed the outcome.
