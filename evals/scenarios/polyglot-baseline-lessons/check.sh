#!/usr/bin/env bash
set -u
o="$WORKDIR/lint.out"; [ -f "$o" ] || { echo "no output"; exit 1; }
if grep -q '^BAD ' "$o"; then echo "missing lesson(s):"; grep '^BAD ' "$o"; exit 1; fi
n=$(grep -c '^OK ' "$o"); [ "$n" -ge 10 ] || { echo "expected >=10 lesson checks, ran $n"; exit 1; }
echo "ok: all $n polyglot baseline lessons present"
