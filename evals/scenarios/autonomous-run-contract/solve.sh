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
  # verify pass must stamp LAST_VERIFY (a done-claim with LAST_VERIFY=none was
  # never certified — regression guard for the stamp added after the live probe)
  echo "=== verify-stamp ==="; "$S/run-verify-status" r1; echo "rc_verify=$?"
  grep '^LAST_VERIFY=' "$MEGAPOWERS_RUN_DIR/r1/status"

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
} > out.txt 2>&1
cat out.txt
