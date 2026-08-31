# Evaluation and release evidence

Megapowers separates four evidence layers. A passing check in one layer does
not prove another layer.

| Layer | Evidence | Release role |
|---|---|---|
| Deterministic regressions | Repository mechanics without model credentials | Required PR gate |
| Trigger recall | Trace-proven skill selection | Enforced for Claude; report-only for Codex |
| Installed-plugin A/B | Treatment reliability with paired control outcomes | Optional diagnostic |
| PR replay | Hidden-oracle correctness on pinned historical work | Disabled pending broker schema `2`; report-only |

## Deterministic gate

```bash
bash evals/run-all.sh --json results.jsonl
go run evals/score.go --strict results.jsonl
```

These commands validate local schemas, runners, hooks, and tools. Strict
scoring rejects malformed, duplicated, incomparable, incomplete,
indeterminate, timed-out, or harness-error data. Passing them proves repository
mechanics only.

The root [evaluation contract](../../evals/README.md) defines result rows,
scoring, artifact handling, and change rules.

## Study protocols

Each study README is authoritative for its commands and boundaries:

- [Trigger recall](../../evals/studies/trigger-recall/README.md) defines its
  probe corpus, gates, isolation, and sanitized outputs.
- [Installed-plugin A/B](../../evals/studies/installed-ab/README.md) defines its
  paired schedule, acceptance rules, resume contract, and broker requirements.
- [PR replay](../../evals/studies/pr-replay/README.md) defines its private case
  manifest and hidden-oracle verdict. Credentialed runs remain disabled.
- [Session observability](../../evals/studies/session-observability/README.md)
  defines the maintainer-only aggregate diagnostic.

Selftests and deterministic contracts do not produce behavioral evidence.
Credentialed studies publish only their sanitized `publish/manifest.json` and
`publish/results.jsonl`. Do not share raw homes, repositories, prompts,
responses, transcripts, credentials, or absolute paths.

## Release order

1. Set the version, then freeze and commit the candidate revision.
2. Run `scripts/release.sh X.Y.Z` against the clean candidate.
3. Review and push the exact revision, then wait for remote CI.
4. Create the signed tag and publish the GitHub release.
5. Run exact-tag fresh-install smoke against the public tag.

Installed A/B and PR replay remain outside the release gate. Post-publish smoke
proves delivery from the public ref, not agent quality.
