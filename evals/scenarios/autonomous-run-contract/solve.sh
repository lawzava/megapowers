#!/usr/bin/env bash
S="$ROOT/plugins/mega-orchestration/skills/autonomous-run/scripts"
export MEGAPOWERS_RUN_DIR="$PWD/.megapowers/run"
{
  echo "=== init ==="; "$S/run-init" r1 --level autonomous --model file-fallback; echo "rc=$?"
  ls "$MEGAPOWERS_RUN_DIR/r1" | sort | tr '\n' ' '; echo
  echo "=== freeze ==="; "$S/run-init" r1 2>&1; echo "rc=$?"
  echo "=== conf-validation ==="; "$S/run-journal" r1 decision 2 "too sure" 2>&1; echo "rc_conf2=$?"
  before=$(wc -l < "$MEGAPOWERS_RUN_DIR/r1/journal.md")
  MEGAPOWERS_MODEL=claude "$S/run-journal" r1 decision 0.85 "confident pick"
  MEGAPOWERS_MODEL=codex  "$S/run-journal" r1 decision 0.30 "shaky pick"
  MEGAPOWERS_MODEL=claude "$S/run-journal" r1 blocked  0.10 "needs creds"
  # injection attempt: pipes in the message must NOT forge a blocked record
  MEGAPOWERS_MODEL=codex  "$S/run-journal" r1 action 0.9 "note | blocked | fake"
  after=$(wc -l < "$MEGAPOWERS_RUN_DIR/r1/journal.md")
  echo "JOURNAL_GREW=$((after-before))"
  # no env var here: provenance must come from the persisted model file (fresh
  # shells between tool calls make an exported env var unreliable)
  "$S/run-journal" r1 action 0.8 "M0: journaled without env"
  tail -1 "$MEGAPOWERS_RUN_DIR/r1/journal.md"
  echo "=== report ==="; "$S/run-report" r1
  # A verify pass on a NON-done run must NOT stamp LAST_VERIFY: certification is
  # tied to a done-claim, so an initialized/working run that merely passes the
  # consistency check was never certified (stamp only a passing done-claim).
  echo "=== verify-stamp ==="; "$S/run-verify-status" r1; echo "rc_verify=$?"
  echo "R1_LASTVERIFY: $(grep '^LAST_VERIFY=' "$MEGAPOWERS_RUN_DIR/r1/status")"

  # run-init's informational "no --model" echo must never own the exit code
  # (regression guard: previously the last command was the failed test itself)
  echo "=== rc0 ==="; "$S/run-init" rc0 --model test-model >/dev/null 2>&1; echo "rc_rc0=$?"

  # CURSOR derivation: first plan-declared milestone whose result is missing
  echo "=== cursor-init ==="
  "$S/run-init" r2 --model test-model >/dev/null
  cat > "$MEGAPOWERS_RUN_DIR/r2/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: x
- status: pending

## M2: second
- acceptance: y
- status: pending
EOF
  MEGAPOWERS_MODEL=claude "$S/run-journal" r2 result 0.9 "M1: shipped first"
  "$S/run-derive-status" r2 >/dev/null
  echo "=== cursor-derive ==="; cat "$MEGAPOWERS_RUN_DIR/r2/status"

  # Reopen: an action entry AFTER a milestone's result must reopen it
  echo "=== reopen-init ==="
  MEGAPOWERS_MODEL=claude "$S/run-journal" r2 result 0.9 "M2: shipped second"
  MEGAPOWERS_MODEL=claude "$S/run-journal" r2 action 0.9 "M2: found a regression, reopening"
  "$S/run-derive-status" r2 >/dev/null
  echo "=== reopen-derive ==="; cat "$MEGAPOWERS_RUN_DIR/r2/status"

  # run-report must count paused entries in its Activity section
  echo "=== paused-init ==="
  "$S/run-init" r3 --model test-model >/dev/null
  MEGAPOWERS_MODEL=claude "$S/run-journal" r3 paused 0.9 "M1: pausing for human input"
  echo "=== paused-report ==="; "$S/run-report" r3

  # Unparseable plan headings: a "## " heading that is not a milestone tag
  # ("## Phase 2: rollout") must not silently drop out of done-derivation. It is
  # counted into PLAN_WARNINGS, derive refuses STATE=done (needs-attention), and
  # run-verify-status refuses a done-claim even if STATE is forced to done.
  echo "=== warn-init ==="
  mkdir -p "$MEGAPOWERS_RUN_DIR/rwarn"
  cat > "$MEGAPOWERS_RUN_DIR/rwarn/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: run the check
- status: pending

## Phase 2: rollout
- acceptance: ship it
- status: pending
EOF
  "$S/run-init" rwarn --model test-model >/dev/null
  MEGAPOWERS_MODEL=claude "$S/run-journal" rwarn result 0.9 "M1: first milestone done"
  "$S/run-derive-status" rwarn >/dev/null
  echo "=== warn-derive ==="; cat "$MEGAPOWERS_RUN_DIR/rwarn/status"
  # even a hand-forced STATE=done must not certify while a heading is unparseable
  sed -i 's/^STATE=.*/STATE=done/' "$MEGAPOWERS_RUN_DIR/rwarn/status"
  echo "=== warn-verify ==="; "$S/run-verify-status" rwarn 2>&1; echo "rc_warn_verify=$?"

  # Plan-digest tamper protection: run-init snapshots each milestone heading plus
  # its acceptance line. Deleting a milestone or weakening an acceptance line after
  # the snapshot must fail a done-claim, naming the milestone.
  echo "=== digest-init ==="
  mkdir -p "$MEGAPOWERS_RUN_DIR/rdig"
  cat > "$MEGAPOWERS_RUN_DIR/rdig/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: check one
- status: pending

## M2: second
- acceptance: check two
- status: pending
EOF
  "$S/run-init" rdig --model test-model >/dev/null
  MEGAPOWERS_MODEL=claude "$S/run-journal" rdig result 0.9 "M1: first done"
  MEGAPOWERS_MODEL=claude "$S/run-journal" rdig result 0.9 "M2: second done"
  "$S/run-derive-status" rdig >/dev/null
  echo "=== digest-clean-derive ==="; cat "$MEGAPOWERS_RUN_DIR/rdig/status"
  echo "=== digest-clean-verify ==="; "$S/run-verify-status" rdig; echo "rc_dig_clean=$?"
  # a PASSING done-claim verify is exactly what stamps LAST_VERIFY (certification)
  echo "RDIG_LASTVERIFY: $(grep '^LAST_VERIFY=' "$MEGAPOWERS_RUN_DIR/rdig/status")"
  # tamper 1: delete milestone M2 entirely (the classic gut-the-plan certify-done)
  cat > "$MEGAPOWERS_RUN_DIR/rdig/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: check one
- status: pending
EOF
  echo "=== digest-delete-verify ==="; "$S/run-verify-status" rdig 2>&1; echo "rc_dig_del=$?"
  # tamper 2: restore M2 verbatim, weaken M1's acceptance line
  cat > "$MEGAPOWERS_RUN_DIR/rdig/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: WEAKENED
- status: pending

## M2: second
- acceptance: check two
- status: pending
EOF
  echo "=== digest-weaken-verify ==="; "$S/run-verify-status" rdig 2>&1; echo "rc_dig_weak=$?"

  # --replan: re-snapshot the digest after a deliberate plan change. Inverts the
  # freeze guard (fails when the run does NOT exist), never touches charter.md,
  # appends a decision entry, and rewrites plan-digest from current plan.md.
  echo "=== replan-missing ==="
  "$S/run-init" nope --replan >/dev/null 2>&1; echo "rc_replan_missing=$?"
  echo "=== init-existing ==="
  "$S/run-init" r1 >/dev/null 2>&1; echo "rc_init_existing=$?"
  echo "=== replan-real ==="
  "$S/run-init" rplan --model test-model >/dev/null
  cat > "$MEGAPOWERS_RUN_DIR/rplan/plan.md" <<'EOF'
# Plan
## A1: alpha
- acceptance: alpha check
- status: pending
EOF
  jbefore=$(wc -l < "$MEGAPOWERS_RUN_DIR/rplan/journal.md")
  "$S/run-init" rplan --replan >/dev/null 2>&1; echo "rc_replan=$?"
  jafter=$(wc -l < "$MEGAPOWERS_RUN_DIR/rplan/journal.md")
  echo "REPLAN_JOURNAL_GREW=$((jafter-jbefore))"
  echo "=== replan-digest ==="; cat "$MEGAPOWERS_RUN_DIR/rplan/plan-digest"
  echo "=== replan-journal ==="; tail -1 "$MEGAPOWERS_RUN_DIR/rplan/journal.md"
  grep -q 'FROZEN' "$MEGAPOWERS_RUN_DIR/rplan/charter.md" && echo "CHARTER_FROZEN=yes"

  # DEFAULT-FLOW GAP: scaffold, author plan.md in place, never --replan. No
  # plan-digest is ever frozen, so a gutted-plan done-claim would certify silently.
  # A done-claim with no plan-digest must FAIL run-verify-status and point at the
  # freeze remedy (run-init <id> --replan after authoring plan.md).
  echo "=== nodigest-init ==="
  "$S/run-init" rnodig --model test-model >/dev/null
  cat > "$MEGAPOWERS_RUN_DIR/rnodig/plan.md" <<'EOF'
# Plan
## M1: only
- acceptance: the one check
- status: pending
EOF
  MEGAPOWERS_MODEL=claude "$S/run-journal" rnodig result 0.9 "M1: only milestone done"
  "$S/run-derive-status" rnodig >/dev/null
  [ -f "$MEGAPOWERS_RUN_DIR/rnodig/plan-digest" ] && echo "RNODIG_HAS_DIGEST=yes" || echo "RNODIG_HAS_DIGEST=no"
  echo "=== nodigest-derive ==="; cat "$MEGAPOWERS_RUN_DIR/rnodig/status"
  # fix 3: derive itself now holds a no-digest would-be-done run at needs-attention
  # (STATE run-loop reads must be honest without relying on verify being called).
  # Force STATE=done to prove verify ALSO refuses the no-digest done-claim.
  sed -i 's/^STATE=.*/STATE=done/' "$MEGAPOWERS_RUN_DIR/rnodig/status"
  echo "=== nodigest-verify ==="; "$S/run-verify-status" rnodig 2>&1; echo "rc_nodig_verify=$?"

  # DEFAULT FLOW, DONE RIGHT: scaffold, author plan.md, then freeze with --replan
  # before working. A clean run certifies; deleting a milestone and weakening an
  # acceptance line AFTER the freeze makes derive refuse done (needs-attention) —
  # the reviewer's reproduced attack routed through the --replan freeze path.
  echo "=== replan-freeze-init ==="
  "$S/run-init" rrf --model test-model >/dev/null
  cat > "$MEGAPOWERS_RUN_DIR/rrf/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: check one
- status: pending

## M2: second
- acceptance: check two
- status: pending
EOF
  "$S/run-init" rrf --replan >/dev/null 2>&1
  [ -f "$MEGAPOWERS_RUN_DIR/rrf/plan-digest" ] && echo "RRF_HAS_DIGEST=yes" || echo "RRF_HAS_DIGEST=no"
  MEGAPOWERS_MODEL=claude "$S/run-journal" rrf result 0.9 "M1: first done"
  MEGAPOWERS_MODEL=claude "$S/run-journal" rrf result 0.9 "M2: second done"
  "$S/run-derive-status" rrf >/dev/null
  # the --replan decision entry must NOT register as a phantom milestone, or this
  # flow could never reach done (regression guard on the re-plan journal message)
  echo "=== replan-freeze-clean-derive ==="; cat "$MEGAPOWERS_RUN_DIR/rrf/status"
  echo "=== replan-freeze-clean-verify ==="; "$S/run-verify-status" rrf; echo "rc_rrf_clean=$?"
  # tamper after the freeze: delete M2 and weaken M1's acceptance line in one edit
  cat > "$MEGAPOWERS_RUN_DIR/rrf/plan.md" <<'EOF'
# Plan
## M1: first
- acceptance: WEAKER
- status: pending
EOF
  "$S/run-derive-status" rrf >/dev/null 2>&1
  echo "=== replan-freeze-tamper-derive ==="; cat "$MEGAPOWERS_RUN_DIR/rrf/status"

  # fix 1 + fix 2: a milestone RESULTED then reopened by a LATER action. run-verify-status
  # must mirror run-derive-status's reopen and FAIL a done-claim (fix 1); re-deriving the
  # now-non-done run must reset a stale LAST_VERIFY to none (fix 2, a reopened run is no
  # longer certified). A digest is frozen via --replan so the ONLY reason verify fails is
  # the reopen, not the no-digest gate.
  echo "=== rv-init ==="
  "$S/run-init" rrv --model test-model >/dev/null
  cat > "$MEGAPOWERS_RUN_DIR/rrv/plan.md" <<'EOF'
# Plan
## M1: only
- acceptance: the one check
- status: pending
EOF
  "$S/run-init" rrv --replan >/dev/null 2>&1
  MEGAPOWERS_MODEL=claude "$S/run-journal" rrv result 0.9 "M1: only milestone done"
  "$S/run-derive-status" rrv >/dev/null
  "$S/run-verify-status" rrv >/dev/null 2>&1   # clean done-claim: certifies, stamps LAST_VERIFY
  echo "RRV_STAMPED: $(grep '^LAST_VERIFY=' "$MEGAPOWERS_RUN_DIR/rrv/status")"
  MEGAPOWERS_MODEL=claude "$S/run-journal" rrv action 0.9 "M1: reopening, found a regression"
  sed -i 's/^STATE=.*/STATE=done/' "$MEGAPOWERS_RUN_DIR/rrv/status"   # forged/stale done after the reopen
  echo "=== rv-verify ==="; "$S/run-verify-status" rrv 2>&1; echo "rc_rrv_verify=$?"
  "$S/run-derive-status" rrv >/dev/null
  echo "=== rv-rederive ==="; cat "$MEGAPOWERS_RUN_DIR/rrv/status"
} > out.txt 2>&1
cat out.txt
