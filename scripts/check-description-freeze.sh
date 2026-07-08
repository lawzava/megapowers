#!/usr/bin/env bash
# Fails when any skill's YAML frontmatter differs from BASE_REF (default v0.1.7).
# The de-prescription pass trims bodies only; descriptions are the tuned trigger
# surface and stay frozen (docs/megapowers/specs/2026-07-07-fable5-de-prescription-design.md).
set -u
BASE_REF="${1:-v0.1.7}"
fail=0
frontmatter() { awk '/^---$/{n++; next} n==1{print} n>=2{exit}'; }
for f in plugins/*/skills/*/SKILL.md; do
  if ! git cat-file -e "$BASE_REF:$f" 2>/dev/null; then
    echo "NEW (not in $BASE_REF, needs explicit approval): $f"; fail=1; continue
  fi
  if ! diff -q <(git show "$BASE_REF:$f" | frontmatter) <(frontmatter < "$f") >/dev/null; then
    echo "FRONTMATTER DRIFT vs $BASE_REF: $f"; fail=1
  fi
done
[ "$fail" -eq 0 ] && echo "description freeze OK ($BASE_REF)"
exit "$fail"
