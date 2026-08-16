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
echo 'Deterministic regressions validate executable contracts. They are not behavioral skill evidence.'
echo
echo '| skill | deterministic regressions | behavioral studies | behavioral evidence |'
echo '|---|---:|---:|---|'
while IFS= read -r skill; do
  [ -n "$skill" ] || continue
  regressions="$(
    declared=0
    for manifest in "$ROOT"/evals/scenarios/*/scenario.toml; do
      [ -f "$manifest" ] || continue
      scenario_skill="$(sed -n 's/^[[:space:]]*skill[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | head -1)"
      scenario_kind="$(sed -n 's/^[[:space:]]*kind[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | head -1)"
      if [ "$scenario_skill" = "$skill" ] && [ "$scenario_kind" = artifact ]; then
        declared=$((declared + 1))
      fi
    done
    printf '%s\n' "$declared"
  )"
  studies="$(awk -F '\t' -v skill="$skill" '!/^#/ && $2 == skill && $3 == "behavioral" { count++ } END { print count + 0 }' "$ROOT/evals/studies/coverage.tsv")"
  if [ "$studies" -gt 0 ]; then evidence=study-declared; else evidence=none; fi
  printf '| %s | %s | %s | %s |\n' "$skill" "$regressions" "$studies" "$evidence"
done <<< "$skills"
