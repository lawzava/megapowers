#!/usr/bin/env bash
set -u
o="$WORKDIR/out.txt"; [ -f "$o" ] || { echo "no output"; exit 1; }
for f in charter.md plan.md runbook.md journal.md status; do grep -q "$f" "$o" || { echo "scaffold missing $f"; exit 1; }; done
awk '/=== freeze ===/{f=1} f&&/frozen/{m=1} f&&/rc=3/{r=1} END{exit !(m&&r)}' "$o" || { echo "charter not frozen (second init must exit 3)"; exit 1; }
grep -q "rc_conf2=2" "$o" || { echo "confidence '2' should be rejected (not in [0,1])"; exit 1; }
grep -q "JOURNAL_GREW=4" "$o" || { echo "journal did not append the expected 4 lines"; exit 1; }
grep -q "model=codex" "$o" || { echo "provenance (model) not recorded"; exit 1; }
grep -qE "^- blocked: 1$" "$o" || { echo "pipe-injection forged a blocked record (blocked count must be 1)"; exit 1; }
# decisions ranked lowest-confidence first: 0.30 line must appear before 0.85 line in the report
c030=$(awk '/## Decisions/{d=1} d&&/conf=0.30/{print NR; exit}' "$o")
c085=$(awk '/## Decisions/{d=1} d&&/conf=0.85/{print NR; exit}' "$o")
[ -n "$c030" ] && [ -n "$c085" ] && [ "$c030" -lt "$c085" ] || { echo "decisions not ranked lowest-confidence first"; exit 1; }
grep -q "Blocked — needs attention" "$o" || { echo "blocked items not surfaced in report"; exit 1; }
echo "ok: run contract holds (scaffold, freeze, append-only, provenance, confidence-ranked report)"
