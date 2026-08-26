# Memory audit manifest

Use one UTF-8 JSON object. `schema_version` must be `"1"`. `records` is an
array of decisions about existing or candidate memory entries.

Each record has these fields:

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

Dates use `YYYY-MM-DD`. The explicit `--as-of` date is the audit boundary.
Retained volatile evidence expires when its verification age exceeds
`max_age_days`.

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

The manifest is an audit artifact, not provider memory. Keep it in process-owned
temporary storage unless the user approves a durable path.
