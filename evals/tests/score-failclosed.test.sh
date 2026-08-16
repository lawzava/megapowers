#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
empty_plugin_hash='sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
treatment_plugin_hash='sha256:1111111111111111111111111111111111111111111111111111111111111111'
other_plugin_hash='sha256:2222222222222222222222222222222222222222222222222222222222222222'

valid_row() {
  local run_id="$1" block_id="$2" arm="$3"
  jq -cn \
    --arg run_id "$run_id" \
    --arg block_id "$block_id" \
    --arg arm "$arm" \
    --arg plugin_hash "sha256:$(printf '%064d' "$([ "$arm" = treatment ] && echo 1 || echo 2)")" \
    '{
      schema_version:"1",
      study:"humanizing-prose-ab",
      evidence_class:"behavioral",
      case_id:"case_1",
      run_id:$run_id,
      block_id:$block_id,
      arm:$arm,
      harness:{name:"claude-code",cli_version:"2.1.0",model:"claude-fable-5",effort:"high"},
      source:{repository:"megapowers",revision:"0123456789abcdef0123456789abcdef01234567"},
      prompt_hash:"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      fixture_hash:"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      plugin_hash:$plugin_hash,
      status:"completed",
      rc:0,
      duration_ms:1234,
      verdict:"pass",
      metrics:{task_success:1,fact_retention:1},
      artifacts:{response:"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
      environment:{os:"linux",arch:"amd64",sandbox:"workspace-write",locale:"C.UTF-8"},
      timestamp:"2026-08-16T12:00:00Z"
    }'
}

installed_row() {
  local run_id="$1" block_id="$2" arm="$3" plugin_hash="$4"
  valid_row "$run_id" "$block_id" "$arm" |
    jq -c --arg plugin_hash "$plugin_hash" '.study = "installed-plugin-ab" | .plugin_hash = $plugin_hash'
}

write_publish_rows() {
  local file="$1" treatment_passes="$2" control_passes="$3" pairs="$4"
  : > "$file"
  local pair arm verdict rc
  for pair in $(seq 1 "$pairs"); do
    for arm in treatment control; do
      verdict=fail
      rc=0
      if [[ $arm == treatment && $pair -le $treatment_passes ]] ||
         [[ $arm == control && $pair -le $control_passes ]]; then
        verdict=pass
        rc=0
      fi
      installed_row "$arm-$pair" "block-$pair" "$arm" \
        "$([[ $arm == treatment ]] && printf '%s' "$treatment_plugin_hash" || printf '%s' "$empty_plugin_hash")" |
        jq -c --arg verdict "$verdict" --argjson rc "$rc" \
          '.environment.sandbox = "bwrap" | .verdict = $verdict | .rc = $rc | .metrics.task_success = (if $verdict == "pass" then 1 else 0 end)' \
          >> "$file"
    done
  done
}

expect_reject() {
  local name="$1" file="$2"
  if go run "$ROOT/evals/score.go" --strict "$file" >"$tmp/$name.out" 2>"$tmp/$name.err"; then
    echo "FAIL $name: scorer accepted invalid input" >&2
    exit 1
  fi
}

printf '{not json}\n' >"$tmp/malformed.jsonl"
expect_reject malformed "$tmp/malformed.jsonl"

: >"$tmp/empty.jsonl"
expect_reject empty "$tmp/empty.jsonl"

valid_row treatment-1 block-1 treatment >"$tmp/incomplete.jsonl"
expect_reject incomplete-arms "$tmp/incomplete.jsonl"

{
  valid_row duplicate block-1 treatment
  valid_row duplicate block-1 control
} >"$tmp/duplicate.jsonl"
expect_reject duplicate-runs "$tmp/duplicate.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c '.source.revision = "fedcba9876543210fedcba9876543210fedcba98"'
} >"$tmp/mixed-provenance.jsonl"
expect_reject mixed-provenance "$tmp/mixed-provenance.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c 'del(.metrics.fact_retention)'
} >"$tmp/mixed-metrics.jsonl"
expect_reject mixed-metrics "$tmp/mixed-metrics.jsonl"

{
  installed_row treatment-1 block-1 treatment "$treatment_plugin_hash"
  installed_row control-1 block-1 control "$treatment_plugin_hash"
} >"$tmp/identical-plugin-hashes.jsonl"
expect_reject identical-plugin-hashes "$tmp/identical-plugin-hashes.jsonl"
grep -q 'treatment and control plugin hashes must differ' "$tmp/identical-plugin-hashes.err"

{
  installed_row treatment-1 block-1 treatment "$treatment_plugin_hash"
  installed_row control-1 block-1 control "$other_plugin_hash"
} >"$tmp/nonempty-control-plugin.jsonl"
expect_reject nonempty-control-plugin "$tmp/nonempty-control-plugin.jsonl"
grep -q 'control plugin hash must identify the empty plugin set' "$tmp/nonempty-control-plugin.err"

