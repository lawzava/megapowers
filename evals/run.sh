#!/usr/bin/env bash
# Run one deterministic regression scenario and emit one schema-v1 JSON row.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVALS="$ROOT/evals"

id=""
keep=0
timeout_seconds=60
while [ $# -gt 0 ]; do
  case "$1" in
    --keep) keep=1; shift ;;
    --timeout)
      [ $# -ge 2 ] || { echo "--timeout requires seconds" >&2; exit 2; }
      timeout_seconds="$2"; shift 2
      ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *)
      [ -z "$id" ] || { echo "only one scenario id is allowed" >&2; exit 2; }
      id="$1"; shift
      ;;
  esac
done
[ -n "$id" ] || { echo "usage: run.sh <scenario-id> [--timeout seconds] [--keep]" >&2; exit 2; }
case "$id" in *[!A-Za-z0-9._-]*|'') echo "invalid scenario id: $id" >&2; exit 2 ;; esac
case "$timeout_seconds" in ''|*[!0-9]*) echo "timeout must be a positive integer" >&2; exit 2 ;; esac
[ "$timeout_seconds" -gt 0 ] || { echo "timeout must be a positive integer" >&2; exit 2; }

sdir="$EVALS/scenarios/$id"
[ -f "$sdir/scenario.toml" ] || { echo "no such scenario: $id" >&2; exit 2; }
[ -f "$sdir/check.sh" ] || { echo "scenario $id has no check.sh" >&2; exit 2; }

tget() {
  sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" "$sdir/scenario.toml" |
    head -1 |
    sed 's/^"//; s/"$//'
}
kind="$(tget kind)"; kind="${kind:-artifact}"
prompt="$(tget prompt)"

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
  if [ -f "$1" ]; then
    hash_stream < "$1"
  else
    printf 'missing:%s\n' "$1" | hash_stream
  fi
}

hash_fixture() {
  local name
  for name in scenario.toml setup.sh solve.sh check.sh; do
    if [ -f "$sdir/$name" ]; then
      printf '%s\t%s\n' "$name" "$(hash_file "$sdir/$name")"
    fi
  done | hash_stream
}

hash_plugins() {
  local file
  if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ROOT" ls-files -co --exclude-standard -- plugins 2>/dev/null |
      LC_ALL=C sort |
      while IFS= read -r file; do
        [ -f "$ROOT/$file" ] && printf '%s\t%s\n' "$file" "$(hash_file "$ROOT/$file")"
      done |
      hash_stream
  elif [ -d "$ROOT/plugins" ]; then
    find "$ROOT/plugins" -type f -print |
      LC_ALL=C sort |
      while IFS= read -r file; do
        printf '%s\t%s\n' "${file#"$ROOT/"}" "$(hash_file "$file")"
      done |
      hash_stream
  else
    printf 'no-plugin-tree\n' | hash_stream
  fi
}

now_ms() {
  local value
  value="$(date +%s%N 2>/dev/null)"
  case "$value" in
    ''|*[!0-9]*) echo $(( $(date +%s) * 1000 )) ;;
    *) echo $(( value / 1000000 )) ;;
  esac
}

timeout_bin=""
if command -v timeout >/dev/null 2>&1; then
  timeout_bin="$(command -v timeout)"
elif command -v gtimeout >/dev/null 2>&1; then
  timeout_bin="$(command -v gtimeout)"
fi

started="$(now_ms)"
workdir="$(mktemp -d "${TMPDIR:-/tmp}/mpeval.$id.XXXXXX")"
trace="$workdir/.trace"
: > "$trace"
cleanup() { [ "$keep" -eq 1 ] || rm -rf "$workdir"; }
trap cleanup EXIT

phase="setup"
stage_rc=0
status="completed"
if [ "$kind" != "artifact" ]; then
  printf 'only artifact scenarios are deterministic regressions; got %s\n' "$kind" >> "$trace"
  stage_rc=2
  status="harness_error"
elif [ -z "$timeout_bin" ]; then
  printf 'timeout or gtimeout is required for bounded evaluation\n' >> "$trace"
  stage_rc=125
  status="harness_error"
elif [ -f "$sdir/setup.sh" ]; then
  "$timeout_bin" "$timeout_seconds" env SCENARIO_DIR="$sdir" ROOT="$ROOT" \
    bash -c 'cd "$1" && exec bash "$2"' run-stage "$workdir" "$sdir/setup.sh" >>"$trace" 2>&1
  stage_rc=$?
  if [ "$stage_rc" -eq 124 ]; then status="timeout"; elif [ "$stage_rc" -ne 0 ]; then status="harness_error"; fi
fi

if [ "$stage_rc" -eq 0 ]; then
  phase="actor"
  if [ -f "$sdir/solve.sh" ]; then
    "$timeout_bin" "$timeout_seconds" env SCENARIO_DIR="$sdir" ROOT="$ROOT" \
      bash -c 'cd "$1" && exec bash "$2"' run-stage "$workdir" "$sdir/solve.sh" >>"$trace" 2>&1
    stage_rc=$?
    if [ "$stage_rc" -eq 124 ]; then status="timeout"; elif [ "$stage_rc" -ne 0 ]; then status="harness_error"; fi
  fi
