#!/usr/bin/env bash
# Report evaluated skills from declared scenario and study oracle manifests.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
skills="$(find "$ROOT/plugins" -type f -name SKILL.md -exec dirname {} \; | while IFS= read -r dir; do basename "$dir"; done | sort)"
count="$(printf '%s\n' "$skills" | sed '/^$/d' | wc -l | tr -d ' ')"

echo '# megapowers eval coverage inventory'
echo
echo "$count shipped skills"
echo
echo '| skill | declared scenarios | declared studies | declared evaluation |'
echo '|---|---:|---:|---|'
while IFS= read -r skill; do
  [ -n "$skill" ] || continue
  scenarios="$( (grep -RhsE "^[[:space:]]*skill[[:space:]]*=[[:space:]]*\"$skill\"" "$ROOT/evals/scenarios" 2>/dev/null || true) | wc -l | tr -d ' ')"
  studies="$(awk -F '\t' -v skill="$skill" '!/^#/ && $2 == skill { count++ } END { print count + 0 }' "$ROOT/evals/studies/coverage.tsv")"
  if [ "$scenarios" -gt 0 ] || [ "$studies" -gt 0 ]; then declaration=present; else declaration=none; fi
  printf '| %s | %s | %s | %s |\n' "$skill" "$scenarios" "$studies" "$declaration"
done <<< "$skills"
