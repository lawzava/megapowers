#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
SDD="$ROOT/plugins/megapowers/skills/subagent-driven-development/SKILL.md"
CODEX="$ROOT/templates/CODEX-LEAD.md"
CLAUDE="$ROOT/templates/CLAUDE.md"
PRIMITIVES="$ROOT/plugins/mega-orchestration/skills/orchestrating/references/harness-primitives.md"
PLANS="$ROOT/plugins/megapowers/skills/writing-plans/SKILL.md"
SUPPORT="$ROOT/docs/harness-support.md"
README="$ROOT/plugins/megapowers/README.md"
AGENT_INSTALL="$ROOT/docs/agent-install.md"
SETUP="$ROOT/docs/setup.md"

guidance_files=(
  "$SDD"
  "$PLANS"
  "$CODEX"
  "$CLAUDE"
  "$PRIMITIVES"
  "$SUPPORT"
  "$README"
  "$AGENT_INSTALL"
  "$SETUP"
)

fail() {
  printf 'recursive guidance contract: %s\n' "$*" >&2
  exit 1
}

runtime_paths=(
  plugins/megapowers/skills/subagent-driven-development/coordinator-prompt.md
  plugins/megapowers/skills/subagent-driven-development/scripts/run-lib.sh
  plugins/megapowers/skills/subagent-driven-development/scripts/sdd-run
  plugins/megapowers/skills/subagent-driven-development/scripts/sdd-worktree
  plugins/megapowers/skills/subagent-driven-development/scripts/tests/sdd-run.test.sh
  plugins/megapowers/skills/subagent-driven-development/scripts/tests/sdd-worktree.test.sh
  evals/scenarios/recursive-multi-writer-contract
)

for path in "${runtime_paths[@]}"; do
  [[ ! -e "$ROOT/$path" ]] || fail "runtime artifact still ships: $path"
done

expected_scripts=$'ownership-preflight\nreview-package\nsdd-workspace\ntask-brief\ntests/ownership-preflight.test.sh'
actual_scripts=$(
  cd "$ROOT/plugins/megapowers/skills/subagent-driven-development/scripts" || exit 1
  find . -type f -print | sed 's#^\./##' | sort
)
[[ $actual_scripts == "$expected_scripts" ]] || fail 'SDD skill scripts changed; recursive mode must not add a runtime'

for file in "${guidance_files[@]}"; do
  if rg -q 'sdd-run|sdd-worktree|refs/megapowers/runs|writer slot|integration slot|stale claim|heartbeat' "$file"; then
    fail "agent guidance contains recursive runtime protocol: ${file#"$ROOT/"}"
  fi
done

if rg -qF 'recursive-guidance.test.sh' "$SDD" "$PLANS" "$CODEX" "$CLAUDE" "$PRIMITIVES"; then
  fail 'repository test leaked into agent-facing guidance'
fi

bytes=$(wc -c < "$SDD" | tr -d ' ')
(( bytes <= 18000 )) || fail "SDD skill exceeds 18000 bytes: $bytes"

must_have() {
  local file=$1
  local text=$2
  local body
  body=$(awk '{$1=$1; printf "%s ", $0}' "$file")
  [[ $body == *"$text"* ]] || fail "missing contract text in ${file#"$ROOT/"}: $text"
}

must_not_have() {
  local file=$1
  local text=$2
  local body
  local needle
  body=$(awk '{$1=$1; printf "%s ", $0}' "$file" | LC_ALL=C tr '[:upper:]' '[:lower:]')
  needle=$(printf '%s' "$text" | LC_ALL=C tr '[:upper:]' '[:lower:]')
  [[ $body != *"$needle"* ]] || fail "forbidden recursive runtime guidance in ${file#"$ROOT/"}: $text"
}

for phrase in \
  'create a worktree for each child' \
  'create worktrees for children' \
  'per-child worktree' \
  'child worktree' \
  'recursive worktree' \
  'run registry' \
  'claim protocol' \
  'slot allocator' \
  'custom scheduler' \
  'private Git refs' \
  'heartbeat'; do
  for file in "${guidance_files[@]}"; do
    must_not_have "$file" "$phrase"
  done
done

must_have "$SDD" 'Recursive coordinator mode is guidance for native Codex and Claude Code subagents, not an execution runtime.'
must_have "$SDD" 'recursive multi-writer'
must_have "$SDD" 'All writers share the current checkout; recursive mode creates no worktrees.'
must_have "$SDD" 'A coordinator may subdivide only the ownership it inherited.'
must_have "$SDD" 'The lead coordinates only its direct children.'
must_have "$SDD" 'returns one synthesized result to its parent'
must_have "$SDD" 'the requirement to wait for its direct children and return one synthesized subtree result'
must_have "$SDD" 'Overlapping ownership, shared interface changes, and dependencies stay sequential.'
must_have "$SDD" 'Concurrent children do not run Git index or ref mutations.'
must_have "$SDD" 'Only the top-level lead performs any authorized Git action'
must_have "$SDD" 'fork_turns = "none"'
must_have "$SDD" 'nested Agent calls'
must_have "$SDD" 'agent teams because teams cannot nest'
must_have "$SDD" 'scripts/ownership-preflight PLAN_FILE'

must_have "$PLANS" '**Parallel safety:**'
must_have "$PLANS" '**Ownership:**'
must_have "$PLANS" '**May decompose:**'

must_have "$CODEX" 'For explicitly selected recursive coordinator mode, native subagents may write concurrently only to disjoint owned paths in the shared checkout.'
must_have "$CODEX" 'Do not create worktrees for this mode.'
must_have "$CODEX" 'Each coordinator waits for its direct children, verifies their combined edits, and returns one synthesized subtree result to its parent.'
must_have "$CLAUDE" 'Recursive coordinator mode uses nested Agent calls, not agent teams.'
must_have "$CLAUDE" 'Do not create worktrees for this mode.'
must_have "$CLAUDE" 'Children must not perform Git index or ref operations.'
must_have "$CLAUDE" 'Each coordinator waits for its direct children, verifies their combined edits, and returns one synthesized subtree result to its parent.'
must_have "$PRIMITIVES" 'Recursive coordinator mode is guidance over native nested subagents.'
must_have "$SUPPORT" 'Megapowers runs a plan preflight before dispatch'
must_have "$SUPPORT" 'no registry, scheduler, or worktree manager participates.'
must_have "$README" 'native recursive coordinator guidance for Codex and Claude Code'
must_have "$AGENT_INSTALL" 'Recursive coordinator mode is the explicit shared-checkout exception; do not create worktrees for it.'
must_have "$SETUP" 'Recursive coordinator mode is the explicit shared-checkout exception; do not create worktrees for it.'

printf 'recursive guidance contract: ok\n'
