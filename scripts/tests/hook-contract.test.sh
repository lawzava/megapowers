#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
guard="$ROOT/plugins/megapowers/hooks/deny-destructive.sh"
wrapper="$ROOT/plugins/megapowers/hooks/run-hook.cmd"

fail() {
  printf 'hook contract: %s\n' "$*" >&2
  exit 1
}

if rg -ni 'effect-broker|ask[ -]tier|decision=.*ask|permissionDecision[^[:alnum:]]*:[^[:alnum:]]*ask' "$guard"; then
  fail 'destructive guard retains a dormant confirmation tier'
fi

set +e
missing_out="$(bash "$wrapper" missing-hook.sh 2>&1)"
missing_rc=$?
set -e
[[ $missing_rc -ne 0 ]] || fail 'Unix wrapper silently accepted a missing hook target'
[[ $missing_out == *'cannot run'* ]] || fail 'Unix wrapper did not explain missing hook target'

printf 'hook contract: ok\n'
