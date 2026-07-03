# Security

## Scope

Nothing in this repository is a security boundary.

- `mega-guardrails`' `deny-destructive` hook is a tripwire against accidents,
  not a sandbox. It string-matches obviously catastrophic commands and can be
  bypassed by anyone trying. Real containment comes from your runtime's
  sandbox and OS permissions.
- The `effect-broker` skill gates irreversible actions by declared action
  class; a model that misdeclares is not stopped by it.
- Hooks are Claude Code only and fail open by absence on other harnesses.

If your threat model includes a malicious or compromised model, none of these
help; use OS-level sandboxing.

## Reporting

Found a bypass with real blast radius (e.g. the deny-destructive suite passes
a command that wipes a disk), a hook that fails closed and locks a session, or
anything that leaks user data? Open a GitHub issue. None of this repo's
components hold secrets or run as services, so coordinated disclosure is
normally unnecessary; if you believe your finding is sensitive anyway, say so
in a minimal issue and a private channel can be arranged.

Please include the exact command/input, the expected vs actual behavior, and
your runtime (Claude Code / Codex / OpenCode / Antigravity + version).
