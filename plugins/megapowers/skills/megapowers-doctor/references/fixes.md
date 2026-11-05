# Fixes by check

- Manifest versions differ, or version unexpected: use `upgrading-megapowers`.
- Unreadable manifest, catalog, skill body, or registry template: confirm the
  installed plugin directory is complete. Use `upgrading-megapowers` to repair it.

For [manual checks](directory-edition.md):

- Output style, Claude Code: run `/config`, open Output style, and select
  `megapowers:Megapowers`, or ask the user which style is selected. The style
  is optional; its setting is `"outputStyle": "megapowers:Megapowers"`.
  Start a new session after changing it.
- Output style, Codex: invoke `humanizing-prose` for writing tasks that need
  its editing guidance.
- Skill discovery: ask for the skill explicitly and confirm its body loads.
  A load alone does not establish that the skill changed the outcome.
