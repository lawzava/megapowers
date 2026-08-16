#!/usr/bin/env bash
# Run deterministic regressions. This suite proves executable contracts only;
# behavioral treatment/control studies are separate keyed release evidence.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVALS="$ROOT/evals"
jsonout=""
timeout_seconds=60
while [ $# -gt 0 ]; do
  case "$1" in
    --json)
      [ $# -ge 2 ] || { echo "--json requires a path" >&2; exit 2; }
      jsonout="$2"; shift 2
      ;;
    --timeout)
      [ $# -ge 2 ] || { echo "--timeout requires seconds" >&2; exit 2; }
      timeout_seconds="$2"; shift 2
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done
case "$timeout_seconds" in ''|*[!0-9]*) echo "timeout must be a positive integer" >&2; exit 2 ;; esac
[ "$timeout_seconds" -gt 0 ] || { echo "timeout must be a positive integer" >&2; exit 2; }

rows="$(mktemp)"
persist_error=0
persist_rows() {
  if [ -n "$jsonout" ]; then
    if ! cp "$rows" "$jsonout"; then
      persist_error=1
    fi
  fi
}
cleanup() {
  persist_rows
  rm -f "$rows"
}
trap cleanup EXIT HUP INT TERM

pass=0
fail=0
indeterminate=0
harness_errors=0
failed_ids=""

tally() {
  local row="$1" id="$2" verdict malformed_trace
  if ! printf '%s' "$row" | jq -e '
      type == "object" and .schema_version == "1" and
      (.status | type == "string") and (.verdict | type == "string")
    ' >/dev/null 2>&1; then
    malformed_trace="$(mktemp)"
    printf '%s\n' "$row" > "$malformed_trace"
    row="$(emit_selftest_row "runner-$id" "$EVALS/run.sh" harness_error 125 harness_error 0 "$malformed_trace")"
    rm -f "$malformed_trace"
  fi
  printf '%s\n' "$row" >> "$rows"
  persist_rows
  verdict="$(printf '%s' "$row" | jq -r '.verdict // "malformed"' 2>/dev/null)"
  case "$verdict" in
    pass)
      pass=$((pass + 1))
      printf '  \033[32mPASS\033[0m %s\n' "$id"
      ;;
    fail)
      fail=$((fail + 1)); failed_ids="$failed_ids $id"
      printf '  \033[31mFAIL\033[0m %s\n' "$id"
      ;;
    indeterminate)
      indeterminate=$((indeterminate + 1)); failed_ids="$failed_ids $id"
      printf '  \033[31mINDET\033[0m %s\n' "$id"
      ;;
    harness_error)
      harness_errors=$((harness_errors + 1)); failed_ids="$failed_ids $id"
      printf '  \033[31mHERR\033[0m %s\n' "$id"
      ;;
    *)
      harness_errors=$((harness_errors + 1)); failed_ids="$failed_ids $id"
      printf '  \033[31mHERR\033[0m %s (malformed result row)\n' "$id"
      ;;
  esac
}

hash_stream() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 | awk '{print $NF}'
  else
    return 127
  fi
}

hash_file() {
  if [ -f "$1" ]; then hash_stream < "$1"; else printf 'missing:%s\n' "$1" | hash_stream; fi
}

plugin_hash="$({
  if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ROOT" ls-files -co --exclude-standard -- plugins 2>/dev/null |
      LC_ALL=C sort |
      while IFS= read -r file; do
        [ -f "$ROOT/$file" ] && printf '%s\t%s\n' "$file" "$(hash_file "$ROOT/$file")"
      done
  elif [ -d "$ROOT/plugins" ]; then
    find "$ROOT/plugins" -type f -print |
      LC_ALL=C sort |
      while IFS= read -r file; do printf '%s\t%s\n' "$file" "$(hash_file "$file")"; done
  else
    printf 'no-plugin-tree\n'
  fi
} | hash_stream 2>/dev/null)"
empty_hash="$(printf '' | hash_stream 2>/dev/null)"
plugin_hash="${plugin_hash:-${empty_hash:-$(printf '%064d' 0)}}"
empty_hash="${empty_hash:-$(printf '%064d' 0)}"

source_revision="unversioned"
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse HEAD >/dev/null 2>&1; then
  source_revision="$(git -C "$ROOT" rev-parse HEAD)"
  if ! git -C "$ROOT" diff --quiet -- plugins evals 2>/dev/null; then source_revision="$source_revision-dirty"; fi
fi
source_repository="$(basename "$ROOT")"
os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch_name="$(uname -m)"
locale_name="${LC_ALL:-${LANG:-C}}"; locale_name="${locale_name// /_}"
sandbox_name="${CODEX_SANDBOX:-unknown}"

now_ms() {
  local value
  value="$(date +%s%N 2>/dev/null)"
  case "$value" in
    ''|*[!0-9]*) echo $(( $(date +%s) * 1000 )) ;;
    *) echo $(( value / 1000000 )) ;;
  esac
}

timeout_bin=""
if command -v timeout >/dev/null 2>&1; then timeout_bin="$(command -v timeout)"
elif command -v gtimeout >/dev/null 2>&1; then timeout_bin="$(command -v gtimeout)"
fi

