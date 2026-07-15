#!/usr/bin/env bash
set -u

o="$WORKDIR/out.txt"
[ -f "$o" ] || { echo "missing evidence: out.txt"; exit 1; }

markers='blocked-by blocker-owner unblock-condition expand-migrate-contract context-and-adr-pass source-role-authority conflict-surfaced diagnosis-before-plan memory-not-duplicate-canonical-docs memory-recall-verification-scope'
for marker in $markers; do
  grep -Eq "^(OK|MISSING) ${marker}$" "$o" || {
    echo "missing marker: $marker"
    exit 1
  }
done

if grep -q '^MISSING ' "$o"; then
  grep '^MISSING ' "$o"
  echo "RED: planning-graph-guidance contract incomplete"
  exit 1
fi

echo "ok: planning graph contract is complete"
