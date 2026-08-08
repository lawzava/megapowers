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

study_hash_file() { # <file>
  if [ "${STUDY_HASH_FORCE_SHASUM:-0}" = 1 ]; then
    shasum -a 256 "$1"
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  else
    shasum -a 256 "$1"
  fi
}

study_hash_stream() { # stdin -> hash only
  if [ "${STUDY_HASH_FORCE_SHASUM:-0}" = 1 ]; then
    shasum -a 256
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum
  else
    shasum -a 256
  fi | awk '{print $1}'
}

study_tree_sha256() { # <root-for-relative-paths> <path>...
  local root="$1" file rel hash input list manifest result
  shift
  for input in "$@"; do
    [ -e "$input" ] || {
      echo "study provenance: declared input is missing: $input" >&2
      return 2
    }
  done
  list="$(mktemp)" || return 2
  manifest="$(mktemp)" || { rm -f "$list"; return 2; }
  if ! find -H "$@" -type f -print | LC_ALL=C sort > "$list"; then
    echo "study provenance: could not enumerate declared inputs" >&2
    rm -f "$list" "$manifest"
    return 2
  fi
  while IFS= read -r file; do
    rel="$file"
    case "$file" in "$root"/*) rel="${file#"$root"/}" ;; esac
    if ! hash="$(study_hash_file "$file" | awk '{print $1}')"; then
      echo "study provenance: could not hash declared input: $file" >&2
      rm -f "$list" "$manifest"
      return 2
    fi
    printf '%s\t%s\n' "$rel" "$hash" >> "$manifest"
  done < "$list"
  if ! result="$(study_hash_stream < "$manifest")"; then
    echo "study provenance: could not hash input manifest" >&2
    rm -f "$list" "$manifest"
    return 2
  fi
  rm -f "$list" "$manifest"
  printf '%s\n' "$result"
}

study_skill_hashes() { # <repository-root> -> JSON array, C-locale glob order
  local root="$1" skill name hash
  local LC_ALL=C
  for skill in "$root"/plugins/*/skills/*/SKILL.md; do
    [ -f "$skill" ] || continue
    name="$(basename "$(dirname "$skill")")"
    hash="$(study_hash_file "$skill" | awk '{print $1}')"
    jq -n --arg skill "$name" --arg sha256 "$hash" '{skill:$skill, sha256:$sha256}'
  done | jq -sc '.'
}

study_paths_overlap() { # <path-a> <path-b>; both canonical
  case "$1/" in "$2/"*) return 0 ;; esac
  case "$2/" in "$1/"*) return 0 ;; esac
  return 1
}

study_record_provenance() { # <existing-run-dir> <agent> <model> <prompt-file> <study-dir>
  local rundir="$1" agent="$2" model="$3" prompt="$4" study_dir="$5"
  local root sha version effort source_hash harness_hash runner prompt_hash skill_hashes
  local input provenance_tmp
  root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
  rundir="$(cd -P "$rundir" 2>/dev/null && pwd -P)" || {
    echo "study provenance: run directory must already exist: $1" >&2
    return 2
  }
  study_dir="$(cd -P "$study_dir" 2>/dev/null && pwd -P)" || {
    echo "study provenance: study directory does not exist: $5" >&2
    return 2
  }
  for input in "$root/plugins" "$study_dir/fixtures" "$study_dir/prompts"; do
    if study_paths_overlap "$rundir" "$input"; then
      echo "study provenance: run directory overlaps declared inputs: $rundir" >&2
      return 2
    fi
  done
  runner="$(cd -P "$(dirname "${BASH_SOURCE[1]}")" && pwd -P)/$(basename "${BASH_SOURCE[1]}")"
  sha="$(git -C "$root" rev-parse HEAD 2>/dev/null || printf unknown)"
  version="$($agent --version 2>/dev/null | head -1 || true)"
  effort="${STUDY_EFFORT:-}"
  source_hash="$(study_tree_sha256 "$root" "$root/plugins" "$study_dir/fixtures" "$study_dir/prompts")" || return 2
  harness_hash="$(study_tree_sha256 "$root" \
    "$root/evals/run.sh" "$root/evals/run-all.sh" "$root/evals/score.go" \
    "$root/evals/studies/lib.sh" "$root/evals/studies/provenance-check.sh" \
    "$runner" "$study_dir/fixtures" "$study_dir/prompts" "$study_dir/oracle.sh")" || return 2
  prompt_hash="$(study_hash_file "$prompt" | awk '{print $1}')" || return 2
  skill_hashes="$(study_skill_hashes "$root")" || return 2
  provenance_tmp="$(mktemp "$rundir/.provenance.XXXXXX")" || return 2
  jq -n --arg repo_sha "$sha" --arg agent "$agent" --arg cli_version "$version" \
    --arg model "$model" --arg effort "$effort" \
    --arg prompt_sha256 "$prompt_hash" \
    --arg source_sha256 "$source_hash" --arg harness_sha256 "$harness_hash" \
    --argjson skill_hashes "$skill_hashes" \
    '{schema_version:1, repo_sha:$repo_sha, source_sha256:$source_sha256, harness_sha256:$harness_sha256, agent_cli:$agent, cli_version:(if $cli_version == "" then null else $cli_version end), model_identity:$model, effort:(if $effort == "" then null else $effort end), prompt_sha256:$prompt_sha256, skill_hashes:$skill_hashes}' \
    > "$provenance_tmp" || { rm -f "$provenance_tmp"; return 2; }
  mv "$provenance_tmp" "$rundir/provenance.json" || { rm -f "$provenance_tmp"; return 2; }
}

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
  local effort="${STUDY_EFFORT:-}"
  local -a effort_args=()
  if [ "$agent" = codex ]; then
    [ -z "$effort" ] || effort_args=(-c "model_reasoning_effort=\"$effort\"")
    ( cd "$repo" && timeout "$run_timeout" codex exec --json --ephemeral \
        --ignore-user-config --ignore-rules --skip-git-repo-check \
        -C "$repo" -s workspace-write -c approval_policy='"never"' -m "$model" \
        "${effort_args[@]}" \
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
    [ -z "$effort" ] || effort_args=(--effort "$effort")
    ( cd "$repo" && timeout "$run_timeout" claude -p "$(cat "$prompt")" \
        --safe-mode --model "$model" --max-turns "$max_turns" \
        "${effort_args[@]}" \
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
  local par="$1" out="$2" jobs rc
  jobs="$(mktemp)"; cat > "$jobs"
  echo "$(wc -l < "$jobs") runs (parallel=$par) -> $out"
  xargs -d '\n' -P "$par" -I{} "$0" --job {} < "$jobs"
  rc=$?
  rm -f "$jobs"
  if [ "$rc" -ne 0 ]; then
    echo "one or more runs failed (rc=$rc); inspect harness_error metadata" >&2
    return "$rc"
  fi
  echo "all runs finished; score with: oracle.sh $out"
}
