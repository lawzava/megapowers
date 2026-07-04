#!/usr/bin/env bash
set -u
out="$WORKDIR/nudge.out"
[ -f "$out" ] || { echo "no output"; exit 1; }
grep -q "codex_reply=ALLOW"            "$out" || { echo "regressed: codex-reply call did not suppress"; exit 1; }
grep -q "codex_subagent=ALLOW"         "$out" || { echo "regressed: codex subagent_type did not suppress"; exit 1; }
grep -q "prose_only=NAG"               "$out" || { echo "regressed: prose mention false-suppressed the nudge"; exit 1; }
grep -q "notdelegate=NAG"              "$out" || { echo "regressed: 'notdelegate' subagent_type false-suppressed the nudge"; exit 1; }
grep -q "first_stop=NAG"               "$out" || { echo "regressed: first stop on a risky diff did not nudge"; exit 1; }
grep -q "second_stop_same_diff=ALLOW"  "$out" || { echo "regressed: once-per-diff-state fix broken - re-nagged on an unchanged risky diff"; exit 1; }
grep -q "third_stop_diff_changed=NAG"  "$out" || { echo "regressed: hook failed to re-nag once the risky diff actually changed"; exit 1; }
echo "ok: delegate-nudge suppression correct"
exit 0
