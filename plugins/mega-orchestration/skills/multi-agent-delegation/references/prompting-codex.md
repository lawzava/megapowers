# Prompting Codex (GPT-5.x) delegates

How to word a dispatch to the Codex delegate so the result comes back usable
on the first pass. Adapted from OpenAI's own guidance for prompting Codex
(codex-plugin-cc, Apache-2.0, Copyright 2026 OpenAI), rewritten for this repo;
provenance in ATTRIBUTION.md.

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
