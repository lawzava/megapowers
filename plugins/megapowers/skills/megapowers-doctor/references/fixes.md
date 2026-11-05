# Fixes by check

- Output style, Claude Code: the style is optional and active only when
  settings set `"outputStyle": "megapowers:Megapowers"`. Bare
  `Megapowers` does not resolve. Run `/config`, open Output style, and select
  `megapowers:Megapowers`, or set that value in the settings file the check
  names. Start a new session.
- Output style, Codex: the plugin does not apply a session-wide style. Invoke
  `humanizing-prose` for writing tasks that need its editing guidance.
- Go toolchain: the doctor and other bundled scripts need Go 1.25 or newer.
  Install it before running the script again, or use the
  [manual checks](directory-edition.md).
- Agent registry: `orchestrating` ignores
  `~/.config/megapowers/agent-capabilities.md` when it is expired or has no
  readable `expires_at`, and falls back to native defaults. Recheck the
  bindings, then set `refreshed_at` to today and `expires_at` about a month
  out. The orchestrating skill ships a template in `assets/`.
- Version or root mismatch, or a newer local marketplace copy: use
  `upgrading-megapowers`.
- Skill loads: the doctor counts successful body-load records in recent workspace transcripts. For other workspaces or periods, use the
  session miner in `writing-agent-instructions`.
