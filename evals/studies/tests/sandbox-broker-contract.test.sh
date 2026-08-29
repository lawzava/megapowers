#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SOURCE="$ROOT/evals/tools/sandbox-broker/main.go"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

test -f "$SOURCE" || {
  echo "FAIL sandbox broker source is missing" >&2
  exit 1
}

go build -o "$tmp/megapowers-eval-broker" "$SOURCE"
"$tmp/megapowers-eval-broker" --selftest >"$tmp/selftest.out"

for claim in \
  "strict JSON rejects unknown fields and trailing data" \
  "request validation requires exact canonical roots" \
  "request validation rejects symlinked and overlapping roots" \
  "credential hardlinks stay outside actor-visible roots" \
  "subscription auth wins before explicit API-key fallback" \
  "Claude subscription credentials stay outside actor tools" \
  "Codex external auth stays memory-only" \
  "app-server rejects unsafe client authority" \
  "Codex subscription egress reaches only approved provider hosts" \
  "child environment excludes inherited credentials" \
  "credential proxy injects only the provider key upstream" \
  "end-to-end fake actor satisfies schema version 2" \
  "incomplete actor trace forces infrastructure failure" \
  "sandbox hides credential and sibling sentinels" \
  "sandbox keeps project writable and plugin read-only" \
  "actor network reaches only the local credential bridge" \
  "protected effect monitor catches restored mutations" \
  "timeout terminates the isolated process tree" \
  "trace normalization requires a complete result" \
  "Codex skills.read activation is normalized" \
  "oracle phase excludes credentials and network" \
  "response redaction removes credential values" \
  "arm inventory is exact"
do
  grep -qF "ok   $claim" "$tmp/selftest.out"
done
grep -qF "sandbox broker selftest: PASS" "$tmp/selftest.out"

long_tmpdir="$tmp/long"
while (( ${#long_tmpdir} < 48 )); do
  long_tmpdir="${long_tmpdir}0123456789"
done
long_tmpdir="${long_tmpdir:0:48}"
mkdir -p "$long_tmpdir"
TMPDIR="$long_tmpdir" "$tmp/megapowers-eval-broker" --selftest \
  >"$tmp/selftest-long-tmpdir.out" 2>"$tmp/selftest-long-tmpdir.err"
grep -qF "sandbox broker selftest: PASS" "$tmp/selftest-long-tmpdir.out"

guard_tmpdir="$long_tmpdir$long_tmpdir$long_tmpdir"
mkdir -p "$guard_tmpdir"
if TMPDIR="$guard_tmpdir" "$tmp/megapowers-eval-broker" --selftest \
  >"$tmp/selftest-guard-tmpdir.out" 2>"$tmp/selftest-guard-tmpdir.err"; then
  echo "FAIL sandbox broker selftest did not reject a socket path beyond the Unix-socket safety bound" >&2
  exit 1
fi
grep -qF "exceeds the 100-byte Linux Unix-socket safety bound" "$tmp/selftest-guard-tmpdir.err"

if printf '%s\n' '{"schema_version":"2","unexpected":true}' | \
  "$tmp/megapowers-eval-broker" >"$tmp/reject.out" 2>"$tmp/reject.err"; then
  echo "FAIL sandbox broker accepted an unknown request field" >&2
  exit 1
fi
grep -qF "decode request" "$tmp/reject.err"
test ! -s "$tmp/reject.out"

if grep -qE -- '--dangerously-skip-permissions|--dangerously-bypass-approvals-and-sandbox' "$SOURCE"; then
  echo "FAIL sandbox broker bypasses harness permissions" >&2
  exit 1
fi

if grep -qF -- '--ignore-user-config' "$SOURCE"; then
  echo "FAIL sandbox broker suppresses the disposable Codex plugin registration" >&2
  exit 1
fi

if grep -qF -- '"--bare"' "$SOURCE"; then
  echo "FAIL sandbox broker disables Claude subscription authentication with --bare" >&2
  exit 1
fi

grep -qF '"defaultMode": "acceptEdits"' "$SOURCE" || {
  echo "FAIL sandbox broker does not allow isolated fixture edits" >&2
  exit 1
}
grep -qF '"Agent", "Task", "Skill"' "$SOURCE" || {
  echo "FAIL sandbox broker does not explicitly allow native delegation and skill tools" >&2
  exit 1
}
if grep -qF '"defaultMode": "dontAsk"' "$SOURCE"; then
  echo "FAIL sandbox broker silently denies fixture actions" >&2
  exit 1
fi

echo "sandbox broker contract: PASS"