fi

if [ "$stage_rc" -eq 0 ]; then
  phase="oracle"
  "$timeout_bin" "$timeout_seconds" env WORKDIR="$workdir" TRACE="$trace" SCENARIO_DIR="$sdir" ROOT="$ROOT" \
    bash -c 'cd "$1" && exec bash "$2"' run-stage "$workdir" "$sdir/check.sh" >>"$trace" 2>&1
  stage_rc=$?
  if [ "$stage_rc" -eq 124 ]; then
    status="timeout"
  fi
fi

case "$status:$stage_rc" in
  completed:0) verdict="pass" ;;
  completed:77) verdict="indeterminate" ;;
  completed:*) verdict="fail" ;;
  *) verdict="harness_error" ;;
esac

ended="$(now_ms)"
duration_ms=$((ended - started)); [ "$duration_ms" -lt 0 ] && duration_ms=0
empty_hash="$(printf '' | hash_stream 2>/dev/null)"; empty_hash="${empty_hash:-$(printf '%064d' 0)}"
prompt_hash="$(printf '%s' "$prompt" | hash_stream 2>/dev/null)"; prompt_hash="${prompt_hash:-$empty_hash}"
fixture_hash="$(hash_fixture 2>/dev/null)"; fixture_hash="${fixture_hash:-$empty_hash}"
plugin_hash="$(hash_plugins 2>/dev/null)"; plugin_hash="${plugin_hash:-$empty_hash}"
trace_hash="$(hash_file "$trace" 2>/dev/null)"; trace_hash="${trace_hash:-$empty_hash}"
source_revision="unversioned"
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse HEAD >/dev/null 2>&1; then
  source_revision="$(git -C "$ROOT" rev-parse HEAD)"
  if ! git -C "$ROOT" diff --quiet -- plugins evals 2>/dev/null; then
    source_revision="$source_revision-dirty"
  fi
fi
source_repository="$(basename "$ROOT")"
timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
run_id="$id-$started-$$"
task_success=0; [ "$verdict" = pass ] && task_success=1
os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch_name="$(uname -m)"
locale_name="${LC_ALL:-${LANG:-C}}"; locale_name="${locale_name// /_}"
sandbox_name="${CODEX_SANDBOX:-unknown}"

if command -v jq >/dev/null 2>&1; then
  jq -cn \
    --arg study "deterministic-regression/scenario" \
    --arg case_id "$id" \
    --arg run_id "$run_id" \
    --arg block_id "$id" \
    --arg source_repository "$source_repository" \
    --arg source_revision "$source_revision" \
    --arg prompt_hash "sha256:$prompt_hash" \
    --arg fixture_hash "sha256:$fixture_hash" \
    --arg plugin_hash "sha256:$plugin_hash" \
    --arg status "$status" \
    --arg verdict "$verdict" \
    --argjson rc "$stage_rc" \
    --argjson duration_ms "$duration_ms" \
    --argjson task_success "$task_success" \
    --arg trace_hash "sha256:$trace_hash" \
    --arg os "$os_name" \
    --arg arch "$arch_name" \
    --arg sandbox "$sandbox_name" \
    --arg locale "$locale_name" \
    --arg timestamp "$timestamp" \
    --arg phase "$phase" \
    '{schema_version:"1",study:$study,evidence_class:"regression",case_id:$case_id,
      run_id:$run_id,block_id:$block_id,arm:"regression",
      harness:{name:"local",cli_version:"local",model:"none",effort:"none"},
      source:{repository:$source_repository,revision:$source_revision},
      prompt_hash:$prompt_hash,fixture_hash:$fixture_hash,plugin_hash:$plugin_hash,
      status:$status,rc:$rc,duration_ms:$duration_ms,verdict:$verdict,
      metrics:{task_success:$task_success},artifacts:{trace:$trace_hash},
      environment:{os:$os,arch:$arch,sandbox:$sandbox,locale:$locale},
      timestamp:$timestamp,phase:$phase}'
else
  printf '{"schema_version":"1","study":"deterministic-regression/scenario","evidence_class":"regression","case_id":"%s","run_id":"%s","block_id":"%s","arm":"regression","harness":{"name":"local","cli_version":"local","model":"none","effort":"none"},"source":{"repository":"%s","revision":"%s"},"prompt_hash":"sha256:%s","fixture_hash":"sha256:%s","plugin_hash":"sha256:%s","status":"%s","rc":%d,"duration_ms":%d,"verdict":"%s","metrics":{"task_success":%d},"artifacts":{"trace":"sha256:%s"},"environment":{"os":"%s","arch":"%s","sandbox":"%s","locale":"%s"},"timestamp":"%s","phase":"%s"}\n' \
    "$id" "$run_id" "$id" "$source_repository" "$source_revision" "$prompt_hash" "$fixture_hash" "$plugin_hash" \
    "$status" "$stage_rc" "$duration_ms" "$verdict" "$task_success" "$trace_hash" "$os_name" "$arch_name" \
    "$sandbox_name" "$locale_name" "$timestamp" "$phase"
fi

[ "$verdict" = "pass" ]
