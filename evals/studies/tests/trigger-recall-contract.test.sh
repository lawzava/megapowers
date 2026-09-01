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
  "rows carry final_words and final_em_dashes" \
  "manifest reports length measurements without thresholds" \
  "rows pass the strict scorer" \
  "reps emit unique blocks" \
  "temporary state removed after success" \
  "actor errors fail closed" \
  "actor failures persist private diagnostics" \
  "transient actor failures retry once" \
  "repeated actor failures still fail closed" \
  "missing skills catalog fails closed as infrastructure" \
  "missing skills catalog persists private diagnostics" \
  "missing skills catalog retries once" \
  "manifest records catalog_rendered" \
  "unavailable catalog signal never fails a run" \
  "actor deadlines fail closed" \
  "live runs require isolated broker" \
  "broker request excludes repository root" \
  "enforce gates reject a recall floor" \
  "report-only gates record violations without failing" \
  "em dash gate flags dashed finals while a median within limit passes" \
  "length gates flag a verbose precision median" \
  "absent length gates stay report-only" \
  "final_words strips fenced code and counts em dashes" \
  "unselectable skills are exempt from recall probe minimums" \
  "frontmatter flag detection reads only the frontmatter" \
  "enforcement applies only to listed harnesses" \
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

# Every shipped, model-selectable skill keeps at least verbatim, paraphrase,
# and buried recall probes; skills whose frontmatter sets
# `disable-model-invocation: true` cannot be selected and carry none. The
# shared pool keeps at least ten pure no-skill probes.
while read -r skill; do
  frontmatter="$(sed -n '2,/^---$/p' "$ROOT/plugins/megapowers/skills/$skill/SKILL.md")"
  count="$(jq --arg s "$skill" '[.cases[] | select(.expected == $s)] | length' \
    "$ROOT/evals/studies/trigger-recall/cases.json")"
  if grep -qE '^disable-model-invocation:[[:space:]]*true[[:space:]]*$' <<<"$frontmatter"; then
    if (( count != 0 )); then
      echo "FAIL skill $skill sets disable-model-invocation but has $count recall probes" >&2
      exit 1
    fi
    continue
  fi
  if (( count < 3 )); then
    echo "FAIL skill $skill has $count recall probes; require 3" >&2
    exit 1
  fi
done < <(jq -r '.skills[].name' "$ROOT/plugins/megapowers/skills/catalog.json")

# Removed skills leave no probes behind.
jq -e '[.cases[] | select(.id | startswith("code-quality"))] | length == 0' \
  "$ROOT/evals/studies/trigger-recall/cases.json" >/dev/null || {
  echo 'FAIL code-quality probes remain in the corpus' >&2
  exit 1
}

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

# The shipped gates enforce Claude only, encode the accepted
# intake-time-selection boundary for verify-and-finish, and bound Claude's
# final response length and em dash usage.
jq -e '
  .mode == "enforce" and
  .enforce_harnesses == ["claude"] and
  (.acceptance.per_skill | has("code-quality") | not) and
  .acceptance.per_skill["verify-and-finish"].min_recall == 0.3 and
  .acceptance.max_median_final_words == 120 and
  .acceptance.max_em_dash_rate == 0.1
' "$ROOT/evals/studies/trigger-recall/gates.json" >/dev/null || {
  echo 'FAIL shipped gates do not encode claude-only enforcement with accepted boundaries and length limits' >&2
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
