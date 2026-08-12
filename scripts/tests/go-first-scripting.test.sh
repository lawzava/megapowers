#!/usr/bin/env bash
# Contract: agent-authored scripts are Go, including inside Python/TS repos.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
CODEX="$ROOT/templates/CODEX.md"
CLAUDE="$ROOT/templates/CLAUDE.md"
OPENCODE="$ROOT/templates/OPENCODE.md"
USING="$ROOT/plugins/megapowers/skills/using-megapowers/SKILL.md"
AGENTS="$ROOT/AGENTS.md"
SKILL="$ROOT/plugins/mega-go/skills/scripting-in-go/SKILL.md"
README="$ROOT/README.md"

fail() {
  printf 'go-first scripting contract: %s\n' "$*" >&2
  exit 1
}

flat_file() {
  tr '\n' ' ' < "$1" | tr -s ' '
}

must_contain() {
  local file=$1
  local phrase=$2
  local flat
  flat=$(flat_file "$file")
  [[ $flat == *"$phrase"* ]] ||
    fail "${file#"$ROOT/"} missing: $phrase"
}

for template in "$CODEX" "$CLAUDE" "$OPENCODE"; do
  grep -q '^## Tooling$' "$template" ||
    fail "${template#"$ROOT/"} missing Tooling section"
  must_contain "$template" 'go run'
  must_contain "$template" 'Do not write Python'
  must_contain "$template" 'Node'
  must_contain "$template" 'multi-line bash'
  must_contain "$template" 'Python or TypeScript'
done

grep -q '^## Scripting$' "$USING" ||
  fail "using-megapowers missing Scripting section"
must_contain "$USING" 'go run'
must_contain "$USING" 'Do not write Python'
must_contain "$USING" 'Python or TypeScript'

grep -q '^## Scripting$' "$AGENTS" ||
  fail "AGENTS.md missing Scripting section"
must_contain "$AGENTS" 'go run'

[[ -f $SKILL ]] || fail "mega-go skill scripting-in-go/SKILL.md missing"
must_contain "$SKILL" 'go run'
must_contain "$SKILL" 'stdlib'

must_contain "$README" 'go run'

printf 'go-first scripting contract: ok\n'
