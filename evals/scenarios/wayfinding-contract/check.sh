#!/usr/bin/env bash
set -u

o="$WORKDIR/out.txt"
[ -f "$o" ] || { echo "missing evidence: out.txt"; exit 1; }

markers='wayfinding-skill-exists local-map-contract decision-file-contract fog-and-current-frontier source-owner-decision-evidence-dependency one-decision-loop tracker-optional no-automatic-commit-or-execution stop-spec-ready-or-blocked plan-ready-requires-approved-design codex-sidecar-exists implicit-invocation-disabled default-prompt-names-wayfinding codex-metadata-validator-exists valid-sidecar-accepted drifted-default-prompt-rejected drifted-implicit-policy-rejected invalid-implicit-boolean-rejected quoted-implicit-boolean-rejected invalid-short-description-rejected missing-required-interface-field-rejected official-optional-metadata-accepted orchestrating-route-exists readme-wayfinding-entry-exists brainstorming-wayfinding-boundary-exists integration-removal-mutations-rejected'
for marker in $markers; do
  grep -Eq "^(OK|MISSING) ${marker}$" "$o" || {
    echo "missing marker: $marker"
    exit 1
  }
done

if grep -q '^MISSING ' "$o"; then
  grep '^MISSING ' "$o"
  echo "RED: wayfinding-contract incomplete"
  exit 1
fi

echo "ok: wayfinding contract is complete"
