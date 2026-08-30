#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
empty_plugin_hash='sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
plugin_hash='sha256:1111111111111111111111111111111111111111111111111111111111111111'

activation_row() {
  local run_id="$1" case_id="$2" block_id="$3" verdict="$4"
  jq -cn \
    --arg run_id "$run_id" \
    --arg case_id "$case_id" \
    --arg block_id "$block_id" \
    --arg verdict "$verdict" \
    --arg plugin_hash "$plugin_hash" \
    '{
      schema_version:"1",
      study:"trigger-recall",
      evidence_class:"activation",
      case_id:$case_id,
      run_id:$run_id,
      block_id:$block_id,
      arm:"treatment",
      harness:{name:"claude-code",cli_version:"2.1.0",model:"claude-fable-5",effort:"high"},
      source:{repository:"megapowers",revision:"0123456789abcdef0123456789abcdef01234567"},
      prompt_hash:"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      fixture_hash:"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      plugin_hash:$plugin_hash,
      status:"completed",
      rc:0,
      duration_ms:1000,
      verdict:$verdict,
      metrics:{activation_success:(if $verdict == "pass" then 1 else 0 end)},
      artifacts:{response:"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",trace:"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
      environment:{os:"linux",arch:"amd64",sandbox:"bwrap",locale:"C.UTF-8"},
      timestamp:"2026-08-30T12:00:00Z"
    }'
}

expect_reject() {
  local name="$1" file="$2"
  if go run "$ROOT/evals/score.go" --strict "$file" >"$tmp/$name.out" 2>"$tmp/$name.err"; then
    echo "FAIL $name: scorer accepted invalid activation input" >&2
    exit 1
  fi
}

# A valid activation file scores, including failed probes as data.
{
  activation_row a-1 debug-paraphrase rep-1 pass
  activation_row a-2 debug-paraphrase rep-2 fail
  activation_row a-3 prose-adjacent rep-1 pass
  activation_row a-4 prose-adjacent rep-2 pass
} >"$tmp/valid.jsonl"
go run "$ROOT/evals/score.go" --strict "$tmp/valid.jsonl" >"$tmp/valid.out"
grep -q 'Activation evidence' "$tmp/valid.out"
grep -q 'debug-paraphrase' "$tmp/valid.out"
grep -q '| 1/2 |' "$tmp/valid.out"
grep -q '| 2/2 |' "$tmp/valid.out"

# Activation rows never enter behavioral pairing or mcnemar output.
if grep -q 'mcnemar_p' "$tmp/valid.out"; then
  echo 'FAIL activation rows leaked into behavioral comparison output' >&2
  exit 1
fi

# Only the treatment arm exists: activation runs always have the plugin installed.
activation_row b-1 debug-paraphrase rep-1 pass |
  jq -c '.arm = "control"' >"$tmp/control-arm.jsonl"
expect_reject control-arm "$tmp/control-arm.jsonl"
grep -q 'activation rows must use arm "treatment"' "$tmp/control-arm.err"

# The plugin hash must not identify the empty plugin set.
activation_row c-1 debug-paraphrase rep-1 pass |
  jq -c --arg hash "$empty_plugin_hash" '.plugin_hash = $hash' >"$tmp/empty-plugin.jsonl"
expect_reject empty-plugin "$tmp/empty-plugin.jsonl"
grep -q 'activation plugin hash must not identify the empty plugin set' "$tmp/empty-plugin.err"

# activation_success is required, binary, and must match the verdict.
activation_row d-1 debug-paraphrase rep-1 pass |
  jq -c 'del(.metrics.activation_success) | .metrics.other = 1' >"$tmp/missing-metric.jsonl"
expect_reject missing-metric "$tmp/missing-metric.jsonl"
grep -q 'activation_success' "$tmp/missing-metric.err"

activation_row e-1 debug-paraphrase rep-1 fail |
  jq -c '.metrics.activation_success = 1' >"$tmp/metric-verdict-mismatch.jsonl"
expect_reject metric-verdict-mismatch "$tmp/metric-verdict-mismatch.jsonl"
grep -q 'activation_success must match the verdict' "$tmp/metric-verdict-mismatch.err"

# A sentinel revision is not provenance.
activation_row f-1 debug-paraphrase rep-1 pass |
  jq -c '.source.revision = "main"' >"$tmp/sentinel-revision.jsonl"
expect_reject sentinel-revision "$tmp/sentinel-revision.jsonl"
grep -q 'must be a full git commit hash' "$tmp/sentinel-revision.err"

# Two rows in one case must not share a rep block.
{
  activation_row g-1 debug-paraphrase rep-1 pass
  activation_row g-2 debug-paraphrase rep-1 pass
  activation_row g-3 prose-adjacent rep-1 pass
  activation_row g-4 prose-adjacent rep-2 pass
} >"$tmp/repeated-block.jsonl"
expect_reject repeated-block "$tmp/repeated-block.jsonl"
grep -q 'repeats rep block' "$tmp/repeated-block.err"

# Unbalanced rep counts across cases hide a partial run.
{
  activation_row h-1 debug-paraphrase rep-1 pass
  activation_row h-2 debug-paraphrase rep-2 pass
  activation_row h-3 prose-adjacent rep-1 pass
} >"$tmp/unbalanced.jsonl"
expect_reject unbalanced "$tmp/unbalanced.jsonl"
grep -q 'unbalanced activation rep counts' "$tmp/unbalanced.err"

# One case must not mix plugin revisions.
{
  activation_row i-1 debug-paraphrase rep-1 pass
  activation_row i-2 debug-paraphrase rep-2 pass |
    jq -c '.plugin_hash = "sha256:2222222222222222222222222222222222222222222222222222222222222222"'
} >"$tmp/mixed-plugin.jsonl"
expect_reject mixed-plugin "$tmp/mixed-plugin.jsonl"
grep -q 'mixes plugin hashes' "$tmp/mixed-plugin.err"

# Infrastructure failures stay unscoreable in the activation class too.
activation_row j-1 debug-paraphrase rep-1 pass |
  jq -c '.status = "timeout" | .rc = 124' >"$tmp/timeout.jsonl"
expect_reject timeout "$tmp/timeout.jsonl"

echo 'strict activation scoring contract: ok'
