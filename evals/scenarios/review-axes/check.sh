#!/usr/bin/env bash
set -u

o="$WORKDIR/out.txt"
[ -f "$o" ] || { echo "missing evidence: out.txt"; exit 1; }

markers='separate-axis-headings specification-axis-severities engineering-axis-severities findings-not-merged-or-reranked ready-to-merge-preserved specification-fail-blocks-downstream'
for marker in $markers; do
  grep -Eq "^(OK|MISSING) ${marker}$" "$o" || {
    echo "missing marker: $marker"
    exit 1
  }
done

if grep -q '^MISSING ' "$o"; then
  grep '^MISSING ' "$o"
  echo "RED: review-axes contract incomplete"
  exit 1
fi

echo "ok: review axes contract is complete"
