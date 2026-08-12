#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
CODEX="$ROOT/templates/CODEX.md"
CLAUDE="$ROOT/templates/CLAUDE.md"
OPENCODE="$ROOT/templates/OPENCODE.md"
RECEIVING="$ROOT/plugins/megapowers/skills/receiving-code-review/SKILL.md"
HUMANIZING="$ROOT/plugins/megapowers/skills/humanizing-prose/SKILL.md"

fail() {
  printf 'outward communication contract: %s\n' "$*" >&2
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

# A public comment is not a transcript sink. This pins the distinction in both
# always-loaded lead templates, where it applies before any prose skill loads.
for template in "$CODEX" "$CLAUDE" "$OPENCODE"; do
  grep -q '^## Outward communication$' "$template" ||
    fail "${template#"$ROOT/"} missing outward communication section"
  must_contain "$template" 'public artifact, not a session report'
  must_contain "$template" 'Reply where the question was asked'
  must_contain "$template" 'minimum evidence'
  must_contain "$template" 'progress narration'
  must_contain "$template" 'review ledgers'
  must_contain "$template" 'top-level status comment'
  must_contain "$template" 'verdict, blocker, and next action'
  must_contain "$template" 'at most three bullets'
  must_contain "$template" 'When another review is required'
  must_contain "$template" 'review trigger as its own minimal comment'
done

# Review handling stays thread-local instead of publishing a fresh aggregate
# ledger after every fix wave and repeating it again at close.
must_contain "$RECEIVING" 'existing review thread'
must_contain "$RECEIVING" 'current decision'
must_contain "$RECEIVING" 'minimum evidence'
must_contain "$RECEIVING" 'aggregate top-level review ledger'
must_contain "$RECEIVING" 'verdict, blocker, and next action'
must_contain "$RECEIVING" 'at most three bullets'
must_contain "$RECEIVING" 'When another review is required'
must_contain "$RECEIVING" 'review trigger as its own minimal comment'

# Humanizing removes process exhaust as well as stock AI vocabulary.
must_contain "$HUMANIZING" 'PR comments'
must_contain "$HUMANIZING" 'current decision'
must_contain "$HUMANIZING" 'minimum evidence'
must_contain "$HUMANIZING" 'Internal work history'
must_contain "$HUMANIZING" 'existing thread'
must_contain "$HUMANIZING" 'verdict, blocker, and next action'
must_contain "$HUMANIZING" 'at most three bullets'
must_contain "$HUMANIZING" 'review trigger as its own minimal comment'

printf 'outward communication contract: ok\n'
