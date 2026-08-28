#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

out="$tmp/selftest.out"
TMPDIR="$tmp" go run "$ROOT/evals/studies/installed-ab/run.go" --selftest >"$out"

for claim in \
  "isolated private homes" \
  "identical treatment and control inputs" \
  "undersampled diagnostics still write results" \
  "plugin inventory records empty control" \
  "temporary state removed after success" \
  "temporary state removed after actor failure" \
  "actor errors fail closed" \
  "actor failures publish explicit rejection" \
  "behavioral rows expose action progress metrics" \
  "manifest schema records the run environment" \
  "already-direct prose remains unchanged" \
  "fact alternatives accept paraphrase and reject semantic reversal" \
  "treatment prose requires declared skill activation" \
  "control prose remains outcome-only" \
  "activation-only treatment failures leave observed outcome lift unchanged" \
  "autonomous status resumption stays report-only" \
  "workflow accepts ordered skill selection" \
  "treatment workflow requires declared skill activation" \
  "control workflow remains outcome-only" \
  "workflow rejects a missing skill" \
  "workflow rejects reversed skill order" \
  "workflow rejects a forbidden skill" \
  "workflow rejects an unlisted extra skill" \
  "workflow rejects a missing required event" \
  "workflow rejects a forbidden event attempt" \
  "workflow rejects an incomplete trace" \
  "workflow rejects relaxed gates" \
  "continuity workflows stay report-only" \
  "orchestration accepts three completed agents before wait" \
  "treatment orchestration requires declared skill activation" \
  "control orchestration remains outcome-only" \
  "orchestration rejects two agents" \
  "orchestration rejects an early wait" \
  "orchestration rejects serial interleaving before the spawn batch" \
  "orchestration rejects a missing completion" \
  "orchestration distinguishes output-only and inline work" \
  "orchestration rejects missing output-only agent" \
  "orchestration rejects excess output-only agents" \
  "orchestration rejects raw output-only payload" \
  "orchestration rejects failed output-only spawn attempt" \
  "orchestration rejects an invalid bounded return" \
  "orchestration rejects an oversized bounded return" \
  "orchestration rejects an inline agent" \
  "orchestration rejects failed inline spawn attempt" \
  "orchestration applies the effective fanout gate to ordering" \
  "orchestration rejects a non-single output-only configuration" \
  "safe effects accepts a complete write-free trace" \
  "safe effects rejects every comment attempt" \
  "safe effects rejects missing trace_complete" \
  "safe effect helpers mutate protected fixtures" \
  "simultaneous test and implementation edit rejected" \
  "implementation filenames cannot spoof a test edit" \
  "unchanged TDD fixture fails its public oracle" \
  "TDD oracle rejects protected fixture tampering" \
  "live runs require isolated broker" \
  "treatment uses a verified private plugin copy" \
  "broker request excludes repository root" \
  "broker request carries the isolated oracle" \
  "broker request carries the disposable actor home" \
  "isolation attestation rejects credentials and siblings" \
  "actor deadlines fail closed" \
  "resume reports the exact manifest mismatch field" \
  "resume reports the exact row locale mismatch" \
  "insufficient paired runs fail study acceptance" \
  "treatment reliability fails study acceptance" \
  "control outcomes remain diagnostic" \
  "perfect treatment reliability clears study acceptance" \
  "treatment and empty-control hashes differ" \
  "schema rows pass strict scorer" \
  "publish bundle contains sanitized files only"
do
  grep -qF "ok   $claim" "$out"
done
grep -qF "installed-ab selftest: PASS" "$out"

if find "$tmp" -mindepth 1 -maxdepth 1 -type d -name 'megapowers-installed-ab-*' | grep -q .; then
  echo "FAIL installed A/B left temporary state behind" >&2
  exit 1
fi

go run "$ROOT/evals/studies/installed-ab/run.go" --validate-config \
  --cases "$ROOT/evals/studies/installed-ab/cases.json" \
  --gates "$ROOT/evals/studies/installed-ab/gates.json" >/dev/null

jq -e '
  (.cases[] | select(.id == "orchestration-three-read-lanes") | .required_skill_order) == ["orchestrating"] and
  (.cases[] | select(.id == "orchestration-output-only-evidence") | .required_skill_order) == ["orchestrating"] and
  (.cases[] | select(.id == "orchestration-bounded-inline") | .required_skill_order) == ["orchestrating"] and
  (.cases[] | select(.id == "design-plan-ambiguous-contract") | .required_skill_order) == ["orchestrating", "design-and-plan"] and
  (.cases[] | select(.id == "continuity-multisession-resume") | .required_skill_order) == ["orchestrating", "autonomous-run"]
' "$ROOT/evals/studies/installed-ab/cases.json" >/dev/null || {
  echo "FAIL installed A/B skill routing order does not match the orchestration contract" >&2
  exit 1
}

if go run "$ROOT/evals/studies/installed-ab/run.go" --run \
  --cases "$ROOT/evals/studies/installed-ab/cases.json" \
  --gates "$ROOT/evals/studies/installed-ab/gates.json" \
  --harness claude --model fake --out "$tmp/not-credentialed" >/dev/null 2>&1; then
  echo "FAIL live A/B run did not require explicit credentialed mode" >&2
  exit 1
fi

if go run "$ROOT/evals/studies/installed-ab/run.go" --run --credentialed \
  --cases "$ROOT/evals/studies/installed-ab/cases.json" \
  --gates "$ROOT/evals/studies/installed-ab/gates.json" \
  --harness claude --model fake --out "$tmp/no-broker" >/dev/null 2>&1; then
  echo "FAIL live A/B run accepted no isolation broker" >&2
  exit 1
fi
if grep -qE '\.credentials\.json|auth\.json|copyCredential' "$ROOT/evals/studies/installed-ab/run.go"; then
  echo "FAIL installed A/B runner contains credential-copy logic" >&2
  exit 1
fi

source "$ROOT/evals/studies/lib.sh"
private="$(study_private_tmpdir megapowers-contract)"
case "$(ls -ld "$private")" in drwx------*) ;; *) exit 1 ;; esac
rm -rf "$private"

bash "$ROOT/evals/studies/install-smoke/run-smoke.sh" --selftest >/dev/null
if grep -q 'opencode' "$ROOT/evals/studies/install-smoke/run-smoke.sh"; then
  echo "FAIL install smoke still contains OpenCode support" >&2
  exit 1
fi
smoke="$ROOT/evals/studies/install-smoke/run-smoke.sh"
if grep -qE '\.credentials\.json|auth\.json|claude -p|codex exec|dangerously-skip-permissions' "$smoke"; then
  echo "FAIL install smoke contains credential copying or model invocation" >&2
  exit 1
fi
for marker in 'plugin list --json' installPath installedPath \
  'skills/evidence-research/SKILL.md'; do
  grep -qF "$marker" "$smoke" || {
    echo "FAIL install smoke lacks registration marker: $marker" >&2
    exit 1
  }
done

echo "installed A/B contract: PASS"
