#!/usr/bin/env bash
# run-recall.sh — measure ORGANIC skill triggering: with the megapowers plugin
# installed in a fresh config home, do on-topic tasks (which never name a skill)
# invoke the right skill, and do off-topic tasks stay quiet?
#
#   run-recall.sh --out DIR [--n 6] [--model claude-fable-5] [--parallel 4]
#
# One plugin-installed config home is built per invocation, then COPIED per run
# (parallel sessions must not share mutable state). Requires claude CLI +
# credentials — run outside any credential-blocking sandbox.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HERE/../../.." && pwd)"

# task table: prompt-file|expected-skill(or - for negatives)|fixture-arg
# on-*        : tuned-wording positives (the original, published set)
# held-*      : held-out paraphrases of the same intents — different wording,
#               so recall here measures intent matching, not phrase echo
# orch-*      : mega-orchestration positives (requires both plugins installed)
# neg-*       : negatives; neg-mention-* contain trigger WORDS without trigger
#               intent (explain/describe only) — the precision probes
TASKS='on-tdd|test-driven-development|
on-debug|systematic-debugging|--bug
on-brainstorm|brainstorming|
on-plans|writing-plans|
held-tdd|test-driven-development|
held-debug|systematic-debugging|--bug
held-brainstorm|brainstorming|
held-plans|writing-plans|
orch-autonomous|autonomous-run|
orch-verify|cross-model-verification|
orch-bestof|best-of-n|
orch-route|orchestrating|
neg-rename|-|
neg-explain|-|
neg-convert|-|
neg-list|-|
neg-mention-tdd|-|
neg-mention-parallel|-|'

run_one() { # task|expected|fixarg|idx|out|model|template
  local task expected fixarg idx out model tpl
  IFS='|' read -r task expected fixarg idx out model tpl <<< "$1"
  local rundir; rundir="$out/$task/run-$(printf '%02d' "$idx")"
  [ -f "$rundir/meta.json" ] && return 0
  rm -rf "$rundir"; mkdir -p "$rundir"
  local work; work="$(mktemp -d "${TMPDIR:-/tmp}/trig.XXXXXX")" || return 1
  cp -r "$tpl" "$work/cfg"
  "$HERE/fixtures/setup-project.sh" "$work/repo" $fixarg >/dev/null 2>&1
  local rc t0=$SECONDS
  ( cd "$work/repo" && CLAUDE_CONFIG_DIR="$work/cfg" timeout 480 claude -p "$(cat "$HERE/prompts/$task.txt")" \
      --model "$model" --max-turns 14 \
      --dangerously-skip-permissions --no-session-persistence \
      --output-format stream-json --verbose \
      > "$rundir/transcript.jsonl" 2> "$rundir/stderr.log" )
  rc=$?
  # every Skill-tool invocation, one name per line
  jq -r 'select(.type=="assistant") | .message.content[]?
         | select(.type=="tool_use" and .name=="Skill")
         | .input.skill // .input.command // empty' \
    "$rundir/transcript.jsonl" > "$rundir/skills-invoked.txt" 2>/dev/null
  jq -n --arg task "$task" --arg expected "$expected" --arg model "$model" \
        --argjson idx "$idx" --argjson rc "$rc" --argjson secs "$((SECONDS - t0))" \
        '{task:$task, expected:$expected, model:$model, idx:$idx, rc:$rc, seconds:$secs}' \
        > "$rundir/meta.json"
  rm -rf "$work"
  echo "done: $task/run-$idx rc=$rc invoked=[$(paste -sd, "$rundir/skills-invoked.txt")]"
}

if [ "${1:-}" = "--job" ]; then run_one "$2"; exit $?; fi

OUT="" N=6 MODEL="claude-fable-5" PAR=4
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --n) N="$2"; shift 2 ;;
    --model) MODEL="$2"; shift 2 ;;
    --parallel) PAR="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ -n "$OUT" ] || { echo "usage: run-recall.sh --out DIR [--n N] [--model M] [--parallel P]" >&2; exit 2; }
mkdir -p "$OUT"; OUT="$(cd "$OUT" && pwd)"

# build the plugin-installed config-home template once. A template left by a
# pre-orchestration version of this script lacks mega-orchestration and would
# silently score every orch-* positive as MISS — refuse it.
TPL="$OUT/cfg-template"
if [ -d "$TPL" ] && [ ! -f "$OUT/setup-install-orch.log" ]; then
  echo "stale cfg-template in $OUT (built without mega-orchestration); use a fresh --out dir" >&2
  exit 2
fi
if [ ! -d "$TPL" ]; then
  mkdir -p "$TPL"
  cp "$HOME/.claude/.credentials.json" "$TPL/" || { echo "no credentials" >&2; exit 1; }
  CLAUDE_CONFIG_DIR="$TPL" timeout 300 claude plugin marketplace add "$REPO_ROOT" \
    > "$OUT/setup-marketplace.log" 2>&1 || { echo "marketplace add failed" >&2; exit 1; }
  CLAUDE_CONFIG_DIR="$TPL" timeout 300 claude plugin install megapowers@megapowers \
    > "$OUT/setup-install.log" 2>&1 || { echo "plugin install failed" >&2; exit 1; }
  # the orch-* positives need the orchestration plugin; installing it also
  # raises the precision bar for every negative (more skills that must stay quiet)
  CLAUDE_CONFIG_DIR="$TPL" timeout 300 claude plugin install mega-orchestration@megapowers \
    > "$OUT/setup-install-orch.log" 2>&1 || { echo "mega-orchestration install failed" >&2; exit 1; }
fi

jobs="$(mktemp)"
for idx in $(seq 1 "$N"); do
  while IFS='|' read -r task expected fixarg; do
    [ -n "$task" ] && echo "$task|$expected|$fixarg|$idx|$OUT|$MODEL|$TPL" >> "$jobs"
  done <<< "$TASKS"
done
echo "$(wc -l < "$jobs") runs (parallel=$PAR) -> $OUT"
xargs -d '\n' -P "$PAR" -I{} "$0" --job {} < "$jobs"
rm -f "$jobs"
echo "all runs finished; score with: oracle.sh $OUT"
