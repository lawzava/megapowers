#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../run-loop.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP" || exit 1
TR="$TMP/transcript.jsonl"

pass=0
fail=0
verdict() {
  local out
  out="$(printf '%s' "$1" | bash "$HOOK" 2>/dev/null)"
  if [ -n "$out" ] && ! printf '%s' "$out" | jq -e . >/dev/null 2>&1; then echo BADJSON; return; fi
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
verdict_env() {
  local name="$1" value="$2" input="$3" out
  out="$(printf '%s' "$input" | env "$name=$value" bash "$HOOK" 2>/dev/null)"
  if [ -n "$out" ] && ! printf '%s' "$out" | jq -e . >/dev/null 2>&1; then echo BADJSON; return; fi
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
j() {
  printf '{"session_id":"%s","stop_hook_active":%s,"transcript_path":"%s","permission_mode":"%s"}' \
    "${1:-s1}" "${2:-false}" "${3:-$TR}" "${4:-default}"
}
check() {
  local got
  got="$(verdict "$2")"
  if [ "$1" = "$got" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$got" "$3"; fi
}
check_got() {
  if [ "$1" = "$2" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$2" "$3"; fi
}
mkrun() {
  mkdir -p ".megapowers/run/$1"
  printf 'STATE=%s\nCURSOR=M2\nLEVEL=%s\nLAST_VERIFY=none\n' "$2" "${3:-on-the-loop}" > ".megapowers/run/$1/status"
}
owner() {
  jq -cn --arg sid "$2" '{schema:1,session_id:$sid,role:"autonomous-run"}' > ".megapowers/run/$1/owner.json"
}

echo "== run-loop explicit ownership tests =="
: > "$TR"
check ALLOW "$(j s1 false "$TR")" "no run"

mkrun r1 working
printf '{"type":"tool_result","content":"read .megapowers/run/r1/plan.md"}\n' > "$TR"
check ALLOW "$(j s1 false "$TR")" "reading a run does not claim it"

printf '{"type":"tool_result","content":"quoted docs: {\"command\":\"scripts/run-claim r1\"}"}\n' > "$TR"
check ALLOW "$(j s1 false "$TR")" "quoted run-claim text does not claim a run"

printf '{"type":"tool_use","name":"Bash","input":{"command":"plugins/mega-orchestration/skills/autonomous-run/scripts/run-claim r1"}}\n' > "$TR"
check BLOCK "$(j s1 false "$TR")" "explicit run-claim invocation claims and drives run"
if jq -e '.session_id == "s1" and .role == "autonomous-run"' .megapowers/run/r1/owner.json >/dev/null 2>&1; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL run claim did not persist owner receipt"
fi

rm -f .megapowers/run/r1/owner.json
printf '{"type":"tool_use","name":"Bash","input":{"command":"scripts/run-claim r1"}}\n{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}\n' > "$TR"
check BLOCK "$(j s1 false "$TR")" "later Bash activity does not erase an earlier claim"

mkrun 'r.2' working
printf '{"type":"tool_use","name":"Bash","input":{"command":"scripts/run-claim rX2"}}\n' > "$TR"
check ALLOW "$(j s2 false "$TR")" "invalid filesystem run id cannot act as a regex"
rm -rf .megapowers/run/r.2

: > "$TR"
check BLOCK "$(j s1 false "$TR")" "matching persisted owner blocks"
check ALLOW "$(j other false "$TR")" "foreign session does not own run"

printf 'not-json\n' > .megapowers/run/r1/owner.json
check ALLOW "$(j s1 false "$TR")" "malformed owner fails open"
rm -f .megapowers/run/r1/owner.json
check ALLOW "$(j '' false "$TR")" "missing session id fails open"

owner r1 s1
check ALLOW "$(j s1 true "$TR")" "stop-hook recursion guard"
check ALLOW "$(j s1 false "$TR" plan)" "plan permission is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_ROLE verify "$(j s1 false "$TR")")" "verify role is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_ROLE judge "$(j s1 false "$TR")")" "judge role is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_PRESET read_only "$(j s1 false "$TR")")" "read-only preset is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_EXACT_OUTPUT 1 "$(j s1 false "$TR")")" "exact-output session is exempt"

mkrun r1 blocked
check ALLOW "$(j s1 false "$TR")" "blocked state allows stop"
mkrun r1 paused
check ALLOW "$(j s1 false "$TR")" "paused state allows stop"
mkrun r1 "done"
check ALLOW "$(j s1 false "$TR")" "done state allows stop"
mkrun r1 working in-the-loop
check ALLOW "$(j s1 false "$TR")" "in-the-loop checkpoint allows stop"
mkrun r1 working on-the-loop
check BLOCK "$(j s1 false "$TR")" "owned active on-the-loop run blocks"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
