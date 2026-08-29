#!/usr/bin/env bash
# Parity gate: every test script under the three test roots must be referenced
# by scripts/validate.sh, so the validation inventory cannot drift from the tree.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
validator="$ROOT/scripts/validate.sh"

fail() {
  printf 'test inventory: %s\n' "$1" >&2
  exit 1
}

wired="$(grep -hoE '(scripts/tests|evals/tests|evals/studies/tests)/[A-Za-z0-9._-]+\.test\.sh' "$validator" | LC_ALL=C sort -u)"
[ -n "$wired" ] || fail 'validate.sh references no test scripts'

found="$(cd "$ROOT" && find scripts/tests evals/tests evals/studies/tests \
  -type f -name '*.test.sh' | LC_ALL=C sort)"

missing="$(comm -23 <(printf '%s\n' "$found") <(printf '%s\n' "$wired"))"
if [ -n "$missing" ]; then
  printf 'test inventory: test scripts not wired into scripts/validate.sh:\n' >&2
  printf '%s\n' "$missing" | sed 's/^/  /' >&2
  exit 1
fi

printf 'test inventory: ok (%d test scripts, all wired)\n' "$(printf '%s\n' "$found" | wc -l)"
