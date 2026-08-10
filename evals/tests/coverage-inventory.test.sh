#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
inventory="$ROOT/evals/coverage-inventory.sh"

test -x "$inventory"
out="$("$inventory")"
grep -q '^# megapowers eval coverage inventory$' <<<"$out"
grep -q '^32 shipped skills$' <<<"$out"
grep -q '| skill | declared scenarios | declared studies | declared evaluation |' <<<"$out"
grep -q '| greenfield-go-stack | 0 | 0 | none |' <<<"$out"
grep -q '| test-driven-development |' <<<"$out"
if grep -Eq '\| (covered|uncovered) \|' <<<"$out"; then
  echo 'FAIL declaration inventory claims derived coverage proof' >&2
  exit 1
fi
grep -q 'manually maintained evidence association' "$ROOT/evals/studies/coverage.tsv"

echo "coverage inventory contract: ok"
