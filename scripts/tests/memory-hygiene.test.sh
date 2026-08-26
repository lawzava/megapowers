#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TOOL="$ROOT/plugins/megapowers/skills/memory-hygiene/scripts/memory-audit.go"
GO_BIN="$(command -v go)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/memory-hygiene-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
last_out=""
last_rc=0

ok() {
  pass=$((pass + 1))
  printf '  ok %s\n' "$1"
}

bad() {
  fail=$((fail + 1))
  printf '  FAIL %s\n' "$1"
  [ -z "$last_out" ] || printf '%s\n' "$last_out" | sed -n '1,20p'
}

run_tool() {
  last_out="$("$GO_BIN" run "$TOOL" "$@" 2>&1)"
  last_rc=$?
}

must_succeed() {
  local name="$1"
  shift
  run_tool "$@"
  if [ "$last_rc" -eq 0 ]; then ok "$name"; else bad "$name (exit $last_rc)"; fi
}

must_fail_with() {
  local name="$1" needle="$2"
  shift 2
  run_tool "$@"
  if [ "$last_rc" -ne 0 ] && grep -qi -- "$needle" <<< "$last_out"; then
    ok "$name"
  else
    bad "$name (exit $last_rc, missing '$needle')"
  fi
}

write_manifest() {
  local path="$1" records="$2"
  printf '{"schema_version":"1","records":%s}\n' "$records" > "$path"
}

printf '== memory hygiene black-box tests ==\n'

valid="$TMP/valid.json"
write_manifest "$valid" '[
  {
    "id":"direct-rule",
    "claim":"The user directly restricted release authority.",
    "origin":"memory/user-rules.md#release-authority",
    "evidence":"direct-statement",
    "decision":"retain",
    "source":"session:abc:turn:7",
    "observed_at":"2026-08-20",
    "scope":"repository writes"
  },
  {
    "id":"current-limit",
    "claim":"The documented service limit was observed at the cited source.",
    "origin":"memory/current-facts.md#service-limit",
    "evidence":"source-backed",
    "decision":"retain",
    "source":"https://example.invalid/official-limit",
    "observed_at":"2026-08-24",
    "verified_at":"2026-08-24",
    "scope":"service plan",
    "volatile":true,
    "max_age_days":7
  },
  {
    "id":"soft-profile",
    "claim":"A profile inference lacks direct support.",
    "origin":"memory/profile.md#style",
    "evidence":"inferred",
    "decision":"remove"
  },
  {
    "id":"missing-transcript",
    "claim":"Only the dated history entry remains available.",
    "origin":"memory/history.md#entry-3",
    "evidence":"history-entry-only",
    "decision":"quarantine"
  }
]'

before_hash="$(sha256sum "$valid" | cut -d' ' -f1)"
must_succeed 'valid mixed audit passes' --input "$valid" --as-of 2026-08-26
after_hash="$(sha256sum "$valid" | cut -d' ' -f1)"
if [ "$before_hash" = "$after_hash" ]; then ok 'audit leaves its input unchanged'; else bad 'audit leaves its input unchanged'; fi
if grep -q '4 records: 2 retain, 1 quarantine, 0 revalidate, 1 remove' <<< "$last_out"; then
  ok 'success summary reports every decision'
else
  bad 'success summary reports every decision'
fi

inferred="$TMP/inferred.json"
write_manifest "$inferred" '[{
  "id":"soft-profile",
  "claim":"The user probably prefers terse output.",
  "origin":"memory/profile.md#style",
  "evidence":"inferred",
  "decision":"retain",
  "source":"session:abc",
  "observed_at":"2026-08-20",
  "scope":"global"
}]'
must_fail_with 'inference cannot remain active' 'cannot retain inferred evidence' \
  --input "$inferred" --as-of 2026-08-26

stale="$TMP/stale.json"
write_manifest "$stale" '[{
  "id":"old-limit",
  "claim":"The documented service limit was observed at the cited source.",
  "origin":"memory/current-facts.md#old-limit",
  "evidence":"source-backed",
  "decision":"retain",
  "source":"https://example.invalid/official-limit",
  "observed_at":"2026-07-01",
  "verified_at":"2026-07-01",
  "scope":"service plan",
  "volatile":true,
  "max_age_days":7
}]'
must_fail_with 'expired volatile fact cannot remain active' 'expired 49 days after verification' \
  --input "$stale" --as-of 2026-08-19