emit_selftest_row() {
  local id="$1" target="$2" status="$3" rc="$4" verdict="$5" duration_ms="$6" trace="$7"
  local fixture_hash trace_hash timestamp task_success run_id
  fixture_hash="$(hash_file "$target" 2>/dev/null)"; fixture_hash="${fixture_hash:-$empty_hash}"
  trace_hash="$(hash_file "$trace" 2>/dev/null)"; trace_hash="${trace_hash:-$empty_hash}"
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  task_success=0; [ "$verdict" = pass ] && task_success=1
  run_id="$id-$(now_ms)-$$"
  jq -cn \
    --arg case_id "$id" --arg run_id "$run_id" --arg block_id "$id" \
    --arg source_repository "$source_repository" --arg source_revision "$source_revision" \
    --arg fixture_hash "sha256:$fixture_hash" --arg plugin_hash "sha256:$plugin_hash" \
    --arg status "$status" --arg verdict "$verdict" --argjson rc "$rc" \
    --argjson duration_ms "$duration_ms" --argjson task_success "$task_success" \
    --arg trace_hash "sha256:$trace_hash" --arg os "$os_name" --arg arch "$arch_name" \
    --arg sandbox "$sandbox_name" --arg locale "$locale_name" --arg timestamp "$timestamp" \
    '{schema_version:"1",study:"deterministic-regression/selftest",evidence_class:"regression",
      case_id:$case_id,run_id:$run_id,block_id:$block_id,arm:"regression",
      harness:{name:"local",cli_version:"local",model:"none",effort:"none"},
      source:{repository:$source_repository,revision:$source_revision},
      prompt_hash:"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      fixture_hash:$fixture_hash,plugin_hash:$plugin_hash,status:$status,rc:$rc,
      duration_ms:$duration_ms,verdict:$verdict,metrics:{task_success:$task_success},
      artifacts:{trace:$trace_hash},environment:{os:$os,arch:$arch,sandbox:$sandbox,locale:$locale},
      timestamp:$timestamp,phase:"selftest"}'
}

run_selftest() {
  local id="$1" target="$2"; shift 2
  local trace started ended rc status verdict row
  trace="$(mktemp)"
  started="$(now_ms)"
  if [ -z "$timeout_bin" ]; then
    printf 'timeout or gtimeout is required\n' > "$trace"
    rc=125; status="harness_error"; verdict="harness_error"
  else
    "$timeout_bin" "$timeout_seconds" "$@" >"$trace" 2>&1
    rc=$?
    case "$rc" in
      0) status="completed"; verdict="pass" ;;
      124) status="timeout"; verdict="harness_error" ;;
      126|127) status="harness_error"; verdict="harness_error" ;;
      *) status="completed"; verdict="fail" ;;
    esac
  fi
  ended="$(now_ms)"
  row="$(emit_selftest_row "$id" "$target" "$status" "$rc" "$verdict" "$((ended - started))" "$trace")"
  rm -f "$trace"
  tally "$row" "$id"
}

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to emit and validate result rows" >&2
  exit 2
fi

for sdir in "$EVALS"/scenarios/*/; do
  [ -f "$sdir/scenario.toml" ] || continue
  id="$(basename "$sdir")"
  row="$(bash "$EVALS/run.sh" "$id" --timeout "$timeout_seconds")"
  tally "$row" "$id"
done

if command -v go >/dev/null 2>&1; then
  run_selftest score-go-selftest "$EVALS/score.go" go run "$EVALS/score.go" --selftest
else
  trace="$(mktemp)"; printf 'go is required\n' > "$trace"
  row="$(emit_selftest_row score-go-selftest "$EVALS/score.go" harness_error 127 harness_error 0 "$trace")"
  rm -f "$trace"; tally "$row" score-go-selftest
fi

run_selftest install-smoke-runner-selftest \
  "$EVALS/studies/install-smoke/run-smoke.sh" \
  bash "$EVALS/studies/install-smoke/run-smoke.sh" --selftest
run_selftest installed-ab-runner-selftest \
  "$EVALS/studies/installed-ab/run.go" \
  go run "$EVALS/studies/installed-ab/run.go" --selftest
run_selftest pr-replay-runner-selftest \
  "$EVALS/studies/pr-replay/replay.go" \
  go run "$EVALS/studies/pr-replay/replay.go" --selftest

strict_failed=0
if command -v go >/dev/null 2>&1; then
  if ! go run "$EVALS/score.go" --strict "$rows" >/dev/null; then strict_failed=1; fi
else
  strict_failed=1
fi
persist_rows

echo
echo "== evals: $pass passed, $fail failed, $indeterminate indeterminate, $harness_errors harness errors =="
[ -n "$failed_ids" ] && echo "   failed:$failed_ids"
[ "$persist_error" -eq 0 ] && [ "$strict_failed" -eq 0 ] && [ "$fail" -eq 0 ] &&
  [ "$indeterminate" -eq 0 ] && [ "$harness_errors" -eq 0 ]
