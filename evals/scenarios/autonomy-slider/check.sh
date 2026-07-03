#!/usr/bin/env bash
set -u
o="$WORKDIR/out.txt"; [ -f "$o" ] || { echo "no output"; exit 1; }
# reversible work must proceed at EVERY level (no false gate on reversible work)
[ "$(grep -c '^REVERSIBLE=proceed' "$o")" -eq 3 ] || { echo "reversible work is gated at some level (must never be)"; exit 1; }
# irreversible must be gated at every level (never 'proceed')
grep -q '^IRREVERSIBLE=proceed' "$o" && { echo "irreversible action allowed to proceed unguarded"; exit 1; }
grep -q 'LEVEL=autonomous'  "$o" && grep -q 'IRREVERSIBLE=stage-for-approval' "$o" || { echo "autonomous should stage irreversible"; exit 1; }
awk '/=== in-the-loop ===/{f=1} f&&/STAGED=pause-for-approval/{p=1} END{exit !p}' "$o" || { echo "in-the-loop should pause on staged actions"; exit 1; }
awk '/=== bad ===/{f=1} f&&/rc=2/{r=1} END{exit !r}' "$o" || { echo "bad level should exit 2"; exit 1; }
echo "ok: autonomy dial correct (reversible never gated; irreversible always gated)"
