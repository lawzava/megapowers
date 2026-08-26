#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
inventory="$ROOT/evals/coverage-inventory.sh"

test -x "$inventory"
out="$("$inventory")"
grep -q '^# megapowers eval coverage inventory$' <<<"$out"
declared_count="$(find "$ROOT/plugins" -type f -name SKILL.md | wc -l | tr -d ' ')"
grep -q "^$declared_count shipped skills$" <<<"$out"
grep -q '| skill | behavioral studies | behavioral evidence |' <<<"$out"
grep -q 'scripts/tests/skill-contracts.test.sh' <<<"$out"
grep -q '| autonomous-run |' <<<"$out"
if grep -Eq '\| (covered|uncovered|present) \|' <<<"$out"; then
  echo 'FAIL inventory upgrades declarations or regressions into efficacy claims' >&2
  exit 1
fi
grep -q 'deterministic regressions are not behavioral skill evidence' "$ROOT/evals/studies/coverage.tsv"

echo "coverage inventory contract: ok"
