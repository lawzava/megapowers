#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

out="$tmp/selftest.out"
TMPDIR="$tmp" go run "$ROOT/evals/studies/pr-replay/replay.go" --selftest >"$out"

for claim in \
  "mutable refs rejected" \
  "gold change unavailable to actor" \
  "missing correctness oracle rejected" \
  "already-green correctness oracle rejected" \
  "file overlap remains diagnostic" \
  "live runs require isolated broker" \
  "isolation attestation rejects gold and credential access" \
  "schema rows pass strict scorer" \
  "publish bundle contains sanitized files only"
do
  grep -qF "ok   $claim" "$out"
done
grep -qF "pr-replay selftest: PASS" "$out"

if find "$tmp" -mindepth 1 -maxdepth 1 -type d -name 'megapowers-pr-replay-*' | grep -q .; then
  echo "FAIL PR replay left temporary state behind" >&2
  exit 1
fi

go run "$ROOT/evals/studies/pr-replay/replay.go" --validate-config \
  --cases "$ROOT/evals/studies/pr-replay/cases.json" >/dev/null

if go run "$ROOT/evals/studies/pr-replay/replay.go" --run \
  --cases "$ROOT/evals/studies/pr-replay/cases.json" \
  --harness codex --model fake --out "$tmp/not-credentialed" >/dev/null 2>&1; then
  echo "FAIL live PR replay did not require explicit credentialed mode" >&2
  exit 1
fi

if go run "$ROOT/evals/studies/pr-replay/replay.go" --run --credentialed \
  --cases "$ROOT/evals/studies/pr-replay/cases.json" \
  --harness codex --model fake --out "$tmp/no-broker" >/dev/null 2>&1; then
  echo "FAIL live PR replay accepted no isolation broker" >&2
  exit 1
fi

# A fully specified credentialed invocation must still be refused before any
# actor or oracle process runs while the broker schema-2 upgrade is pending.
stub="$tmp/reviewed-broker"
printf '#!/bin/sh\n' >"$stub"
chmod +x "$stub"
stub_hash="$(sha256sum "$stub" | cut -d' ' -f1)"
if go run "$ROOT/evals/studies/pr-replay/replay.go" --run --credentialed \
  --cases "$ROOT/evals/studies/pr-replay/cases.json" \
  --harness codex --model fake --out "$tmp/cred-refused" \
  --sandbox-broker "$stub" --broker-sha256 "sha256:$stub_hash" \
  >"$tmp/cred.out" 2>"$tmp/cred.err"; then
  echo "FAIL credentialed PR replay ran while the broker schema-2 upgrade is pending" >&2
  exit 1
fi
if ! grep -qF "sandbox broker schema-2 upgrade is required before credentialed runs" "$tmp/cred.err"; then
  echo "FAIL credentialed PR replay refusal does not name the broker schema-2 upgrade" >&2
  cat "$tmp/cred.err" >&2
  exit 1
fi
if [ -e "$tmp/cred-refused" ]; then
  echo "FAIL refused credentialed PR replay produced run artifacts" >&2
  exit 1
fi
if grep -qE '\.credentials\.json|auth\.json|copyCredential' "$ROOT/evals/studies/pr-replay/replay.go"; then
  echo "FAIL PR replay runner contains credential-copy logic" >&2
  exit 1
fi

echo "PR replay contract: PASS"
