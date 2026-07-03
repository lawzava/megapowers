#!/usr/bin/env bash
# run-h2h.sh — head-to-head: does an INSTALLED suite change discipline
# organically, and how does megapowers compare to upstream Superpowers?
#
# Three arms, same task (the gauntlet control prompt — task only, no skill
# named, no preamble): a bare config home, one with megapowers installed from
# this checkout, one with upstream Superpowers installed from its marketplace.
# Discipline is scored by the gauntlet's four sub-oracles; organic triggering
# is recorded per run (skills-invoked.txt). Unlike the gauntlet (which measures
# the WORDING, prepended), this measures the DELIVERED product: install, then
# see what the agent reaches for on its own.
#
#   run-h2h.sh --out DIR [--n 8] [--model claude-fable-5]
#              [--arms bare,megapowers,superpowers] [--parallel 4]
#              [--run-timeout 600] [--superpowers-source obra/superpowers]
#
# Score with the gauntlet oracle (arms appear as extra columns rows):
#   evals/studies/gauntlet/oracle.sh DIR
#
# Requires the claude CLI + credentials — run outside any credential-blocking
# sandbox. Claude Code only for now: the arm is an installed plugin, and the
# install surface differs per harness.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HERE/../../.." && pwd)"
GAUNTLET="$HERE/../gauntlet"

run_one() { # arm|idx|out|model|template|run_timeout
  local arm idx out model tpl run_timeout
  IFS='|' read -r arm idx out model tpl run_timeout <<< "$1"
  local malias
  case "$model" in
    claude-haiku-4-5) malias=haiku ;;
    claude-fable-5)   malias=frontier ;;
    *)                malias="$(printf '%s' "$model" | tr -c '[:alnum:].-' '-')" ;;
  esac
  local rundir; rundir="$out/$malias/$arm/run-$(printf '%02d' "$idx")"
  [ -f "$rundir/meta.json" ] && return 0
  rm -rf "$rundir"; mkdir -p "$rundir"
  local work; work="$(mktemp -d "${TMPDIR:-/tmp}/h2h.XXXXXX")" || return 1
  cp -r "$tpl" "$work/cfg"
  local repo="$work/repo"
  "$GAUNTLET/fixtures/setup-gauntlet.sh" "$repo" >/dev/null 2>&1 || { rm -rf "$work"; return 1; }
  git -C "$repo" rev-list --count --all > "$rundir/baseline-commits.txt"

  local t0=$SECONDS rc
  ( cd "$repo" && CLAUDE_CONFIG_DIR="$work/cfg" timeout "$run_timeout" \
      claude -p "$(cat "$GAUNTLET/prompts/gauntlet-control.txt")" \
      --model "$model" --max-turns 25 \
      --dangerously-skip-permissions --no-session-persistence \
      --output-format stream-json --verbose \
      > "$rundir/transcript.jsonl" 2> "$rundir/stderr.log" )
  rc=$?
  jq -r 'select(.type=="result") | .result // empty' \
    "$rundir/transcript.jsonl" > "$rundir/final-message.txt" 2>/dev/null
  # organic triggering: every Skill-tool invocation, one name per line
  jq -r 'select(.type=="assistant") | .message.content[]?
         | select(.type=="tool_use" and .name=="Skill")
         | .input.skill // .input.command // empty' \
    "$rundir/transcript.jsonl" > "$rundir/skills-invoked.txt" 2>/dev/null

  git -C "$repo" rev-list --count --all > "$rundir/commits-after.txt"
  git -C "$repo" status --porcelain > "$rundir/git-status.txt" 2>/dev/null
  git -C "$repo" diff --name-only HEAD > "$rundir/changed-files.txt" 2>/dev/null
  [ -f "$repo/.test-runs.log" ] && cp "$repo/.test-runs.log" "$rundir/test-runs.log"
  if ( cd "$repo" && timeout 60 python3 -m unittest -q >/dev/null 2>&1 ); then
    echo pass > "$rundir/suite-status.txt"; else echo fail > "$rundir/suite-status.txt"; fi
  local task=FAIL
  ( cd "$repo" && timeout 60 python3 -c 'from wordbench import word_freq; assert word_freq("a B a") == {"a": 2, "b": 1}' >/dev/null 2>&1 ) && task=PASS
  jq -n --arg model "$model" --arg agent claude --arg mode "$arm" --arg task "$task" \
        --argjson idx "$idx" --argjson rc "$rc" --argjson secs "$((SECONDS - t0))" \
        '{model:$model, agent:$agent, mode:$mode, idx:$idx, rc:$rc, seconds:$secs, task:$task}' \
        > "$rundir/meta.json"
  rm -rf "$work"
  echo "done: $malias/$arm/run-$idx rc=$rc task=$task invoked=[$(paste -sd, "$rundir/skills-invoked.txt" 2>/dev/null)]"
}

