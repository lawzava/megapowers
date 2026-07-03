#!/usr/bin/env bash
# run.sh — run ONE eval scenario and emit a single JSON result row on stdout.
#
# Usage: run.sh <scenario-id> [--agent <name>] [--control] [--keep]
#
# Scenario kinds (declared in scenario.toml):
#   artifact  -> runs the scenario's solve.sh (a shipped script/hook) in a throwaway
#                workdir, then check.sh. No agent, no API key. CI-safe.
#   behavior  -> invokes an agent with the scenario prompt, then check.sh. Uses the
#                agent named by --agent (default: mock). --control withholds the skill.
#   trigger   -> like behavior, but check.sh passes when the skill did NOT fire.
#
# NEVER runs in the repo under test: every run gets a fresh mktemp workdir (an audit
# once had agent repros commit into the repo — the harness makes that impossible).
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVALS="$ROOT/evals"

id=""; agent="mock"; mode="skill"; keep=0
while [ $# -gt 0 ]; do
  case "$1" in
    --agent) agent="$2"; shift 2 ;;
    --control) mode="control"; shift ;;
    --keep) keep=1; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *) id="$1"; shift ;;
  esac
done
[ -n "$id" ] || { echo "usage: run.sh <scenario-id> [--agent <name>] [--control] [--keep]" >&2; exit 2; }

sdir="$EVALS/scenarios/$id"
[ -f "$sdir/scenario.toml" ] || { echo "no such scenario: $id" >&2; exit 2; }
[ -f "$sdir/check.sh" ] || { echo "scenario $id has no check.sh" >&2; exit 2; }

# minimal TOML value reader (key = "value" | key = value), first match wins.
tget() { sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" "$sdir/scenario.toml" | head -1 | sed 's/^"//; s/"$//'; }
kind="$(tget kind)"; kind="${kind:-artifact}"
skill="$(tget skill)"
prompt="$(tget prompt)"

# portable millisecond clock: prefer ns (Linux), fall back to seconds*1000 (BSD/macOS)
now_ms() {
  local n; n="$(date +%s%N 2>/dev/null)"
  case "$n" in ''|*[!0-9]*) echo $(( $(date +%s) * 1000 )) ;; *) echo $(( n / 1000000 )) ;; esac
}
started="$(now_ms)"
workdir="$(mktemp -d "${TMPDIR:-/tmp}/mpeval.$id.XXXXXX")"
trace="$workdir/.trace"
: > "$trace"
cleanup() { [ "$keep" -eq 1 ] || rm -rf "$workdir"; }
trap cleanup EXIT

# 1. setup (seed the workdir)
if [ -f "$sdir/setup.sh" ]; then
  ( cd "$workdir" && SCENARIO_DIR="$sdir" ROOT="$ROOT" bash "$sdir/setup.sh" ) >>"$trace" 2>&1
fi

# 2. act
case "$kind" in
  artifact)
    if [ -f "$sdir/solve.sh" ]; then
      ( cd "$workdir" && SCENARIO_DIR="$sdir" ROOT="$ROOT" bash "$sdir/solve.sh" ) >>"$trace" 2>&1
    fi
    ;;
  behavior|trigger)
    if [ "$agent" = "mock" ]; then
      if [ "$mode" = "control" ]; then
        : # control: the compliant behavior is withheld; do nothing
      elif [ -f "$sdir/mock/actions.sh" ]; then
        ( cd "$workdir" && SCENARIO_DIR="$sdir" ROOT="$ROOT" bash "$sdir/mock/actions.sh" ) >>"$trace" 2>&1
      fi
    else
      # real agent: resolve its command template from agents.toml (falls back to example)
      cfg="$EVALS/agents.toml"; [ -f "$cfg" ] || cfg="$EVALS/agents.example.toml"
      tmpl="$(sed -n "s/^[[:space:]]*${agent}[[:space:]]*=[[:space:]]*//p" "$cfg" | head -1 | sed 's/^"//; s/"$//')"
      if [ -z "$tmpl" ]; then echo "no agent template '$agent' in $cfg" >&2; exit 2; fi
      # skill vs control is expressed by the template (e.g. a {{SKILLS}} placeholder);
      # here we only substitute the prompt and workdir and run it.
      cmd="${tmpl//\{\{PROMPT\}\}/$prompt}"
      cmd="${cmd//\{\{WORKDIR\}\}/$workdir}"
      cmd="${cmd//\{\{MODE\}\}/$mode}"
      ( cd "$workdir" && eval "$cmd" ) >>"$trace" 2>&1
    fi
    ;;
  *) echo "unknown scenario kind: $kind" >&2; exit 2 ;;
esac

# 3. check (the oracle)
WORKDIR="$workdir" TRACE="$trace" SCENARIO_DIR="$sdir" MODE="$mode" \
  bash "$sdir/check.sh" >>"$trace" 2>&1
rc=$?
case "$rc" in
  0) verdict="pass" ;;
  77) verdict="indeterminate" ;;
  *) verdict="fail" ;;
esac

ended="$(now_ms)"
ms=$(( ended - started )); [ "$ms" -lt 0 ] && ms=0

# emit one JSON row (jq if present for safe escaping, else a plain line)
if command -v jq >/dev/null 2>&1; then
  jq -cn --arg id "$id" --arg skill "$skill" --arg kind "$kind" --arg agent "$agent" \
        --arg mode "$mode" --arg verdict "$verdict" --argjson ms "$ms" \
    '{scenario:$id, skill:$skill, kind:$kind, agent:$agent, mode:$mode, verdict:$verdict, ms:$ms}'
else
  printf '{"scenario":"%s","skill":"%s","kind":"%s","agent":"%s","mode":"%s","verdict":"%s","ms":%d}\n' \
    "$id" "$skill" "$kind" "$agent" "$mode" "$verdict" "$ms"
fi

[ "$verdict" = "pass" ]