revalidate="$TMP/revalidate.json"
write_manifest "$revalidate" '[{
  "id":"old-limit",
  "claim":"The documented service limit needs a current source check.",
  "origin":"memory/current-facts.md#old-limit",
  "evidence":"source-backed",
  "decision":"revalidate",
  "source":"https://example.invalid/official-limit",
  "observed_at":"2026-07-01",
  "verified_at":"2026-07-01",
  "scope":"service plan",
  "volatile":true,
  "max_age_days":7
}]'
must_succeed 'expired fact passes when marked for revalidation' \
  --input "$revalidate" --as-of 2026-08-19

missing="$TMP/missing.json"
write_manifest "$missing" '[{
  "id":"missing-source",
  "claim":"A retained fact lacks provenance.",
  "origin":"memory/facts.md#missing",
  "evidence":"direct-observation",
  "decision":"retain",
  "observed_at":"2026-08-20",
  "scope":"one environment"
}]'
must_fail_with 'retained fact requires a source' 'retain or revalidate record requires source' \
  --input "$missing" --as-of 2026-08-26

future="$TMP/future.json"
write_manifest "$future" '[{
  "id":"future-observation",
  "claim":"An observation cannot postdate the audit.",
  "origin":"memory/facts.md#future",
  "evidence":"direct-observation",
  "decision":"retain",
  "source":"command:status",
  "observed_at":"2026-08-27",
  "scope":"local checkout"
}]'
must_fail_with 'future evidence date fails closed' 'observed_at is after as-of date' \
  --input "$future" --as-of 2026-08-26

duplicate="$TMP/duplicate.json"
write_manifest "$duplicate" '[
  {"id":"same","claim":"One.","origin":"a","evidence":"unknown","decision":"remove"},
  {"id":"same","claim":"Two.","origin":"b","evidence":"unknown","decision":"remove"}
]'
must_fail_with 'duplicate identifiers fail closed' 'duplicate id' \
  --input "$duplicate" --as-of 2026-08-26

unknown_field="$TMP/unknown-field.json"
printf '%s\n' '{"schema_version":"1","records":[],"write_provider_memory":true}' > "$unknown_field"
must_fail_with 'unknown manifest fields fail closed' 'unknown field' \
  --input "$unknown_field" --as-of 2026-08-26

missing_records="$TMP/missing-records.json"
printf '%s\n' '{"schema_version":"1"}' > "$missing_records"
must_fail_with 'records array is required' 'records array is required' \
  --input "$missing_records" --as-of 2026-08-26

unbounded="$TMP/unbounded.json"
write_manifest "$unbounded" '[{
  "id":"unbounded",
  "claim":"The refresh window must remain bounded.",
  "origin":"memory/current-facts.md#unbounded",
  "evidence":"source-backed",
  "decision":"revalidate",
  "source":"https://example.invalid/official",
  "observed_at":"2026-08-20",
  "scope":"service plan",
  "volatile":true,
  "max_age_days":36501
}]'
must_fail_with 'volatile refresh window is bounded' 'max_age_days cannot exceed 36500' \
  --input "$unbounded" --as-of 2026-08-26

secret="$TMP/secret.json"
secret_value='abcdefghijklmnop'
write_manifest "$secret" "[{
  \"id\":\"secret-value\",
  \"claim\":\"api_key=$secret_value\",
  \"origin\":\"memory/facts.md#secret\",
  \"evidence\":\"unknown\",
  \"decision\":\"remove\"
}]"
must_fail_with 'secret-like content is rejected without echoing it' 'secret-like content' \
  --input "$secret" --as-of 2026-08-26
if grep -q "$secret_value" <<< "$last_out"; then bad 'secret value is absent from errors'; else ok 'secret value is absent from errors'; fi

ln -s "$valid" "$TMP/link.json"
must_fail_with 'symlink input fails closed' 'symlink' \
  --input "$TMP/link.json" --as-of 2026-08-26

must_fail_with 'explicit as-of date is required' 'as-of is required' --input "$valid"
must_fail_with 'input path is required' 'input is required' --as-of 2026-08-26

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
