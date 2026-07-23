# Provider: Claude (headless CLI)

The different-vendor route when a non-Anthropic model leads. When Claude Code
itself is the lead, use its native subagents instead; this channel exists for a
non-Claude lead (or a wrapper agent) that needs an Anthropic pass.

## Channel

- Independent one-shot with an API key: `claude -p "<prompt>" --bare --model
  <id> --effort <level>`. With OAuth, `delegate-run` copies only the credential
  into a disposable `CLAUDE_CONFIG_DIR` and runs from a disposable directory;
  this excludes user plugins, hooks, memory, and project instructions, though
  enterprise-managed configuration may still apply. Keep the prompt immediately after `-p`:
  `--tools` is variadic and can otherwise consume it as a tool name. Safe mode excludes ambient Claude
  `--bare` excludes project instructions, plugins, hooks, and MCP servers; put the task,
  output contract, verification, constraints, and any required project-guidance
  paths in the prompt. The effort flag speaks the same
  low/medium/high/xhigh/max scale as the catalog's [efforts], so the resolved
  EFFORT value passes through unmapped. For machine-checkable output add
  `--output-format json` and state the required JSON shape in the prompt.
- Read-only reviews: append `--permission-mode plan --tools Read,Glob,Grep`
  after the other flags, pass the artifact inline or by path, and instruct no
  edits; the lead applies fixes.
- Independent review is deliberately one-shot. Use a separate non-review
  workflow if a task genuinely needs a resumable session.

## Prompting

Because both modes remove ordinary ambient context, make the prompt self-contained. Use
the same contract shape as any delegate dispatch: task, output contract,
verification, constraints, and the exact paths it may read. For adversarial
verification reuse the review output schema documented in [codex.md](codex.md);
the schema is vendor-neutral.
