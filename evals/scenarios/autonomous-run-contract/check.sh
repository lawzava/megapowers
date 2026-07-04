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
# fix 4: a verify pass on a NON-done run must NOT stamp LAST_VERIFY (certification
# is tied to a done-claim; r1 is initialized, so LAST_VERIFY stays none)
grep -q "R1_LASTVERIFY: LAST_VERIFY=none" "$o" || { echo "non-done verify must NOT stamp LAST_VERIFY (only a passing done-claim certifies)"; exit 1; }

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

# Unparseable "## " heading: PLAN_WARNINGS must count it, derive must refuse done,
# and run-verify-status must refuse the done-claim even when STATE is forced to done.
warn_derive="$(awk '/=== warn-derive ===/{f=1;next} /=== warn-verify ===/{f=0} f' "$o")"
printf '%s\n' "$warn_derive" | grep -q '^PLAN_WARNINGS=1' || { echo "unparseable heading not counted (PLAN_WARNINGS=1 expected)"; exit 1; }
printf '%s\n' "$warn_derive" | grep -q '^STATE=done' && { echo "derive must NOT reach STATE=done with an unparseable heading"; exit 1; }
printf '%s\n' "$warn_derive" | grep -q '^STATE=needs-attention' || { echo "derive should mark STATE=needs-attention under a plan warning"; exit 1; }
grep -q "rc_warn_verify=1" "$o" || { echo "run-verify-status must fail a done-claim while PLAN_WARNINGS != 0"; exit 1; }

# Plan-digest: a clean run certifies; deleting a milestone or weakening an
# acceptance line fails the done-claim, naming the affected milestone.
grep -q "rc_dig_clean=0" "$o" || { echo "clean run with matching plan-digest must certify (rc 0)"; exit 1; }
# fix 4: a PASSING done-claim verify IS what stamps LAST_VERIFY (the certification marker)
grep -qE "RDIG_LASTVERIFY: LAST_VERIFY=[0-9]{4}-" "$o" || { echo "a passing done-claim verify must stamp LAST_VERIFY into status"; exit 1; }
del_section="$(awk '/=== digest-delete-verify ===/{f=1;next} /=== digest-weaken-verify ===/{f=0} f' "$o")"
printf '%s\n' "$del_section" | grep -q 'rc_dig_del=1' || { echo "deleting a declared milestone must fail run-verify-status"; exit 1; }
printf '%s\n' "$del_section" | grep -q 'M2' || { echo "digest failure must name the missing milestone (M2)"; exit 1; }
weak_section="$(awk '/=== digest-weaken-verify ===/{f=1;next} /=== replan-missing ===/{f=0} f' "$o")"
printf '%s\n' "$weak_section" | grep -q 'rc_dig_weak=1' || { echo "weakening an acceptance line must fail run-verify-status"; exit 1; }
printf '%s\n' "$weak_section" | grep -q 'M1' || { echo "digest failure must name the changed milestone (M1)"; exit 1; }

# --replan: fails when the run does not exist; normal init on an existing run also
# fails; a real replan re-snapshots the digest, journals a decision, keeps charter.
grep -q "rc_replan_missing=3" "$o" || { echo "--replan on a missing run must exit 3"; exit 1; }
grep -q "rc_init_existing=3" "$o" || { echo "normal init on an existing (frozen) run must exit 3"; exit 1; }
grep -q "rc_replan=0" "$o" || { echo "--replan on an existing run must succeed (rc 0)"; exit 1; }
grep -q "REPLAN_JOURNAL_GREW=1" "$o" || { echo "--replan must append exactly one journal entry"; exit 1; }
replan_j="$(awk '/=== replan-journal ===/{f=1;next} /CHARTER_FROZEN=/{f=0} f' "$o")"
printf '%s\n' "$replan_j" | grep -qE '\| decision \|.*re-plan' || { echo "--replan must append a decision entry recording the re-plan"; exit 1; }
replan_d="$(awk '/=== replan-digest ===/{f=1;next} /=== replan-journal ===/{f=0} f' "$o")"
printf '%s\n' "$replan_d" | grep -q '^A1 ' || { echo "--replan must rewrite plan-digest from current plan.md (milestone A1 expected)"; exit 1; }
grep -q "CHARTER_FROZEN=yes" "$o" || { echo "--replan must leave charter.md frozen (untouched)"; exit 1; }

# fix 1: the DEFAULT-flow gap. Scaffold, author plan.md in place, never --replan =>
# no plan-digest is ever frozen. derive still reaches done (verify is the gate), but
# a done-claim with NO plan-digest must FAIL run-verify-status, naming the freeze remedy.
grep -q "RNODIG_HAS_DIGEST=no" "$o" || { echo "author-then-never-replan flow must leave NO plan-digest (default-flow gap precondition)"; exit 1; }
nodig_derive="$(awk '/=== nodigest-derive ===/{f=1;next} /=== nodigest-verify ===/{f=0} f' "$o")"
printf '%s\n' "$nodig_derive" | grep -q '^STATE=done' || { echo "derive should still reach STATE=done without a digest (verify, not derive, is the no-digest gate)"; exit 1; }
nodig_verify="$(awk '/=== nodigest-verify ===/{f=1;next} /=== replan-freeze-init ===/{f=0} f' "$o")"
printf '%s\n' "$nodig_verify" | grep -q 'rc_nodig_verify=1' || { echo "a done-claim with no plan-digest must FAIL run-verify-status (gutted plan would otherwise certify silently)"; exit 1; }
printf '%s\n' "$nodig_verify" | grep -qi 'never frozen' || { echo "no-digest verify failure must say the plan was never frozen"; exit 1; }
printf '%s\n' "$nodig_verify" | grep -q -- '--replan' || { echo "no-digest verify failure must point at run-init <id> --replan to freeze"; exit 1; }

# fix 5 regression: default flow done RIGHT via the --replan freeze, then the
# reviewer's reproduced attack. Freezing via --replan gives a digest, so the clean
# run certifies; deleting a milestone and weakening an acceptance line after the
# freeze makes derive refuse done (STATE=needs-attention).
grep -q "RRF_HAS_DIGEST=yes" "$o" || { echo "--replan on the authored plan must freeze a plan-digest (default-flow freeze path)"; exit 1; }
# the --replan decision entry must not register as a phantom milestone: the clean
# flow (author -> --replan -> work -> derive) must actually reach STATE=done
rrf_clean="$(awk '/=== replan-freeze-clean-derive ===/{f=1;next} /=== replan-freeze-clean-verify ===/{f=0} f' "$o")"
printf '%s\n' "$rrf_clean" | grep -q '^STATE=done' || { echo "--replan then completing every milestone must derive STATE=done (the re-plan journal entry must not become a phantom milestone)"; exit 1; }
grep -q "rc_rrf_clean=0" "$o" || { echo "a run frozen via --replan must certify clean (rc 0)"; exit 1; }
rrf_tamper="$(awk '/=== replan-freeze-tamper-derive ===/{f=1} f' "$o")"
printf '%s\n' "$rrf_tamper" | grep -q '^STATE=needs-attention' || { echo "deleting a milestone + weakening acceptance after a --replan freeze must make derive refuse done (needs-attention)"; exit 1; }

echo "ok: run contract holds (scaffold, freeze, append-only, provenance, done-claim verify-stamp only, confidence-ranked report, init exit code, derived cursor, reopen-on-activity, paused activity count, plan-warnings, plan-digest tamper, no-digest done-claim refused, replan-freeze regression, replan)"
