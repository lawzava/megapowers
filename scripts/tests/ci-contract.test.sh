#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ci="$ROOT/.github/workflows/ci.yml"
freshness="$ROOT/.github/workflows/freshness.yml"

fail() {
  printf 'ci contract: %s\n' "$*" >&2
  exit 1
}

for pin in \
  actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 \
  actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 \
  actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 \
  actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f; do
  grep -qF "$pin" "$ci" || fail "missing reviewed action pin: $pin"
done

grep -qF 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1' "$freshness" ||
  fail 'freshness workflow checkout is not pinned'
grep -qF 'persist-credentials: false' "$freshness" ||
  fail 'freshness workflow persists checkout credentials'
if rg -n 'uses:[[:space:]]+[^#[:space:]]+@v[0-9]+' "$freshness"; then
  fail 'freshness workflow retains a mutable major action reference'
fi

if rg -n 'uses:[[:space:]]+[^#[:space:]]+@v[0-9]+' "$ci"; then
  fail 'mutable major action reference remains'
fi

checkout_count="$(grep -c 'uses: actions/checkout@' "$ci")"
credential_count="$(grep -c 'persist-credentials: false' "$ci")"
[[ $checkout_count -eq $credential_count ]] ||
  fail 'every checkout must disable persisted credentials'

grep -qF '@anthropic-ai/claude-code@2.1.257' "$ci" ||
  fail 'Claude Code CLI is not version-pinned'
grep -qF 'apt-get install -y jq ripgrep shellcheck' "$ci" ||
  fail 'CI does not install the declared ripgrep test dependency'
grep -qF 'go run evals/score.go --strict results.jsonl' "$ci" ||
  fail 'CI does not strict-score deterministic results'
grep -qF 'if: always()' "$ci" || fail 'CI does not preserve result artifacts after failure'
grep -qF 'path: results.jsonl' "$ci" || fail 'CI does not upload the sanitized JSONL'

printf 'ci contract: ok\n'
