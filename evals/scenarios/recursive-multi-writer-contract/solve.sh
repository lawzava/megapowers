#!/usr/bin/env bash
set -uo pipefail

skill="$ROOT/plugins/megapowers/skills/subagent-driven-development/SKILL.md"
prompt="$ROOT/plugins/megapowers/skills/subagent-driven-development/coordinator-prompt.md"
writing="$ROOT/plugins/megapowers/skills/writing-plans/SKILL.md"
codex="$ROOT/templates/CODEX-LEAD.md"
claude="$ROOT/templates/CLAUDE.md"
guidance_policy="$ROOT/evals/scenarios/recursive-multi-writer-contract/guidance-policy.awk"
guidance_policy_test="$ROOT/evals/scenarios/recursive-multi-writer-contract/guidance-policy.test.sh"

mark() {
  name=$1; shift
  if "$@"; then printf 'OK %s\n' "$name"; else printf 'MISSING %s\n' "$name"; fi
}

contains() {
  file=$1 pattern=$2
  flattened=$(tr '\n' ' ' < "$file") || return 1
  grep -Eiq "$pattern" <<< "$flattened"
}

validates_policy() {
  file=$1
  awk -f "$guidance_policy" "$file"
}

{
  mark plan-fields contains "$writing" 'Parallel safety.{0,240}Ownership.{0,240}May decompose'
  mark private-refs contains "$skill" 'refs/megapowers/runs/'
  mark bounded-worktrees contains "$skill" 'three writer worktrees and one integration worktree'
  mark bounded-cache contains "$skill" 'bounded shared cache|bounded per-run cache'
  mark branch-ownership contains "$skill" 'one writer.{0,100}(branch|worktree)|(branch|worktree).{0,100}one writer'
  mark sequential-commit-authorization contains "$skill" 'Sequential SDD commits once per task when the human selects this skill'
  mark recursive-explicit-commit-authorization contains "$skill" 'requires separate, explicit, run-specific human authorization.{0,200}--allow-task-commits'
  mark recursive-selection-not-authorization contains "$skill" 'Selecting this skill or the recursive workflow alone is not authorization'
  mark commit-authorization-boundary contains "$skill" 'In neither mode does task-local commit authorization grant push, merge-to-main, pull request, release, or deploy authority'
  mark coordinator-result contains "$prompt" 'Return one final result to the parent'
  mark release-lifecycle contains "$prompt" 'release the integration slot.{0,240}release the writer slot.{0,120}release the exact writer slot token.{0,120}release.*node claim'
  mark no-inexact-writer-release contains "$prompt" 'release the exact writer slot token recorded earlier'
  mark owner-target contains "$skill" 'run owner.{0,120}(alone|only).{0,100}(target|feature)'
  mark no-stale-takeover contains "$skill" '(never|do not).{0,100}(steal|release).{0,100}stale'
  mark fail-closed contains "$prompt" '(fail closed|Do not fall back to the parent checkout)'
  mark codex-fresh contains "$prompt" 'fork_turns = "none"'
  mark codex-depth-five contains "$prompt" 'If it already has five task-name components beneath /root, do not spawn another subagent; continue locally or report the limit\.'
  mark claude-no-teams contains "$prompt" 'Do not use agent teams because teams do not nest'
  mark no-recursive-agent-teams contains "$prompt" 'Do not use agent teams because teams do not nest'
  mark formal-policy validates_policy "$prompt"
  mark policy-fixtures bash "$guidance_policy_test"
  mark codex-lead-rule contains "$codex" 'Recursive SDD is the only multi-writer exception'
  mark claude-lead-rule contains "$claude" 'Recursive SDD uses nested Agent calls, not agent teams'
  mark registry-tests bash "$ROOT/plugins/megapowers/skills/subagent-driven-development/scripts/tests/sdd-run.test.sh"
  mark worktree-tests bash "$ROOT/plugins/megapowers/skills/subagent-driven-development/scripts/tests/sdd-worktree.test.sh"
} > out.txt

cat out.txt
