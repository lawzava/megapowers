# Fixes by check

- Output style, Claude Code: the style is optional and active only when
  settings set `outputStyle` to exactly `megapowers:Megapowers`. Bare
  `Megapowers` does not resolve and leaves the style silently off. Fix: run
  `/config`, open Output style, and select `megapowers:Megapowers`, or set
  `"outputStyle": "megapowers:Megapowers"` in the settings file the check
  names. Start a new session.
- Output style, Codex: the trusted SessionStart hook adds the style unless
  `MEGAPOWERS_OUTPUT_STYLE=off` is set in the launching environment.
- Go toolchain or runner cache: hooks build a Go runner on first use and cache
  it under a name that includes the Go version. Go older than 1.25 cannot
  build it. Fix: install Go 1.25 or newer, then start a new session. If a stale
  runner still fails, delete the cache directory the check names.
- Hooks not registered: the plugin is installed but the harness has not loaded
  `hooks/hooks.json`. Fix: re-enable the plugin; on Codex review and trust the
  plugin hooks (trust is recorded per hook hash, so a release that changes a
  hook asks again); then restart the session.
- Version or root mismatch: the installed cache differs from the marketplace
  head. Use `upgrading-megapowers`.
- Session stores: Claude Code keeps transcripts under
  `~/.claude/projects/<project>/`; Codex keeps rollouts under
  `~/.codex/sessions/<year>/<month>/<day>/`.
