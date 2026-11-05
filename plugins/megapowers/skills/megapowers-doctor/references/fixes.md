# Fixes by check

- Output style, Claude Code: the style is optional and active only when
  settings set `"outputStyle": "megapowers:Megapowers"`. Bare
  `Megapowers` does not resolve. Run `/config`, open Output style, and select
  `megapowers:Megapowers`, or set that value in the settings file the check
  names. Start a new session.
- Output style, Codex: the plugin does not apply a session-wide style. Invoke
  `humanizing-prose` for writing tasks that need its editing guidance.
- Inaccessible inputs: report unchecked files and use the harness's available
  read-only tools to inspect them.
  Follow the [manual checks](directory-edition.md).
- Agent registry: `orchestrating` ignores
  `~/.config/megapowers/agent-capabilities.md` when it is expired or has no
  readable `expires_at`, and falls back to native defaults. Recheck the
  bindings, then set `refreshed_at` to today and `expires_at` about a month
  out. The orchestrating skill ships a template in `assets/`.
- Version or root mismatch, or a newer local marketplace copy: use
  `upgrading-megapowers`.
- Skill loads: inspect successful body-load records in recent workspace traces.
  For other workspaces or periods, follow session mining in
  `writing-agent-instructions`.
