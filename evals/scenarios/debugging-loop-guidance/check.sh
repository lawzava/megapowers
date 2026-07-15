#!/usr/bin/env bash
set -u

o="$WORKDIR/out.txt"
[ -f "$o" ] || { echo "missing evidence: out.txt"; exit 1; }

markers='performance-baseline pre-hypothesis-loop manual-correlation evidence-cost-order minimized-slow-oracle stable-public-seam independent-expected-value probe-cleanup deterministic-test-substitute'
for marker in $markers; do
  grep -Eq "^(OK|MISSING) ${marker}$" "$o" || {
    echo "missing marker: $marker"
    exit 1
  }
done

if grep -q '^MISSING ' "$o"; then
  grep '^MISSING ' "$o"
  echo "RED: debugging-loop-guidance contract incomplete"
  exit 1
fi

echo "ok: debugging loop contract is complete"