if [ "${1:-}" = "--job" ]; then run_one "$2"; exit $?; fi

OUT="" N=8 MODEL="claude-fable-5" ARMS="bare,megapowers,superpowers" PAR=4 RUN_TIMEOUT=600
SUPERPOWERS_SOURCE="obra/superpowers"
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --n) N="$2"; shift 2 ;;
    --model) MODEL="$2"; shift 2 ;;
    --arms) ARMS="$2"; shift 2 ;;
    --parallel) PAR="$2"; shift 2 ;;
    --run-timeout) RUN_TIMEOUT="$2"; shift 2 ;;
    --superpowers-source) SUPERPOWERS_SOURCE="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ -n "$OUT" ] || { echo "usage: run-h2h.sh --out DIR [--n N] [--model M] [--arms ..]" >&2; exit 2; }
mkdir -p "$OUT"; OUT="$(cd "$OUT" && pwd)"

# one config-home template per arm, built once, COPIED per run (parallel
# sessions must not share mutable state — same pattern as trigger-recall).
# Built in a staging dir and renamed only on success, so a half-built template
# from a failed install can never be silently reused as a (polluted) arm.
build_template() { # arm -> template dir on stdout
  # two `local`s: in a single one, $arm would expand before the assignment
  # takes effect (SC2318), leaving tpl/stage pointing at "cfg-".
  local arm="$1"
  local tpl="$OUT/cfg-$arm" stage="$OUT/cfg-$arm.building"
  if [ ! -d "$tpl" ]; then
    rm -rf "$stage"; mkdir -p "$stage"
    cp "$HOME/.claude/.credentials.json" "$stage/" || { echo "no credentials" >&2; return 1; }
    case "$arm" in
      bare) : ;;
      megapowers)
        CLAUDE_CONFIG_DIR="$stage" timeout 300 claude plugin marketplace add "$REPO_ROOT" \
          > "$OUT/setup-$arm-marketplace.log" 2>&1 || { echo "marketplace add failed ($arm)" >&2; return 1; }
        CLAUDE_CONFIG_DIR="$stage" timeout 300 claude plugin install megapowers@megapowers \
          > "$OUT/setup-$arm-install.log" 2>&1 || { echo "plugin install failed ($arm)" >&2; return 1; }
        ;;
      superpowers)
        CLAUDE_CONFIG_DIR="$stage" timeout 300 claude plugin marketplace add "$SUPERPOWERS_SOURCE" \
          > "$OUT/setup-$arm-marketplace.log" 2>&1 || { echo "marketplace add failed ($arm)" >&2; return 1; }
        CLAUDE_CONFIG_DIR="$stage" timeout 300 claude plugin install superpowers@superpowers \
          > "$OUT/setup-$arm-install.log" 2>&1 || { echo "plugin install failed ($arm)" >&2; return 1; }
        ;;
      *) echo "unknown arm: $arm" >&2; return 1 ;;
    esac
    mv "$stage" "$tpl" || return 1
  fi
  echo "$tpl"
}

jobs="$(mktemp)"
for arm in ${ARMS//,/ }; do
  tpl="$(build_template "$arm")" || exit 1
  for idx in $(seq 1 "$N"); do
    echo "$arm|$idx|$OUT|$MODEL|$tpl|$RUN_TIMEOUT" >> "$jobs"
  done
done
echo "$(wc -l < "$jobs") runs (parallel=$PAR) -> $OUT"
xargs -d '\n' -P "$PAR" -I{} "$0" --job {} < "$jobs"
rm -f "$jobs"
echo "all runs finished; score with: evals/studies/gauntlet/oracle.sh $OUT"
