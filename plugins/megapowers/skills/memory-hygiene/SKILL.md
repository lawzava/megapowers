---
name: memory-hygiene
description: Use when auditing, pruning, correcting, refreshing, or automatically applying approved evidence-based cleanup to native harness memory.
when_to_use: Trigger phrases: audit memory, prune stale memories, fix a wrong memory, refresh memory facts, clean up what you remember.
disable-model-invocation: true
metadata:
  short-description: Evidence-based audit and cleanup of harness memory
---

# Memory Hygiene

Keep native memory factual, dated, and traceable. This workflow audits and
applies approved cleanup. It cannot prove a remembered current-state claim is
still true.

## Audit read-only

Start read-only. Identify active memory, indexes, and available source records.
Do not guess provider paths or treat an index as evidence. Quarantine every new
candidate before promotion.

Retain only `direct-statement`, `direct-observation`, `source-backed`, or
`history-entry-only` evidence. Do not retain `inferred`, `speculative`,
`unknown`, or `contested` claims in active memory. A missing transcript permits
`history-entry-only` evidence with exact dated history metadata; it does not
support reconstructed content.

For each retained or revalidation record, capture its origin, source,
`observed_at` date, and scope. Mark volatile claims and set a bounded
`max_age_days`. Revalidate expired volatile claims against an authoritative
source before retention. Memory cannot refresh itself.

Conflicts must remain preserved and surfaced. Never resolve silently or
overwrite one source with another. Do not put secrets or credentials in the
manifest or memory.

## Validate the decision

Create a temporary audit manifest using
[references/audit-manifest.md](references/audit-manifest.md). Resolve
`scripts/memory-audit.go` beside this `SKILL.md`, then run:

```bash
go run "$memory_audit_tool" --input "$audit_manifest" --as-of YYYY-MM-DD
```

The validator rejects unsupported evidence, expired facts, missing provenance,
malformed dates, duplicate IDs, unknown fields, symlinks, and common credential
patterns. It never edits provider memory.

## Apply one approved patch

Derive the exact patch from the validated manifest. Every edit must map to a
validated record ID. Include each target, effect, and recovery action. For a
destructive remove or delete, create a recoverable backup outside active memory
and include its rollback path. Report a verified no-op when no edit remains.

Show the exact patch and request one approval. Approval binds the exact target
snapshot, content, and effects. Do not write before approval. A target change
invalidates approval; rebuild the audit and request approval for the new patch.

Once the user approves, apply the exact patch automatically through the current
harness's supported memory update boundary, without another command or second
approval. Stop on the first mismatch or failure. Report applied and unapplied
changes; never widen the approved patch or guess a provider path.

Read back every target. Rebuild the manifest from that state and rerun the
validator with the current date. Do not add audit output to session startup.
