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
  "plugin inventory records empty control" \
  "temporary state removed after success" \
  "temporary state removed after actor failure" \
  "actor errors fail closed" \
  "already-direct prose remains unchanged" \
  "autonomous status resumption stays report-only" \
  "simultaneous test and implementation edit rejected" \
  "live runs require isolated broker" \
  "treatment uses a verified private plugin copy" \
  "broker request excludes repository root" \
  "isolation attestation rejects credentials and siblings" \
  "actor deadlines fail closed" \
  "insufficient paired runs block publication" \
  "treatment reliability blocks publication" \
  "control outcomes remain diagnostic" \
  "perfect treatment reliability clears publication" \
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
  'skills/test-first-implementation/SKILL.md'; do
  grep -qF "$marker" "$smoke" || {
    echo "FAIL install smoke lacks registration marker: $marker" >&2
    exit 1
  }
done

echo "installed A/B contract: PASS"
