# Provider: Codex

Channel mechanics and prompting guidance for dispatching to Codex (OpenAI).
Routing (which roles come here, tier, effort) lives in delegates.toml; a channel
can apply only the route fields it exposes. This file is how to reach the
backend and word the dispatch. Prompting guidance
adapted from OpenAI's own guidance for prompting Codex (codex-plugin-cc,
Apache-2.0, Copyright 2026 OpenAI), rewritten for this repo; provenance in
ATTRIBUTION.md.

## Channels

- Inside Codex: native subagents; spawn directly when same-model parallelism
  helps. Native v2 uses the current session model and effort, and
  `fork_turns = "none"` supplies fresh transcript context without changing
  either. If the resolved route requires a different Codex model or effort, use
  a role-aware surface or bounded `codex exec` run instead.
- From Claude Code: prefer OpenAI's first-party
  [`codex-plugin-cc`](https://github.com/openai/codex-plugin-cc). Its
  `/codex:review`, `/codex:adversarial-review`, `/codex:rescue`, and
  `/codex:transfer` surfaces wrap the local Codex CLI and app server, reusing
  the installed Codex authentication and configuration. Use its background job
  commands when Claude Code should remain the lead interface.
- From another harness under a sandboxed lead: the first-party MCP server
  (`codex mcp-server`, tools `codex` / `codex-reply`). The harness spawns the
  server outside its command sandbox, so Codex auth works even when that sandbox
  denies `~/.codex/auth.json`. No `profile` param over MCP: pin `model` and
  `config {model_reasoning_effort}` from the resolved route in each call.
- Unsandboxed: `codex exec` with explicit sandbox flags (`--sandbox read-only`
  for reviews, `--sandbox workspace-write` in a worktree for builds);
  `--output-schema <schema.json>` returns a machine-checkable verdict. The Codex
  SDK (`@openai/codex-sdk` on npm, `openai-codex` on PyPI) is the same channel
  from code, with the same auth caveat.
- Continuing a thread: `codex exec resume --last` (or `resume <session-id>`), a
  thread ID held by the SDK, or the `codex-reply` MCP tool. A bare `codex exec`
  starts a fresh thread each call.

## The stance

Prompt Codex like an operator, not a collaborator. Keep the prompt compact and
block-structured (XML tags work well): state the task, the output contract,
the follow-through defaults, and the small set of constraints that matter.
Prefer a better contract over more words: when output misses the bar, tighten
the output contract and verification rules before raising reasoning effort or
adding narrative explanation.

## Blocks that earn their place

- `<task>`: one paragraph, the goal and the definition of done.
- `<output_contract>`: the exact shape of the return (schema, sections, or
  diff format). Pair with `codex exec --output-schema` for a machine-checkable
  verdict.
- `<verification>`: what the delegate must run or check before answering, and
  what evidence to cite.
- `<constraints>`: only the constraints that change the work (files in scope,
  what not to touch, the sandbox preset from delegates.toml).
- `<context_gating>`: when required context might be missing, say what to do
  (ask, or stop and report) instead of letting it guess.

## Adversarial review template

For the `verify` and `code_review` roles. The reviewer's job is to break
confidence in the change, not to validate it: default to skepticism, give no
credit for good intent or likely follow-up work, and treat happy-path-only
behavior as a real weakness. Prefer one strong finding over several weak ones.

Attack surfaces to name in the prompt: auth and tenant isolation, data loss or
corruption, rollback and idempotency, race conditions, version skew and
migration, observability gaps.

## Review output schema

Request this shape (via `--output-schema` or the contract block) so the lead
can act on the verdict without re-parsing prose:

```json
{
  "verdict": "approve | needs-attention",
  "findings": [{
    "severity": "critical | major | minor",
    "file": "path",
    "lines": "start-end",
    "confidence": 0.0,
    "finding": "...",
    "recommendation": "..."
  }],
  "next_steps": ["..."]
}
```

One strong refutation outweighs any number of clean passes; the lead
re-verifies material fixes (see mega-orchestration:cross-model-verification).
