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
- Agent registry: `orchestrating` ignores
  `~/.config/megapowers/agent-capabilities.md` when it is expired or has no
  readable `expires_at`, and falls back to native defaults. Fix: recheck the
  bindings, then set `refreshed_at` to today and `expires_at` about a month
  out. The orchestrating skill ships a template in `assets/`.
- Version or root mismatch: the installed cache differs from the marketplace
  head. Use `upgrading-megapowers`.
- Skill loads: the `skill loads` line counts this workspace's sessions from
  the last seven days that loaded each skill, with the same detection the
  gates use, on both harnesses. A skill missing from the line did not load in
  any of them; check its trigger with the user's own wording before changing
  its text. For other workspaces or periods, use the session miner in
  `writing-agent-instructions`.
- No hook run: `no megapowers hook has run on <harness>` means the harness
  never executed the hooks in this cache. On Codex, review and trust the
  plugin hooks, then start a new session. On Claude Code, check that the
  plugin is enabled.
- Marketplace copy differs: the harness refreshed its marketplace but not the
  install. Use `upgrading-megapowers`.
- Completion or effect gate: a `PreToolUse` denial that names
  `verify-and-finish` or `safe-effects` is the once-per-session gate, not a
  fault. Load the named skill and run the command again; the retry passes. A
  session without an ID or with an unwritable hook cache gets a reminder
  instead of the stop.
