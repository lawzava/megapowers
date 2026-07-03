#!/usr/bin/env bash
S="$ROOT/plugins/mega-orchestration/skills/autonomous-run/scripts"
export MEGAPOWERS_RUN_DIR="$PWD/.megapowers/run"
{
  echo "=== init ==="; "$S/run-init" r1 --level autonomous; echo "rc=$?"
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
  echo "=== report ==="; "$S/run-report" r1
} > out.txt 2>&1
cat out.txt
