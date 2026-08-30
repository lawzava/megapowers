#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

out="$tmp/selftest.out"
TMPDIR="$tmp" go run "$ROOT/evals/studies/trigger-recall/run.go" --selftest >"$out"

for claim in \
  "recall accepts a successful expected selection" \
  "recall rejects a missing expected selection" \
  "recall rejects a failed expected selection attempt" \
  "precision rejects an unexpected skill selection" \
  "precision counts failed unexpected attempts" \
  "precision ignores allowed co-selection" \
  "no-skill probes reject any megapowers selection" \
  "non-catalog selections stay out of precision" \
  "rows carry binary activation_success" \
  "rows pass the strict scorer" \
  "reps emit unique blocks" \
  "temporary state removed after success" \
  "actor errors fail closed" \
  "transient actor failures retry once" \
  "repeated actor failures still fail closed" \
  "actor deadlines fail closed" \
  "live runs require isolated broker" \
  "broker request excludes repository root" \
  "enforce gates reject a recall floor" \
  "report-only gates record violations without failing" \
  "publish bundle contains sanitized files only"
do
  grep -qF "ok   $claim" "$out"
done
grep -qF "trigger-recall selftest: PASS" "$out"

if find "$tmp" -mindepth 1 -maxdepth 1 -type d -name 'megapowers-trigger-recall-*' | grep -q .; then
  echo "FAIL trigger recall left temporary state behind" >&2
  exit 1
fi

go run "$ROOT/evals/studies/trigger-recall/run.go" --validate-config \
  --cases "$ROOT/evals/studies/trigger-recall/cases.json" \
  --gates "$ROOT/evals/studies/trigger-recall/gates.json" >/dev/null

# Every shipped skill keeps at least verbatim, paraphrase, and buried recall
# probes; the shared pool keeps at least ten pure no-skill probes.
while read -r skill; do
  count="$(jq --arg s "$skill" '[.cases[] | select(.expected == $s)] | length' \
    "$ROOT/evals/studies/trigger-recall/cases.json")"
  if (( count < 3 )); then
    echo "FAIL skill $skill has $count recall probes; require 3" >&2
    exit 1
  fi
done < <(jq -r '.skills[].name' "$ROOT/plugins/megapowers/skills/catalog.json")

jq -e '[.cases[] | select(.kind == "no-skill")] | length >= 10' \
  "$ROOT/evals/studies/trigger-recall/cases.json" >/dev/null || {
  echo 'FAIL no-skill pool is smaller than ten probes' >&2
  exit 1
}
jq -e '[.cases[] | select(.kind == "no-skill") | .expected] | all(. == null)' \
  "$ROOT/evals/studies/trigger-recall/cases.json" >/dev/null || {
  echo 'FAIL a no-skill probe declares an expected skill' >&2
  exit 1
}
jq -e '[.cases[].id] | length == (unique | length)' \
  "$ROOT/evals/studies/trigger-recall/cases.json" >/dev/null || {
  echo 'FAIL duplicate case ids' >&2
  exit 1
}
jq -e '[.cases[].provenance] | all(length > 0)' \
  "$ROOT/evals/studies/trigger-recall/cases.json" >/dev/null || {
  echo 'FAIL a probe is missing provenance' >&2
  exit 1
}

if go run "$ROOT/evals/studies/trigger-recall/run.go" --run \
  --cases "$ROOT/evals/studies/trigger-recall/cases.json" \
  --gates "$ROOT/evals/studies/trigger-recall/gates.json" \
  --harness claude --model fake --out "$tmp/not-credentialed" >/dev/null 2>&1; then
  echo "FAIL live trigger run did not require explicit credentialed mode" >&2
  exit 1
fi

if go run "$ROOT/evals/studies/trigger-recall/run.go" --run --credentialed \
  --cases "$ROOT/evals/studies/trigger-recall/cases.json" \
  --gates "$ROOT/evals/studies/trigger-recall/gates.json" \
  --harness claude --model fake --out "$tmp/no-broker" >/dev/null 2>&1; then
  echo "FAIL live trigger run accepted no isolation broker" >&2
  exit 1
fi

if grep -qE '\.credentials\.json|auth\.json|copyCredential' "$ROOT/evals/studies/trigger-recall/run.go"; then
  echo "FAIL trigger recall runner contains credential-copy logic" >&2
  exit 1
fi

echo "trigger recall contract: PASS"
