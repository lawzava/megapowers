#!/usr/bin/env bash
# Inventory declarations without upgrading deterministic checks into efficacy evidence.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
skills="$(find "$ROOT/plugins" -type f -name SKILL.md -exec dirname {} \; |
  while IFS= read -r directory; do basename "$directory"; done |
  LC_ALL=C sort -u)"
count="$(printf '%s\n' "$skills" | sed '/^$/d' | wc -l | tr -d ' ')"

echo '# megapowers eval coverage inventory'
echo
echo "$count shipped skills"
echo
echo 'Per-skill deterministic contract regressions live in scripts/tests/skill-contracts.test.sh.'
echo 'They validate executable contracts and are not behavioral skill evidence.'
echo
echo '| skill | behavioral studies | behavioral evidence |'
echo '|---|---:|---|'
while IFS= read -r skill; do
  [ -n "$skill" ] || continue
  studies="$(awk -F '\t' -v skill="$skill" '!/^#/ && $2 == skill && $3 == "behavioral" { count++ } END { print count + 0 }' "$ROOT/evals/studies/coverage.tsv")"
  if [ "$studies" -gt 0 ]; then evidence=study-declared; else evidence=none; fi
  printf '| %s | %s | %s |\n' "$skill" "$studies" "$evidence"
done <<< "$skills"
