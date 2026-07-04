#!/usr/bin/env bash
# Dependency-free test for run-loop.sh. Builds a throwaway run dir, feeds
# Stop-hook JSON on stdin, and asserts BLOCK (loop continues) or ALLOW (stop ok).
# Every BLOCK output is additionally validated as parseable JSON via jq.
# Run: plugins/mega-orchestration/hooks/tests/run-loop.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../run-loop.sh"
SCRIPTS="$HERE/../../skills/autonomous-run/scripts"
TMP="$(mktemp -d)"
cd "$TMP" || exit 1
TR="$TMP/transcript.jsonl"

pass=0; fail=0
verdict() {
  local out; out="$(printf '%s' "$1" | bash "$HOOK" 2>/dev/null)"
  if [ -n "$out" ] && ! printf '%s' "$out" | jq -e . >/dev/null 2>&1; then echo BADJSON; return; fi
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
j() { printf '{"stop_hook_active":%s,"transcript_path":"%s"}' "$1" "$2"; }
check() {
  local got; got="$(verdict "$2")"
  if [ "$1" = "$got" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$got" "$3"; fi
}
mkrun() { # mkrun <id> <state> [level] [cursor]
  mkdir -p ".megapowers/run/$1"
  printf 'STATE=%s\nCURSOR=%s\nLEVEL=%s\nLAST_VERIFY=none\n' "$2" "${4:-M2}" "${3:-on-the-loop}" > ".megapowers/run/$1/status"
}

echo "== run-loop tests =="
: > "$TR"
check ALLOW "$(j false "$TR")" "no run dir at all -> allow"

mkrun r1 working
: > "$TR"
check ALLOW "$(j false "$TR")" "active run never touched this session -> allow (no haunting)"

printf 'tool_result: wrote .megapowers/run/r1/journal.md\n' > "$TR"
check BLOCK "$(j false "$TR")" "active run touched this session (path form) -> block"

# Bare-id mention must NOT count as touched: short ids match prose too easily.
printf 'we discussed r1 briefly in prose\n' > "$TR"
check ALLOW "$(j false "$TR")" "bare id mention without the run path -> allow"

# Path-prefix collision: touching r1-old must not count as touching r1.
printf 'tool_result: wrote .megapowers/run/r1-old/journal.md\n' > "$TR"
check ALLOW "$(j false "$TR")" "sibling run with id prefix (r1-old) does not arm r1"
printf 'tool_result: wrote .megapowers/run/r1/journal.md\n' > "$TR"

check ALLOW "$(j true "$TR")" "stop_hook_active loop guard -> allow"

mkrun r1 blocked
check ALLOW "$(j false "$TR")" "STATE=blocked -> allow (honest stop)"

mkrun r1 paused
check ALLOW "$(j false "$TR")" "STATE=paused -> allow"

mkrun r1 'done'
check ALLOW "$(j false "$TR")" "STATE=done -> allow"

mkrun r1 initialized
check BLOCK "$(j false "$TR")" "STATE=initialized (scaffolded, not started) -> block"

# in-the-loop runs checkpoint with the human at milestone boundaries; the hook
# must not override that gate.
mkrun r1 working in-the-loop
check ALLOW "$(j false "$TR")" "LEVEL=in-the-loop -> allow (human checkpoint wins)"

# CURSOR is agent-written free text; quotes/backslashes must not break the JSON.
mkrun r1 working on-the-loop 'M2 "quoted" \back'
check BLOCK "$(j false "$TR")" "hostile CURSOR still yields valid block JSON"

# The reason must carry the run id and the verify pointer, so the model can act.
mkrun r1 working
out="$(printf '%s' "$(j false "$TR")" | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | jq -re '.reason' 2>/dev/null | grep -q 'r1' \
   && printf '%s' "$out" | jq -re '.reason' 2>/dev/null | grep -q 'run-verify-status'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL :: block reason names the run and the verify path\n'
fi

# Missing transcript file -> fail open.
check ALLOW "$(j false "$TMP/nope.jsonl")" "missing transcript -> allow (fail open)"

# A second, untouched run must not block when the touched one is done.
mkrun r1 'done'
mkrun r2 working
printf 'only .megapowers/run/r1/ mentioned here\n' > "$TR"
check ALLOW "$(j false "$TR")" "other active run not touched this session -> allow"

# Lifecycle integration: after freezing the plan-digest, journaling results for every
# milestone, and re-deriving, the state must be done and the hook must allow the stop.
# A done-claim needs a FROZEN plan (--replan snapshots plan-digest); without one derive
# holds at needs-attention, so the sanctioned finish path freezes before completing.
if [ -x "$SCRIPTS/run-init" ]; then
  ( cd "$TMP" && "$SCRIPTS/run-init" lc1 >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-init" lc1 --replan >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc1 result 0.9 "M1: check passed" >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-derive-status" lc1 >/dev/null 2>&1 )
  st="$(sed -n 's/^STATE=//p' "$TMP/.megapowers/run/lc1/status" | head -1)"
  printf 'touched .megapowers/run/lc1/journal.md\n' > "$TR"
  if [ "$st" = "done" ] && [ "$(verdict "$(j false "$TR")")" = "ALLOW" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL :: sanctioned finish path (journal result -> derive done -> allow), got STATE=%s\n' "$st"
  fi
  # Done must cover the DECLARED plan: journaling only M1 while plan.md
  # declares M2 keeps the run working (and blocked from stopping).
  ( cd "$TMP" && "$SCRIPTS/run-init" lc3 >/dev/null 2>&1 )
  printf '# Plan\n## M1: first\n- acceptance: t1\n## M2: second\n- acceptance: t2\n' > "$TMP/.megapowers/run/lc3/plan.md"
  ( cd "$TMP" && "$SCRIPTS/run-init" lc3 --replan >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc3 result 0.9 "M1: check passed" >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-derive-status" lc3 >/dev/null 2>&1 )
  st3="$(sed -n 's/^STATE=//p' "$TMP/.megapowers/run/lc3/status" | head -1)"
  printf 'touched .megapowers/run/lc3/journal.md\n' > "$TR"
  if [ "$st3" = "working" ] && [ "$(verdict "$(j false "$TR")")" = "BLOCK" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL :: declared-but-unjournaled milestone keeps run working (got STATE=%s)\n' "$st3"
  fi
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc3 result 0.9 "M2: check passed" >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-derive-status" lc3 >/dev/null 2>&1 )
  st3b="$(sed -n 's/^STATE=//p' "$TMP/.megapowers/run/lc3/status" | head -1)"
  if [ "$st3b" = "done" ] && [ "$(verdict "$(j false "$TR")")" = "ALLOW" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL :: completing the declared plan derives done (got STATE=%s)\n' "$st3b"
  fi
  # An untagged result plus a hand-edited done must fail verification.
  ( cd "$TMP" && "$SCRIPTS/run-init" lc4 >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc4 result 0.9 "untagged final success" >/dev/null 2>&1 )
  sed -i 's/^STATE=.*/STATE=done/' "$TMP/.megapowers/run/lc4/status"
  if ( cd "$TMP" && "$SCRIPTS/run-verify-status" lc4 >/dev/null 2>&1 ); then
    fail=$((fail + 1)); printf '  FAIL :: hand-edited done + untagged result must fail run-verify-status\n'
  else
    pass=$((pass + 1))
  fi

  # And a paused entry derives to paused, which also allows.
  ( cd "$TMP" && "$SCRIPTS/run-init" lc2 >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc2 action 0.9 "M1: started" >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-journal" lc2 paused 0.9 "checkpoint for review" >/dev/null 2>&1 )
  ( cd "$TMP" && "$SCRIPTS/run-derive-status" lc2 >/dev/null 2>&1 )
  st2="$(sed -n 's/^STATE=//p' "$TMP/.megapowers/run/lc2/status" | head -1)"
  printf 'touched .megapowers/run/lc2/journal.md\n' > "$TR"
  if [ "$st2" = "paused" ] && [ "$(verdict "$(j false "$TR")")" = "ALLOW" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL :: sanctioned pause path (journal paused -> derive paused -> allow), got STATE=%s\n' "$st2"
  fi
fi

echo "== $pass passed, $fail failed =="
rm -rf "$TMP"
[ "$fail" -eq 0 ]
