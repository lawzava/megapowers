#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
guard="$ROOT/evals/check-portability-boundary.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/plugins/mega/skills/plain" "$tmp/plugins/mega-orchestration/skills/orchestrating"
printf '%s\n' 'Use a vendor SDK when it fits the stack.' > "$tmp/plugins/mega/skills/plain/SKILL.md"
"$guard" "$tmp"
mkdir -p "$tmp/no-rg"
for command in bash dirname env find grep mktemp rm sort; do
  ln -s "$(command -v "$command")" "$tmp/no-rg/$command"
done
if ! PATH="$tmp/no-rg" "$guard" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL portability guard requires ripgrep'
  exit 1
fi
printf '%s\n' 'Run Codex --json and OpenCode run.' >> "$tmp/plugins/mega/skills/plain/SKILL.md"
if PATH="$tmp/no-rg" "$guard" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL semantic skill accepted capitalized harness command'
  exit 1
fi
printf '%s\n' 'Use a vendor SDK when it fits the stack.' > "$tmp/plugins/mega/skills/plain/SKILL.md"
printf '%s\n' 'Run codex --json and opencode run with gpt-5.6-sol.' >> "$tmp/plugins/mega/skills/plain/SKILL.md"
if "$guard" "$tmp" >/dev/null 2>&1; then
  echo 'FAIL semantic skill accepted harness-specific command'
  exit 1
fi
mv "$tmp/plugins/mega/skills/plain/SKILL.md" "$tmp/plugins/mega-orchestration/skills/orchestrating/SKILL.md"
"$guard" "$tmp"
"$guard" "$tmp/"

mkdir -p "$tmp/plugins/mega/skills/plain"
for forbidden in \
  'In Codex, use native nested subagents.' \
  "Codex's native workflow" \
  'Run OpenCode.' \
  'Use Claude-specific mechanics.' \
  'Use `claude` directly.' \
  'Set fork_turns to none.'
do
  printf '%s\n' "$forbidden" > "$tmp/plugins/mega/skills/plain/SKILL.md"
  if "$guard" "$tmp" >/dev/null 2>&1; then
    echo "FAIL semantic skill accepted punctuated harness mechanics: $forbidden"
    exit 1
  fi
done
rm -rf "$tmp/plugins/mega/skills/plain"

empty="$tmp/empty-root"
mkdir -p "$empty/plugins"
if "$guard" "$empty" >/dev/null 2>&1; then
  echo 'FAIL portability guard accepted zero discovered skills'
  exit 1
fi
missing="$tmp/missing-root"
mkdir -p "$missing"
if "$guard" "$missing" >/dev/null 2>&1; then
  echo 'FAIL portability guard accepted failed skill discovery'
  exit 1
fi
"$guard" "$ROOT"
echo 'portability boundary contract: ok'
