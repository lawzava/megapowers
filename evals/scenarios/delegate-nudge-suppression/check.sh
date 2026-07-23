#!/usr/bin/env bash
set -u

out="$WORKDIR/nudge.out"
[ -f "$out" ] || { echo "no output"; exit 1; }
grep -q "delegate_call_without_receipt=NAG" "$out" || { echo "delegate invocation falsely counted as review proof"; exit 1; }
grep -q "repeated_without_receipt=NAG" "$out" || { echo "unchanged risky diff escaped without a receipt"; exit 1; }
grep -q "current_approval=ALLOW" "$out" || { echo "current independent approval receipt did not allow"; exit 1; }
grep -q "stale_approval=NAG" "$out" || { echo "stale receipt allowed a changed tree"; exit 1; }
grep -q "needs_attention=NAG" "$out" || { echo "needs-attention receipt allowed completion"; exit 1; }
echo "ok: delegate-nudge receipt gate correct"
