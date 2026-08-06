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

# --- enforcement lifecycle: the per-repository opt-out, and where it is read ---
# The state of this gate is declared in ../enforcement.toml, so a repository can
# silence it from a project layer instead of patching the hook. These cases pin
# the layer precedence and the fail-closed default: a rules file that cannot be
# read must leave the gate ON, because the alternative is that deleting a file
# quietly disables a Stop-hook gate.
#
# THEY ALSO PIN WHERE THE PROJECT LAYER COMES FROM, and that is what changed. Every
# case below used to write the layer into the WORKTREE and expect it honored, which
# is the hook trusting policy written by the pending tree. This hook drives an
# autonomous run, and that run writes the tree: an uncommitted `state = "off"` let
# the run release itself with nothing said. The layer is read from the base commit
# now, so each of these cases commits it, and the pending cases at the end assert
# that an uncommitted one buys nothing and is announced instead.
#
# The BLOCK case immediately above is the baseline every case here mutates from:
# same run, same owner, same status, only the project layer differs.
reason() { printf '%s' "$1" | bash "$HOOK" 2>/dev/null | jq -r '.reason // ""' 2>/dev/null; }
notice() { printf '%s' "$1" | bash "$HOOK" 2>/dev/null | jq -r '.systemMessage // ""' 2>/dev/null; }
check_has() {
  case "$2" in
    *"$1"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL want %s in output :: %s :: %s\n' "$1" "$3" "$2" ;;
  esac
}
check_lacks() {
  case "$2" in
    *"$1"*) fail=$((fail + 1)); printf '  FAIL unwanted %s in output :: %s :: %s\n' "$1" "$3" "$2" ;;
    *) pass=$((pass + 1)) ;;
  esac
}

# A LAYER OUTSIDE A REPOSITORY IS NOT A REVIEWED LAYER. Asserted before `git init`,
# because afterwards the same file is merely uncommitted rather than unversioned.
# With no repository there is no committed content at all, so the shipped state
# stands, exactly as it does in a repository with no commits.
mkdir -p .megapowers
if git rev-parse --show-toplevel >/dev/null 2>&1; then
  fail=$((fail + 1)); echo "  FAIL test premise: the fixture directory is already inside a git repository"
else
  pass=$((pass + 1))
fi
printf '[rules.autonomous-run-continuation]\nstate = "off"\n' > .megapowers/enforcement.toml
check BLOCK "$(j s1 false "$TR")" "a worktree layer outside any repository is not honored"

git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'seed\n' > seed.txt
git add seed.txt
git commit -qm init
# ...and inside a repository, an uncommitted layer is still just a pending file.
check BLOCK "$(j s1 false "$TR")" "an uncommitted layer in a fresh repository is not honored"

# COMMITTED, which is the supported opt-out and costs one commit: the price of not
# letting a run write its own exemption.
commit_rules() {
  printf '%s\n' "$1" > .megapowers/enforcement.toml
  git add .megapowers/enforcement.toml
  git commit -qm "rules"
}

commit_rules '[rules.autonomous-run-continuation]
state = "off"'
check ALLOW "$(j s1 false "$TR")" "committed project layer state=off silences the gate"

commit_rules '[rules.autonomous-run-continuation]
state = "advisory"'
check ALLOW "$(j s1 false "$TR")" "committed project layer state=advisory does not block"

commit_rules '[rules.autonomous-run-continuation]
state = "enforced"'
check BLOCK "$(j s1 false "$TR")" "committed project layer state=enforced keeps the gate on"

# A typo is not a licence to ship unreviewed. It reads as off by the consumer
# rule, which is why scripts/check-enforcement.sh rejects unknown values before
# the file ever reaches a session.
commit_rules '[rules.autonomous-run-continuation]
state = "enfoced"'
check ALLOW "$(j s1 false "$TR")" "unknown state reads as off at the consumer"

# A layer that names a different rule says nothing about this one, so the shipped
# state still decides. This is per-key merging, not per-file replacement.
commit_rules '[rules.some-other-rule]
state = "off"'
check BLOCK "$(j s1 false "$TR")" "a layer silencing another rule leaves this one enforced"

