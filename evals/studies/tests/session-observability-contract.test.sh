#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

out="$tmp/selftest.out"
TMPDIR="$tmp" go run "$ROOT/evals/studies/session-observability/run.go" --selftest >"$out"

for claim in \
  "explicit file inputs required" \
  "duplicate root groups rejected" \
  "duplicate root bytes rejected" \
  "one-root patterns suppressed" \
  "two-root patterns promoted" \
  "canaries and sensitive fields excluded" \
  "typed sidecars fail closed"
do
  grep -qF "ok   $claim" "$out"
done
grep -qF "session-observability selftest: PASS" "$out"

if find "$tmp" -mindepth 1 -maxdepth 1 -type d -name 'megapowers-session-observability-*' | grep -q .; then
  echo "FAIL session observability left temporary state behind" >&2
  exit 1
fi

if go run "$ROOT/evals/studies/session-observability/run.go" \
  --root-group "$tmp/missing-one" --root-group "$tmp/missing-two" >/dev/null 2>&1; then
  echo "FAIL session observability accepted missing typed sidecars" >&2
  exit 1
fi

for marker in \
  'at least two independent --root-group inputs are required' \
  'decoder.DisallowUnknownFields()' \
  'PRIVATE-CANARY-session-body-path-id-time'
do
  grep -qF "$marker" "$ROOT/evals/studies/session-observability/run.go"
done

echo "session observability contract: PASS"
