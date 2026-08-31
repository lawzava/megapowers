#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MAP="$ROOT/verification/megapowers.json"
INSTALL_RUNNER="$ROOT/evals/studies/install-smoke/run-smoke.sh"

fail() {
  printf 'verification map contract: %s\n' "$*" >&2
  exit 1
}

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

for journey in claude-exact-tag-install codex-exact-tag-install; do
  mapfile -t runner < <(jq -er --arg id "$journey" '.journeys[] | select(.id == $id) | .runner[]' "$MAP")
  [[ ${runner[0]} == bash && ${runner[1]} == evals/studies/install-smoke/run-smoke.sh ]] ||
    fail "$journey does not use the install-smoke runner"
  printf '%s\n' "${runner[@]}" | grep -qx -- '--out' || fail "$journey omits --out"
  printf '%s\n' "${runner[@]}" | grep -qx -- '--harnesses' || fail "$journey omits --harnesses"
  printf '%s\n' "${runner[@]}" | grep -qx -- '--source' || fail "$journey omits --source"
  printf '%s\n' "${runner[@]}" | grep -qx -- '--ref' || fail "$journey omits --ref"
  if printf '%s\n' "${runner[@]}" | grep -qx -- '--harness'; then
    fail "$journey uses unsupported --harness"
  fi
  while IFS= read -r flag; do
    grep -qF -- "$flag)" "$INSTALL_RUNNER" || fail "$journey uses unknown runner flag $flag"
  done < <(printf '%s\n' "${runner[@]}" | grep '^--' | sort -u)
done

grep -qF 'map drift' "$ROOT/docs/advanced/verification-maps.md"
grep -qF 'does not generate' "$ROOT/docs/advanced/verification-maps.md"

echo 'verification map contract: ok'
