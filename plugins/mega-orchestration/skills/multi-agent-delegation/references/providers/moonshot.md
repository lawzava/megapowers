# Provider: Moonshot (Kimi)

The third-vendor delegate channel. It exists so independence survives one vendor
being unreachable: with only Claude and Codex, an Anthropic-authored artifact has
exactly one cross-vendor route, and a perspective-diverse panel
(mega-orchestration:cross-model-verification step 4) cannot be seated from
distinct vendors at all. Routing (which roles come here, tier, effort) lives in
delegates.toml and models.toml; this file is how to reach the backend and word the
dispatch.

## Status: declared, not reachable

The catalog ships `[providers.moonshot]` with `enabled = false`, and
`delegate-resolve` skips a disabled candidate, so nothing dispatches here yet and
no gate can fail on it. Two things must be true before the entry is flipped:

1. A working channel on the machine that routes, verified by a live
   `delegate-run --role verify` against a scratch artifact that returns a
   schema-valid receipt.
2. A `moonshot` adapter in `delegate-run`. It dispatches on `VENDOR` and knows
   only `anthropic` and `openai`; any other vendor exits 6 with
   `no safe adapter for vendor`. The adapter owes the same guarantees the other
   two give: read-only tool access, no ambient user configuration, and a
   machine-checkable JSON verdict.

Flipping `enabled` without both is worse than having one alternate vendor: the
route then fails inside a review gate instead of resolving past it.

## Models

Two tiers. Both identifiers and both context windows come from Moonshot's
published [model list](https://platform.kimi.ai/docs/models.md) (read
2026-07-30):

| Tier | Model | Context |
| --- | --- | --- |
| `frontier` | `kimi-k3` | 1M tokens |
| `strong` | `kimi-k2.7-code` | 256k tokens |

`kimi-k3` is the 2.8T-parameter open-weights model (weights published
2026-07-27), multimodal, and the only one of the two that the independence roles
resolve to. `kimi-k2.7-code` is the cheaper coding model and carries the strong
tier. Note the context asymmetry: unlike the other providers, dropping a tier
here also drops the window by a factor of four, so a large review package that
fits `kimi-k3` may not fit the strong tier.

## Channels

- OpenAI-compatible HTTP API: base URL `https://api.moonshot.ai/v1`, the model
  identifier in the `model` parameter, driven by the OpenAI SDK. This is the
  documented surface and the only one with a documented reasoning-effort
  parameter, so it is the channel an adapter should target first.
- Kimi Code CLI: the binary is `kimi`, which is what the catalog's `binary` field
  probes with `command -v`. It also exposes `kimi acp` (Agent Client Protocol
  service) and `kimi mcp` (MCP server management), either of which a harness can
  drive as a delegate surface.
- Open weights, so the same model is reachable self-hosted or through a
  third-party host when the first-party API is not an option. Independence is a
  claim about the vendor whose model produced the judgement, not about whose
  hardware ran it, so a self-hosted `kimi-k3` still satisfies a
  different-vendor route.

Not sourced, do not guess: a headless one-shot CLI invocation with a schema flag,
equivalent to `codex exec --output-schema` or `claude -p --json-schema`, is not
documented for `kimi`. Whether the OpenAI-compatible endpoint accepts
`response_format` with a full JSON schema or only a plain JSON-object mode is
also unconfirmed. Both must be established from Moonshot's own documentation
before the adapter is written, because the review verdict below has to be
machine-checkable, not merely requested in prose.

## Effort mapping

`kimi-k3` accepts `reasoning_effort` of `low`, `high`, or `max`, and `max` is its
default, per the
[K3 quickstart](https://platform.kimi.ai/docs/guide/kimi-k3-quickstart) (read
2026-07-30). Thinking mode is always on and cannot be disabled. The catalog's
global effort scale has rungs Moonshot does not offer, so
`[providers.moonshot].efforts` declares only the honest subset:

| Catalog effort | `reasoning_effort` |
| --- | --- |
| `low` | `low` |
| `high` | `high` |
| `xhigh` | `max` |
| `medium` | not offered, not declared |
| `ultra` | Codex-only band, never resolves here |

An undeclared effort is not a skip. Once `enabled` is true, a role whose effort
falls outside the list above makes `delegate-resolve` print
`provider 'moonshot' does not support role ... effort ...` and exit 2 the moment
the chain reaches Moonshot, rather than walking on to `claude`, which does accept
`medium`. So the cost is not that Moonshot gets passed over: it takes the whole
route down with it, and it does so precisely when codex is excluded or
unreachable, which is the case the fallback chain exists for. Only the
role-scoped `--vendors` probe skips on an unsupported effort, which is why the
two surfaces disagree. Nothing is exposed today because all nine role efforts are
`high`; giving a role `medium` or `ultra` later breaks that role's fallback the
moment this provider is enabled.

Every delegated role runs `high`, so the ordinary dispatch sends
`reasoning_effort=high`. Send it explicitly on every call: the API default is
`max`, which is more spend than any resolved route asked for.

## The stance

Prompt Kimi as an operator, not a collaborator, the same as the other channels:
compact, block-structured (XML tags work well), stating the task, the output
contract, the verification steps, and only the constraints that change the work.
Prefer a tighter output contract over more words. The 1M-token window invites
dumping the whole repository into the prompt; do not. Pass the review package and
the exact paths the delegate may read, so the finding cites something the lead can
check.

## Blocks that earn their place

- `<task>`: one paragraph, the goal and the definition of done.
- `<output_contract>`: the exact shape of the return.
- `<verification>`: what the delegate must run or check before answering, and
  what evidence to cite.
- `<constraints>`: files in scope, what not to touch, the sandbox preset from
  delegates.toml.
- `<context_gating>`: when required context might be missing, say what to do
  (ask, or stop and report) instead of letting it guess.

## Adversarial review template

For the `verify`, `plan_review`, and `code_review` roles. The reviewer's job is to
break confidence in the change, not to validate it: default to skepticism, give no
credit for good intent or likely follow-up work, and treat happy-path-only
behavior as a real weakness. Prefer one strong finding over several weak ones.

Attack surfaces to name in the prompt: auth and tenant isolation, data loss or
corruption, rollback and idempotency, race conditions, version skew and
migration, observability gaps.

## Review output schema

`delegate-run` passes `schemas/review-verdict-v1.json`. The schema is
vendor-neutral and identical across providers; only the flag that carries it
differs. Its stable shape is:

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

`delegate-run` re-validates the returned object itself and rejects unknown keys,
so an adapter here must return exactly these fields and nothing else. One strong
refutation outweighs any number of clean passes; the lead re-verifies material
fixes (see mega-orchestration:cross-model-verification).
