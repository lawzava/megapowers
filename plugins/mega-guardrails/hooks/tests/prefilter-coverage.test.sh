#!/usr/bin/env bash
# Prefilter-coverage guard for deny-destructive.sh.
#
# The hook runs a cheap grep prefilter (PREFILTER_TOKENS) before the expensive parser;
# a no-hit fast-ALLOWs at any size. That is only safe if the prefilter HITS every
# command the parser would deny or ask about. This test extracts PREFILTER_TOKENS
# straight from the hook and replays EVERY DENY and ASK fixture from the behavior suite
# (deny-destructive.test.sh) through it, asserting each one hits. If someone adds a new
# deny/ask pattern without extending the prefilter, a fixture stops hitting and this
# fails, catching a fast-allow of a deniable command.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../deny-destructive.sh"
SUITE="$HERE/deny-destructive.test.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

# Pull the prefilter regex from the hook itself (single-line assignment), so the test
# always uses the script's own token union rather than a hand-copied duplicate.
eval "$(grep -E '^PREFILTER_TOKENS=' "$HOOK" || true)"
if [ -z "${PREFILTER_TOKENS:-}" ]; then
  echo "  FAIL PREFILTER_TOKENS not defined in $HOOK"
  echo "== prefilter-coverage: 0 passed, 1 failed =="
  exit 1
fi

pass=0; fail=0; checked=0
while IFS= read -r cmd; do
  [ -n "$cmd" ] || continue
  checked=$((checked + 1))
  if printf '%s' "$cmd" | grep -Eq "$PREFILTER_TOKENS"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL prefilter MISSES a deny/ask fixture :: %s\n' "$cmd"
  fi
done < <(sed -n "s/^check \(DENY\|ASK\) '\([^']*\)'.*/\2/p" "$SUITE")

if [ "$checked" -eq 0 ]; then
  echo "  FAIL no DENY/ASK fixtures found in $SUITE"; fail=$((fail + 1))
fi

echo "== prefilter-coverage: $pass passed, $fail failed ($checked deny/ask fixtures) =="
[ "$fail" -eq 0 ]
