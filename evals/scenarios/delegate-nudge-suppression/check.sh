#!/usr/bin/env bash
set -u
out="$WORKDIR/nudge.out"
[ -f "$out" ] || { echo "no output"; exit 1; }
grep -q "codex_reply=ALLOW"     "$out" || { echo "regressed: codex-reply call did not suppress"; exit 1; }
grep -q "codex_subagent=ALLOW"  "$out" || { echo "regressed: codex subagent_type did not suppress"; exit 1; }
grep -q "prose_only=NAG"        "$out" || { echo "regressed: prose mention false-suppressed the nudge"; exit 1; }
grep -q "notdelegate=NAG"       "$out" || { echo "regressed: 'notdelegate' subagent_type false-suppressed the nudge"; exit 1; }
echo "ok: delegate-nudge suppression correct"
exit 0
