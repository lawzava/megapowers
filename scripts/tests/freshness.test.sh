#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script="$ROOT/scripts/check-freshness.sh"
impl="$ROOT/scripts/check-freshness.go"
workflow="$ROOT/.github/workflows/freshness.yml"

grep -q 'defaultMaxAge = 30' "$impl"
grep -q 'docs/harness-support.md' "$impl"
grep -q 'Harness tooling reviewed: 2026-08-16' "$script"
if rg -n 'models[.]toml|delegates[.]toml|OpenCode|mega-orchestration|mega-frontend' "$impl" "$script" "$workflow"; then
  printf 'freshness contract: removed surface remains\n' >&2
  exit 1
fi
grep -q "cron: '17 6 \* \* 1'" "$workflow"
grep -qF 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1' "$workflow"
grep -qF 'persist-credentials: false' "$workflow"

"$script" --max-age-days 36500 >/dev/null
TZ=UTC "$script" --max-age-days 36500 >/dev/null

tmp="$(mktemp -d "${TMPDIR:-/tmp}/freshness-test.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/docs" "$tmp/scripts"
printf '%s\n' 'Last reviewed: 2999-01-01' > "$tmp/docs/harness-support.md"
printf '%s\n' '# Harness tooling reviewed: 2999-01-01' > "$tmp/scripts/check-freshness.sh"

set +e
negative_out="$(MEGAPOWERS_ROOT="$ROOT" go run "$impl" --max-age-days -1 2>&1)"
negative_rc=$?
future_out="$(MEGAPOWERS_ROOT="$tmp" go run "$impl" --max-age-days 36500 2>&1)"
future_rc=$?
set -e
if [ "$negative_rc" -eq 0 ] || ! grep -q 'non-negative' <<< "$negative_out"; then
  printf 'freshness contract: negative max age did not fail closed\n' >&2
  exit 1
fi
if [ "$future_rc" -eq 0 ] || ! grep -q 'future' <<< "$future_out"; then
  printf 'freshness contract: future review date did not fail closed\n' >&2
  exit 1
fi

printf 'freshness contract: ok\n'
