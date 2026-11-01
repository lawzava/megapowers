# Memory hygiene

Use this reference when auditing, pruning, correcting, refreshing, or applying
approved evidence-based cleanup to native harness memory. It cannot prove a
remembered current-state claim is still true.

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
overwrite one source with another. Do not put sensitive values or sign-in data in the
manifest or memory.

## Audit manifest and validator

Write one temporary UTF-8 JSON object in process-owned temporary storage unless
the user approves a durable path. `schema_version` must be `"1"`. `records` is
an array of decisions about existing or candidate memory entries:

- `id`: stable audit identifier using lowercase letters, digits, `.`, `_`, or
  `-`.
- `claim`: the exact claim under review.
- `origin`: provider memory location or candidate identifier.
- `evidence`: `direct-statement`, `direct-observation`, `source-backed`,
  `history-entry-only`, `inferred`, `speculative`, `unknown`, or `contested`.
- `decision`: `retain`, `quarantine`, `revalidate`, or `remove`.
- `source`, `observed_at`, and `scope`: required for `retain` and `revalidate`.
- `verified_at`: required for retained `source-backed` or volatile claims.
- `volatile`: true when the claim can change after observation.
- `max_age_days`: required integer from 1 through 36500 for a volatile claim.

Dates use `YYYY-MM-DD`. Retained volatile evidence expires when its
verification age exceeds `max_age_days` at the explicit `--as-of` audit date.

Example:

```json
{
  "schema_version": "1",
  "records": [
    {
      "id": "service-limit",
      "claim": "The official source showed the recorded service limit.",
      "origin": "memory/current-facts.md#service-limit",
      "evidence": "source-backed",
      "decision": "revalidate",
      "source": "https://example.invalid/official-limit",
      "observed_at": "2026-07-01",
      "verified_at": "2026-07-01",
      "scope": "service plan",
      "volatile": true,
      "max_age_days": 30
    }
  ]
}
```

Resolve `scripts/memory-audit.go` in this skill's directory, then run:

```bash
go run "$SKILL_DIR/scripts/memory-audit.go" --input "$audit_manifest" --as-of YYYY-MM-DD
```

The validator rejects unsupported evidence, expired facts, missing provenance,
malformed dates, duplicate IDs, unknown fields, symlinks, and common sensitive-value
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
