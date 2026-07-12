# Provider: Claude (headless CLI)

The different-vendor route when a non-Anthropic model leads. When Claude Code
itself is the lead, use its native subagents instead; this channel exists for a
non-Claude lead (or a wrapper agent) that needs an Anthropic pass.

## Channel

- One-shot: `claude -p "<prompt>"`. Pin the model and effort from the resolved
  route with `--model <id>` and `--effort <level>`; the flag speaks the same
  low/medium/high/xhigh/max scale as the catalog's [efforts], so the resolved
  EFFORT value passes through unmapped. For machine-checkable output add
  `--output-format json` and state the required JSON shape in the prompt.
- Read-only reviews: pass the artifact inline or by path and instruct no edits;
  the review changes nothing, the lead applies fixes.
- Continuing a thread: `claude -p --resume <session-id>`.

## Prompting

Same contract shape as any delegate dispatch: task, output contract,
verification, constraints. For adversarial verification reuse the review output
schema documented in [codex.md](codex.md); the schema is vendor-neutral.