{
  installed_row treatment-1 block-1 treatment "$empty_plugin_hash"
  installed_row control-1 block-1 control "$empty_plugin_hash"
} >"$tmp/empty-treatment-plugin.jsonl"
expect_reject empty-treatment-plugin "$tmp/empty-treatment-plugin.jsonl"
grep -q 'treatment plugin hash must not identify the empty plugin set' "$tmp/empty-treatment-plugin.err"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c '.verdict = "indeterminate"'
} >"$tmp/indeterminate.jsonl"
expect_reject indeterminate "$tmp/indeterminate.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c '.status = "timeout" | .verdict = "harness_error" | .rc = 124'
} >"$tmp/timeout.jsonl"
expect_reject timeout "$tmp/timeout.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c '.status = "harness_error" | .verdict = "harness_error" | .rc = 125'
} >"$tmp/harness-error.jsonl"
expect_reject harness-error "$tmp/harness-error.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c '.surprise = true'
} >"$tmp/unknown-field.jsonl"
expect_reject unknown-field "$tmp/unknown-field.jsonl"

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control | jq -c 'del(.rc)'
} >"$tmp/missing-required.jsonl"
expect_reject missing-required "$tmp/missing-required.jsonl"
grep -q 'required field "rc" is missing' "$tmp/missing-required.err" || {
  cat "$tmp/missing-required.err" >&2
  exit 1
}

{
  valid_row treatment-1 block-1 treatment
  valid_row control-1 block-1 control
} >"$tmp/valid.jsonl"
go run "$ROOT/evals/score.go" --strict "$tmp/valid.jsonl" >"$tmp/valid.out"
grep -q 'humanizing-prose-ab' "$tmp/valid.out"
grep -q 'treatment' "$tmp/valid.out"
grep -q 'control' "$tmp/valid.out"
grep -q 'Named metric means' "$tmp/valid.out"
grep -q 'fact_retention' "$tmp/valid.out"
grep -q 'mcnemar_p' "$tmp/valid.out"

{
  installed_row treatment-1 block-1 treatment "$treatment_plugin_hash"
  installed_row control-1 block-1 control "$empty_plugin_hash"
} >"$tmp/valid-installed-ab.jsonl"
go run "$ROOT/evals/score.go" --strict "$tmp/valid-installed-ab.jsonl" >"$tmp/valid-installed-ab.out"
grep -q 'installed-plugin-ab' "$tmp/valid-installed-ab.out"

{
  installed_row treatment-1 block-1 treatment "$treatment_plugin_hash"
  installed_row control-1 block-1 control "$empty_plugin_hash" | jq -c '.verdict = "fail" | .rc = 1 | .metrics.task_success = 0'
} >"$tmp/nonzero-installed-actor.jsonl"
expect_reject nonzero-installed-actor "$tmp/nonzero-installed-actor.jsonl"
grep -q 'installed-plugin completed rows must have actor rc 0' "$tmp/nonzero-installed-actor.err"

{
  installed_row treatment-1 block-1 treatment "$treatment_plugin_hash"
  installed_row control-1 block-1 control "$empty_plugin_hash"
} >"$tmp/non-broker-sandbox.jsonl"
if go run "$ROOT/evals/score.go" --strict \
  --publishable-gates "$ROOT/evals/studies/installed-ab/gates.json" \
  "$tmp/non-broker-sandbox.jsonl" >"$tmp/non-broker-sandbox.out" 2>"$tmp/non-broker-sandbox.err"; then
  echo 'FAIL publishability accepted non-broker sandbox provenance' >&2
  exit 1
fi
grep -q 'broker-attested OS boundary' "$tmp/non-broker-sandbox.err"

write_publish_rows "$tmp/perfect-treatment.jsonl" 10 10 10
go run "$ROOT/evals/score.go" --strict \
  --publishable-gates "$ROOT/evals/studies/installed-ab/gates.json" \
  "$tmp/perfect-treatment.jsonl" >/dev/null

write_publish_rows "$tmp/control-diagnostic.jsonl" 10 0 10
go run "$ROOT/evals/score.go" --strict \
  --publishable-gates "$ROOT/evals/studies/installed-ab/gates.json" \
  "$tmp/control-diagnostic.jsonl" >/dev/null

write_publish_rows "$tmp/paired-mcnemar.jsonl" 12 0 12
go run "$ROOT/evals/score.go" --strict "$tmp/paired-mcnemar.jsonl" >"$tmp/paired-mcnemar.out"
grep -q '| 0.000488 |' "$tmp/paired-mcnemar.out"

write_publish_rows "$tmp/imperfect-treatment.jsonl" 9 10 10
if go run "$ROOT/evals/score.go" --strict \
  --publishable-gates "$ROOT/evals/studies/installed-ab/gates.json" \
  "$tmp/imperfect-treatment.jsonl" >"$tmp/imperfect-publish.out" 2>"$tmp/imperfect-publish.err"; then
  echo 'FAIL publishability accepted an imperfect treatment' >&2
  exit 1
fi
grep -q 'require every treatment run to pass' "$tmp/imperfect-publish.err"

write_publish_rows "$tmp/too-few-pairs.jsonl" 9 9 9
if go run "$ROOT/evals/score.go" --strict \
  --publishable-gates "$ROOT/evals/studies/installed-ab/gates.json" \
  "$tmp/too-few-pairs.jsonl" >"$tmp/too-few-pairs.out" 2>"$tmp/too-few-pairs.err"; then
  echo 'FAIL publishability accepted too few paired runs' >&2
  exit 1
fi
grep -q 'balanced pairs; require 10' "$tmp/too-few-pairs.err"

echo 'strict score fail-closed contract: ok'
