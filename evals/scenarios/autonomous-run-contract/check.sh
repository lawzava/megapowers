#!/usr/bin/env bash
set -u
o="$WORKDIR/out.txt"; [ -f "$o" ] || { echo "no output"; exit 1; }
for f in charter.md plan.md runbook.md journal.md status; do grep -q "$f" "$o" || { echo "scaffold missing $f"; exit 1; }; done
awk '/=== freeze ===/{f=1} f&&/frozen/{m=1} f&&/rc=3/{r=1} END{exit !(m&&r)}' "$o" || { echo "charter not frozen (second init must exit 3)"; exit 1; }
grep -q "rc_conf2=2" "$o" || { echo "confidence '2' should be rejected (not in [0,1])"; exit 1; }
grep -q "JOURNAL_GREW=4" "$o" || { echo "journal did not append the expected 4 lines"; exit 1; }
grep -q "model=codex" "$o" || { echo "provenance (model) not recorded"; exit 1; }
grep -q "model=file-fallback" "$o" || { echo "persisted model file did not back-fill provenance when env unset"; exit 1; }
grep -qE "^- blocked: 1$" "$o" || { echo "pipe-injection forged a blocked record (blocked count must be 1)"; exit 1; }
# decisions ranked lowest-confidence first: 0.30 line must appear before 0.85 line in the report
c030=$(awk '/## Decisions/{d=1} d&&/conf=0.30/{print NR; exit}' "$o")
c085=$(awk '/## Decisions/{d=1} d&&/conf=0.85/{print NR; exit}' "$o")
[ -n "$c030" ] && [ -n "$c085" ] && [ "$c030" -lt "$c085" ] || { echo "decisions not ranked lowest-confidence first"; exit 1; }
grep -q "Blocked — needs attention" "$o" || { echo "blocked items not surfaced in report"; exit 1; }
grep -q "rc_verify=0" "$o" || { echo "run-verify-status did not pass on a consistent run"; exit 1; }
grep -qE "^LAST_VERIFY=[0-9]{4}-" "$o" || { echo "verify pass did not stamp LAST_VERIFY into status"; exit 1; }

# run-init's "no --model" note must be informational only: exit 0 when --model IS given
grep -q "rc_rc0=0" "$o" || { echo "run-init --model X must exit 0 (informational echo poisoned the exit code)"; exit 1; }

# CURSOR must advance to the first undone plan-declared milestone (M1 done, M2 not)
cursor_section="$(awk '/=== cursor-derive ===/{f=1;next} /=== reopen-init ===/{f=0} f' "$o")"
printf '%s\n' "$cursor_section" | grep -q '^CURSOR=M2' || { echo "CURSOR did not derive to M2 after M1's result"; exit 1; }

# Reopen: a later action entry for an already-done milestone must undo done-ness,
# both for STATE (not done) and CURSOR (back to that milestone)
reopen_section="$(awk '/=== reopen-derive ===/{f=1;next} /=== paused-init ===/{f=0} f' "$o")"
printf '%s\n' "$reopen_section" | grep -q '^STATE=working' || { echo "a later action entry for M2 should reopen it, so STATE must not be done"; exit 1; }
printf '%s\n' "$reopen_section" | grep -q '^CURSOR=M2' || { echo "CURSOR should return to M2 once its result is superseded by a later action"; exit 1; }

# run-report's Activity section must count paused journal entries
paused_section="$(awk '/=== paused-report ===/{f=1} f' "$o")"
printf '%s\n' "$paused_section" | grep -qE '^- paused: 1$' || { echo "run-report Activity section does not count paused entries"; exit 1; }

echo "ok: run contract holds (scaffold, freeze, append-only, provenance, verify-stamp, confidence-ranked report, init exit code, derived cursor, reopen-on-activity, paused activity count)"
