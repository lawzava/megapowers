#!/usr/bin/env bash
set -u
out="$WORKDIR/polluter.out"
[ -f "$out" ] || { echo "no output"; exit 1; }

# both the glob-terminal and literal-terminal patterns must find their 2 files
[ "$(grep -c "Found 2 test files" "$out")" -ge 2 ] || { echo "a matching pattern did not find both tests (glob or nested-literal)"; exit 1; }
grep -q "rc_match=0"          "$out" || { echo "matching glob should exit 0 (all clean, npm no-op)"; exit 1; }
grep -q "rc_lit=0"            "$out" || { echo "matching literal-terminal glob should exit 0"; exit 1; }
grep -q "No test files matched" "$out" || { echo "non-matching glob should error loudly"; exit 1; }
grep -q "rc_nomatch=2"        "$out" || { echo "non-matching glob should exit 2, not a false clean"; exit 1; }
# the false-negative we fixed: 'all tests clean' must NOT appear for the zero-match run
awk '/=== NON-MATCHING GLOB ===/{f=1} f && /No polluter found - all tests clean/{print "FALSE CLEAN"; exit 3}' "$out" | grep -q "FALSE CLEAN" && { echo "regressed: zero-match reported clean"; exit 1; }
echo "ok: find-polluter bounded and honest"
exit 0
