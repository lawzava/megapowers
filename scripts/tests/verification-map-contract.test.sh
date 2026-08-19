#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MAP="$ROOT/verification/megapowers.json"

jq -e '
  .schema_version == "1" and
  .application == "megapowers" and
  .status == "pilot" and
  (.journeys | length >= 3) and
  ([.journeys[].id] | length) == ([.journeys[].id] | unique | length) and
  ([.journeys[] | select(
    (.id | type == "string" and length > 0) and
    (.surface | type == "string" and length > 0) and
    (.harness == "local" or .harness == "claude" or .harness == "codex") and
    (.doctor | type == "array" and length > 0) and
    (.runner | type == "array" and length > 0) and
    (.isolated_state | type == "string" and length > 0) and
    (.cleanup | type == "string" and length > 0) and
    (.evidence | type == "array" and length > 0)
  )] | length) == (.journeys | length)
' "$MAP" >/dev/null

if rg -ni '(/home/|credentials|auth[.]json|secret|customer)' "$MAP"; then
  echo 'FAIL verification map contains private or credential state' >&2
  exit 1
fi

grep -qF 'map drift' "$ROOT/docs/advanced/verification-maps.md"
grep -qF 'does not generate' "$ROOT/docs/advanced/verification-maps.md"

echo 'verification map contract: ok'
