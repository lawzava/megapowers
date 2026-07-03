#!/usr/bin/env bash
set -u
o="$WORKDIR/eb.out"; [ -f "$o" ] || { echo "no output"; exit 1; }
# reversible: APPROVAL=none + PROCEED=yes at every level (never gated)
[ "$(grep -c '^APPROVAL=none' "$o")" -ge 3 ] || { echo "reversible/staged approval counts wrong"; exit 1; }
# irreversible must require approval at ALL three levels (never auto-fire, even autonomous)
irr_appr=$(awk '/=== irreversible\//{f=1} f&&/^APPROVAL=required/{c++} /=== (reversible|staged|bad)/{f=0} END{print c+0}' "$o")
[ "$irr_appr" -eq 3 ] || { echo "irreversible must require approval at every level (got $irr_appr/3)"; exit 1; }
# irreversible never says PROCEED=yes
awk '/=== irreversible\//{f=1} f&&/^PROCEED=yes/{print "BAD"; exit} /=== (reversible|staged|bad)/{f=0}' "$o" | grep -q BAD && { echo "irreversible allowed to proceed without staging"; exit 1; }
# staged always requires a dry-run
[ "$(grep -c '^DRY_RUN=required' "$o")" -ge 6 ] || { echo "staged/irreversible must always require a dry-run"; exit 1; }
# reversible never requires a dry-run
awk '/=== reversible\//{f=1} f&&/^DRY_RUN=required/{print "BAD"; exit} /=== (staged|irreversible|bad)/{f=0}' "$o" | grep -q BAD && { echo "reversible should not require a dry-run"; exit 1; }
awk '/=== bad ===/{f=1} f&&/rc=2/{r=1} /=== extra/{f=0} END{exit !r}' "$o" || { echo "bad class should exit 2"; exit 1; }
awk '/=== extra-arg ===/{f=1} f&&/unexpected extra argument/{m=1} f&&/rc=2/{r=1} END{exit !(m&&r)}' "$o" || { echo "extra positional arg should be rejected (exit 2)"; exit 1; }
echo "ok: effect-broker protocol correct across classes x levels"
