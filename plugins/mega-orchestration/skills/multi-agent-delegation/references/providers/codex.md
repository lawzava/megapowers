# Provider: Codex

Last reviewed: 2026-08-05.

Channel mechanics and prompting guidance for dispatching to Codex (OpenAI).
Routing (which roles come here, tier, effort) lives in delegates.toml; a channel
can apply only the route fields it exposes. This file is how to reach the
backend and word the dispatch.

Two upstream sources under different terms, and the difference matters because
only one of them is an open-source license. The stance, effort, preamble,
autonomy, and prompt-caching guidance follow OpenAI's [GPT-5.6 prompt
guidance](https://developers.openai.com/api/docs/guides/prompt-guidance-gpt-5p6),
a documentation page (Copyright 2026 OpenAI, all rights reserved) that a handful
of sentences below quote verbatim as short quotations. The adversarial-review
framing and the review output schema are adapted from codex-plugin-cc
(Apache-2.0, Copyright 2026 OpenAI), rewritten for this repo. Both sources have
their own entry in ATTRIBUTION.md; the Apache grant covers the plugin only and
does not reach the docs page, so do not relicense text from here on the strength
of it.

Advice attributed to models older than GPT-5.6 is deliberately absent rather
than merely unmentioned: the tightened contract-following described below is
what made that advice wrong, so carrying it forward would conflict with
everything under it.

- [Channels](#channels)
- [The stance](#the-stance)
- [Cut what the contract already says](#cut-what-the-contract-already-says)
- [Reasoning effort](#reasoning-effort)
- [Preambles](#preambles)
- [What the request authorizes](#what-the-request-authorizes)
- [A brief that fits](#a-brief-that-fits)
- [Blocks that earn their place](#blocks-that-earn-their-place)
- [Adversarial review template](#adversarial-review-template)
- [Review output schema](#review-output-schema)

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
block-structured (XML tags work well): state the task, the output contract, the
follow-through defaults, and the small set of constraints that matter. Prefer a
better contract over more words: when output misses the bar, tighten the output
contract and verification rules before raising reasoning effort or adding
narrative explanation.

## Cut what the contract already says

GPT-5-class models follow prompt contracts closely, so conflicting rules can
create more instability than missing detail. A dispatch brief is layered on top
of an output schema, a sandbox flag, and a resolved preset, all of which are
already binding. Restating any of them adds a second copy that drifts, and two
copies that disagree is a harder failure to debug than a rule that was never
stated. What to cut from a brief:

- "Return valid JSON with these fields." `--output-schema` carries the schema
  and the launcher re-validates the result. Name the schema, do not paraphrase
  it.
- "Do not modify any files" on a read_only route. `--sandbox read-only` enforces
  it. Repeating it invites the model to reason about which rule governs.
- "Be exhaustive" next to "prefer one strong finding over several weak ones".
  Those point opposite ways. Keep the one the role wants.
- House style, commit conventions, and repository rules the delegate's own
  configuration already loads.

What survives the cut is the part no other layer states: the task, the
acceptance test, the files in scope, and what to do when context is missing.

## Reasoning effort

Use low for latency-sensitive work when it preserves quality, medium as a
balanced starting point, and high or xhigh only when evals show a meaningful
gain. Effort is routed per role rather than inherited from whoever dispatches:
`[role_efforts]` in delegates.toml is the authority, and `[providers.codex]
effort` is only the default for a role that leaves it unset. The shipped table
splits five high against four medium. High is what a role earns by adjudicating
whether work ships, where a miss ships the defect. Medium is where everything
else stays: bounded work the lead re-tests anyway, and browser work bounded by
the evidence the driver captures rather than by reasoning depth. xhigh appears
nowhere in the shipped table and needs eval evidence under `evals/` before it
appears anywhere; `ultra` is the complex-profile band and never resolves through
a delegated role.

The rule is written down because practice drifts up on its own. The 2026-08-05
audit measured 6763 Codex turns at xhigh against 2298 at high, a standing
premium nobody chose per role and no eval justified. Send the resolved EFFORT
explicitly on every call, including over MCP, where there is no profile
parameter to fall back on.

## Preambles

Ask for one short visible preamble before the first tool call, then sparse
outcome-based updates at major phase changes. Do not ask the delegate to
narrate routine tool calls. A delegate that reports every read spends its output
budget on transcript, and for a review role the only output that reaches the
lead is the schema verdict anyway.

## What the request authorizes

State what level of action the dispatch authorizes, so the delegate can continue
safe, in-scope work without pausing to ask, and still stops before anything
external, destructive, costly, or scope-expanding. Take the level from the
preset the route resolved and say it once, in `<constraints>`:

- read_only: read the package, run non-mutating commands, report. No writes, no
  installs, no network calls.
- build: write inside the assigned worktree, run the acceptance test, and
  iterate on a failure without asking. No commits, no pushes, no writes outside
  the worktree, no new dependencies.
- Everything else, from a deploy to a schema migration to a paid API call, stops
  and reports back rather than asking for confirmation in-thread. The lead owns
  those.

## A brief that fits

Keep reusable prefixes stable and avoid unnecessary churn in large system
prompts: an edited prefix invalidates the cached one, and a brief that is
rebuilt every dispatch costs full price every dispatch. Vary the task block,
not the frame around it.

A brief that fits is worth more than a brief that is complete. The audit found
154 Codex sessions that compacted more than ten times, and every compaction is
the model rereading a summary of what it was told instead of the thing itself.
Pass the review package and the exact paths the delegate may read, and ask for
roughly 1000 to 2000 tokens back: a condensed result the lead can act on, not a
transcript it has to mine.

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

`delegate-run` passes `schemas/review-verdict-v1.json` through
`--output-schema`. Its stable shape is:

```json
{
  "verdict": "approve | needs_attention",
  "findings": [{
    "severity": "critical | major | minor",
    "file": "path",
    "lines": "start-end",
    "confidence": 0.0,
    "finding": "...",
    "recommendation": "..."
  }],
  "next_steps": ["..."],
  "evidence": {
    "commands": ["..."],
    "screenshots": ["..."]
  }
}
```

One strong refutation outweighs any number of clean passes; the lead
re-verifies material fixes (see mega-orchestration:cross-model-verification).
