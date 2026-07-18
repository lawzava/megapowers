#!/usr/bin/env bash
# lib.sh — shared runner core for the real-agent studies. Sourced by the
# per-study run-*.sh scripts; each keeps only its fixture setup, prompt
# naming, and ground-truth diagnostics.

# model alias used in run-dir paths
study_malias() {
  case "$1" in
    claude-haiku-4-5) printf 'haiku' ;;
    claude-fable-5)   printf 'frontier' ;;
    *)                printf '%s' "$1" | tr -c '[:alnum:].-' '-' ;;
  esac
}

# which CLI serves this model
study_agent() { case "$1" in gpt-*|codex*) printf 'codex' ;; *) printf 'claude' ;; esac; }

# Run one subject-agent session in <repo> with <prompt_file>, writing
# transcript.jsonl (claude-shaped), transcript-raw.jsonl (codex only),
# final-message.txt, and stderr.log into <rundir>. Returns the agent's rc.
#
# Codex notes: --ignore-user-config drops the user's config.toml AND global
# AGENTS.md (verified: a subject asked to quote outside instructions reports
# none) while auth still comes from CODEX_HOME. Its JSONL is normalized into
# the claude event shape the oracles read: completed command_executions
# become Bash tool_use events (bash -lc wrapper stripped so anchored regexes
# see the inner command); completed file_changes become ONE Write tool_use
# joining all changed paths (a patch that writes test+impl together must
# score as one simultaneous write, not test-first).
#
# Claude notes: --safe-mode keeps user-level CLAUDE.md, plugins, and hooks
# out of BOTH arms so ambient discipline config cannot confound the control.
study_exec() { # <agent> <model> <repo> <prompt_file> <rundir> <run_timeout> <max_turns>
  local agent="$1" model="$2" repo="$3" prompt="$4" rundir="$5" run_timeout="$6" max_turns="$7" rc
  if [ "$agent" = codex ]; then
    ( cd "$repo" && timeout "$run_timeout" codex exec --json --ephemeral \
        --ignore-user-config --ignore-rules --skip-git-repo-check \
        -C "$repo" -s workspace-write -c approval_policy='"never"' -m "$model" \
        "$(cat "$prompt")" \
        > "$rundir/transcript-raw.jsonl" 2> "$rundir/stderr.log" </dev/null )
    rc=$?
    jq -c 'select(.type=="item.completed") | .item
           | if .type=="command_execution" then
               {type:"assistant", message:{content:[{type:"tool_use", name:"Bash",
                 input:{command: (.command // ""
                   | sub("^(/bin/)?(ba)?sh -lc ";"") | sub("^['\''\"]";"") | sub("['\''\"]$";""))}}]}}
             elif .type=="file_change" then
               {type:"assistant", message:{content:[{type:"tool_use", name:"Write",
                 input:{file_path: ((.changes // []) | map(.path) | join(" "))}}]}}
             else empty end' \
      "$rundir/transcript-raw.jsonl" > "$rundir/transcript.jsonl" 2>> "$rundir/stderr.log"
    jq -rs '[.[] | select(.type=="item.completed") | .item | select(.type=="agent_message") | .text] | last // empty' \
      "$rundir/transcript-raw.jsonl" > "$rundir/final-message.txt" 2>/dev/null
  else
    ( cd "$repo" && timeout "$run_timeout" claude -p "$(cat "$prompt")" \
        --safe-mode --model "$model" --max-turns "$max_turns" \
        --dangerously-skip-permissions --no-session-persistence \
        --output-format stream-json --verbose \
        > "$rundir/transcript.jsonl" 2> "$rundir/stderr.log" )
    rc=$?
    jq -r 'select(.type=="result") | .result // empty' \
      "$rundir/transcript.jsonl" > "$rundir/final-message.txt" 2>/dev/null
  fi
  return "$rc"
}

# Fan out job lines (stdin) through "$0 --job <line>", PAR at a time. The
# caller enumerates jobs idx-major so models/modes interleave and rate drift
# cannot bias one cell.
study_fanout() { # <par> <out>; job lines on stdin
  local par="$1" out="$2" jobs
  jobs="$(mktemp)"; cat > "$jobs"
  echo "$(wc -l < "$jobs") runs (parallel=$par) -> $out"
  xargs -d '\n' -P "$par" -I{} "$0" --job {} < "$jobs"
  rm -f "$jobs"
  echo "all runs finished; score with: oracle.sh $out"
}
