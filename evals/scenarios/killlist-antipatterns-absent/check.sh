#!/usr/bin/env bash
set -u
out="$WORKDIR/lint.out"
[ -f "$out" ] || { echo "no lint output"; exit 1; }
if grep -q '^BAD ' "$out"; then
  echo "anti-pattern regression(s):"; grep '^BAD ' "$out"; exit 1
fi
n="$(grep -c '^OK ' "$out")"
[ "$n" -ge 6 ] || { echo "expected >=6 lint checks, ran $n"; exit 1; }
echo "ok: all $n kill-list lints clean"
exit 0
