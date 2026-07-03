#!/usr/bin/env bash
set -u
o="$WORKDIR/inv.out"; [ -f "$o" ] || { echo "no output"; exit 1; }
if grep -q '^BAD ' "$o"; then echo "invariant(s) missing:"; grep '^BAD ' "$o"; exit 1; fi
n="$(grep -c '^OK ' "$o")"; [ "$n" -ge 11 ] || { echo "expected >=11 invariant checks, ran $n"; exit 1; }
echo "ok: all $n swarm-primitive invariants present"
