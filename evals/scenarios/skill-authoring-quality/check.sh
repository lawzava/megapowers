#!/usr/bin/env bash
set -u

o="$WORKDIR/out.txt"
[ -f "$o" ] || { echo "missing evidence: out.txt"; exit 1; }

markers='guidance-unit-deletion-no-op hard-dependency optional-enrichment-graceful-degradation observable-leading-vocabulary'
for marker in $markers; do
  grep -Eq "^(OK|MISSING) ${marker}$" "$o" || {
    echo "missing marker: $marker"
    exit 1
  }
done

if grep -q '^MISSING ' "$o"; then
  grep '^MISSING ' "$o"
  echo "RED: skill-authoring-quality contract incomplete"
  exit 1
fi

echo "ok: skill authoring quality contract is complete"
