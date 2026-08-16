#!/usr/bin/env bash
# Keep harness mechanics out of portable semantic skill bodies.
set -euo pipefail

root_arg="${1:-$(dirname "$0")/..}"
ROOT="$(cd "$root_arg" 2>/dev/null && pwd -P)" || {
  echo "check-portability-boundary: cannot resolve root: $root_arg" >&2
  exit 2
}
allowed='plugins/megapowers/skills/independent-review/SKILL.md'
skills_file="$(mktemp)"
trap 'rm -f "$skills_file"' EXIT
if ! find "$ROOT/plugins" -type f -path '*/skills/*/SKILL.md' -print0 > "$skills_file"; then
  echo "check-portability-boundary: skill discovery failed under $ROOT/plugins" >&2
  exit 2
fi
if [ ! -s "$skills_file" ]; then
  echo "check-portability-boundary: no skills discovered under $ROOT/plugins" >&2
  exit 2
fi
bad=0
scanned=0
while IFS= read -r -d '' skill; do
  scanned=$((scanned + 1))
  rel="${skill#"$ROOT/"}"
  [ "$rel" = "$allowed" ] && continue
  if grep -Ein \
    -e 'gpt-[0-9]+[.][0-9]+' \
    -e 'claude-[a-z]+-[0-9]' \
    -e '(^|[^[:alnum:]_-])codex([^[:alnum:]_]|$)' \
    -e '(^|[^[:alnum:]_-])claude([^[:alnum:]_]|$)' \
    -e '(^|[^[:alnum:]_-])fork_turns([^[:alnum:]_]|$)' \
    "$skill"; then
    bad=1
  else
    rc=$?
    [ "$rc" -eq 1 ] || {
      echo "check-portability-boundary: scanner failed for $rel (exit $rc)" >&2
      exit 2
    }
  fi
done < "$skills_file"
[ "$scanned" -gt 0 ] || {
  echo "check-portability-boundary: no readable skills discovered under $ROOT/plugins" >&2
  exit 2
}
[ "$bad" -eq 0 ]
