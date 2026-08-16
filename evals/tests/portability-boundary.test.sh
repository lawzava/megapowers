#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
guard="$ROOT/evals/check-portability-boundary.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/plugins/megapowers/skills/plain" \
  "$tmp/plugins/megapowers/skills/independent-review"
printf '%s\n' 'Use a provider SDK when it fits the stack.' > \
  "$tmp/plugins/megapowers/skills/plain/SKILL.md"
"$guard" "$tmp"

mkdir -p "$tmp/no-rg"
for command in bash dirname env find grep mktemp rm; do
  ln -s "$(command -v "$command")" "$tmp/no-rg/$command"
done
PATH="$tmp/no-rg" "$guard" "$tmp"

for forbidden in \
  'In Codex, use native nested agents.' \
  'Use Claude-specific mechanics.' \
  'Run `claude` directly.' \
  'Set fork_turns to none.' \
  'Pin gpt-5.6-sol.'; do
  printf '%s\n' "$forbidden" > "$tmp/plugins/megapowers/skills/plain/SKILL.md"
  if "$guard" "$tmp" >/dev/null 2>&1; then
    echo "FAIL semantic skill accepted harness mechanics: $forbidden" >&2
    exit 1
  fi
done

printf '%s\n' 'Use a provider SDK when it fits the stack.' > \
  "$tmp/plugins/megapowers/skills/plain/SKILL.md"
printf '%s\n' 'Review with Claude or Codex explicitly.' > \
  "$tmp/plugins/megapowers/skills/independent-review/SKILL.md"
"$guard" "$tmp"

empty="$tmp/empty"
mkdir -p "$empty/plugins"
if "$guard" "$empty" >/dev/null 2>&1; then
  echo 'FAIL portability guard accepted zero skills' >&2
  exit 1
fi

"$guard" "$ROOT"
echo 'portability boundary contract: ok'