# Fail closed. An empty layer contributes no key, so resolution falls through to
# the shipped file, which ships enforced.
commit_rules ''
check BLOCK "$(j s1 false "$TR")" "empty project layer falls through to the shipped state"

git rm -q .megapowers/enforcement.toml
git commit -qm "drop rules"
check BLOCK "$(j s1 false "$TR")" "no project layer leaves the shipped state in force"

# --- a pending policy edit is never honored, and never silent ---
# INVERTED FROM THE OLD WORKTREE-TRUST CASES. Each of these used to be the whole
# opt-out, honored the moment it was written; each is now announced instead.

# The ADDITION with no committed layer: the loosening direction, and the one that
# needs no commit at all to attempt.
printf '[rules.autonomous-run-continuation]\nstate = "off"\n' > .megapowers/enforcement.toml
check BLOCK "$(j s1 false "$TR")" "a pending addition of state=off does not silence the gate"
check_has "pending addition" "$(reason "$(j s1 false "$TR")")" "the block names the pending addition"
check_has "would set state = off" "$(reason "$(j s1 false "$TR")")" "the block names the direction of the edit"

# The EDIT over a committed enforced layer.
commit_rules '[rules.autonomous-run-continuation]
state = "enforced"'
printf '[rules.autonomous-run-continuation]\nstate = "off"\n' > .megapowers/enforcement.toml
check BLOCK "$(j s1 false "$TR")" "a pending edit to state=off does not silence the gate"
check_has "pending edit" "$(reason "$(j s1 false "$TR")")" "the block names the pending edit"

# ...and the same edit STAGED, with the worktree copy restored to its committed
# bytes. `git diff HEAD` alone reads that tree as clean, so the edit sat one plain
# `git commit` from governing with nothing said.
git add .megapowers/enforcement.toml
git show HEAD:.megapowers/enforcement.toml > .megapowers/enforcement.toml
check BLOCK "$(j s1 false "$TR")" "a staged edit masked by a restored worktree copy still blocks"
check_has "would set state = off" "$(reason "$(j s1 false "$TR")")" \
  "the staged edit is read from the index, not from the worktree copy that matches HEAD"
git reset -q HEAD -- .megapowers/enforcement.toml
git checkout -q -- .megapowers/enforcement.toml

# THE TIGHTENING DIRECTION IS ANNOUNCED TOO, and it is the case with no block to
# ride on: the committed layer says off, so the gate does nothing and the notice is
# the only thing that reaches the human.
commit_rules '[rules.autonomous-run-continuation]
state = "off"'
printf '[rules.autonomous-run-continuation]\nstate = "enforced"\n' > .megapowers/enforcement.toml
check ALLOW "$(j s1 false "$TR")" "a pending edit to state=enforced is not honored either"
check_has "Policy notice" "$(notice "$(j s1 false "$TR")")" "the tightening edit is announced on its own"
check_has "would set state = enforced" "$(notice "$(j s1 false "$TR")")" "the notice names the direction"

# A DELETION of that committed off layer would return the repository to the shipped
# enforced default, so it moves this rule and is announced the same way.
rm -f .megapowers/enforcement.toml
check ALLOW "$(j s1 false "$TR")" "a pending deletion is not honored either"
check_has "pending deletion" "$(notice "$(j s1 false "$TR")")" "the deletion is announced"
git checkout -q -- .megapowers/enforcement.toml

# AND SILENCE WHERE THERE IS NOTHING TO SAY. A pending edit that leaves this rule
# where it stood is not this hook's news, and hooks/delegate-nudge.sh already
# announces the file itself at the same stop. Two messages for one fact is the
# noise that gets a notice muted.
commit_rules '[rules.autonomous-run-continuation]
state = "enforced"'
printf '[rules.autonomous-run-continuation]\nstate = "enforced"\n[rules.risky-logic-review]\nstate = "off"\n' > .megapowers/enforcement.toml
check BLOCK "$(j s1 false "$TR")" "a pending edit that does not move this rule still blocks"
check_lacks "pending edit" "$(reason "$(j s1 false "$TR")")" \
  "an edit that leaves this rule alone is not announced by this hook"
git checkout -q -- .megapowers/enforcement.toml

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
