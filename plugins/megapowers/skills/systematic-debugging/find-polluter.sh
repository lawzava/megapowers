#!/usr/bin/env bash
# Bisection script to find which test creates unwanted files/state
# Usage: ./find-polluter.sh <file_or_dir_to_check> <test_pattern>
# Example: ./find-polluter.sh '.git' 'src/**/*.test.ts'

set -e

if [ $# -ne 2 ]; then
  echo "Usage: $0 <file_to_check> <test_pattern>"
  echo "Example: $0 '.git' 'src/**/*.test.ts'"
  exit 1
fi

POLLUTION_CHECK="$1"
TEST_PATTERN="$2"

echo "Searching for test that creates: $POLLUTION_CHECK"
echo "Test pattern: $TEST_PATTERN"
echo ""

# Get list of test files by expanding the caller's glob directly with bash
# `globstar`, so `**` matches zero OR more directories — both 'src/foo.test.ts'
# and 'src/a/b/foo.test.ts' for 'src/**/*.test.ts', and both direct and nested
# hits for a literal terminal like 'tests/**/foo.test.ts'. (find's -path cannot
# express "zero or more path segments", which under-matched literal terminals.)
# Requires bash 4+ for globstar.
shopt -s globstar nullglob 2>/dev/null
# intentional: $TEST_PATTERN must glob-expand (globstar) into the matches array
# shellcheck disable=SC2206
matches=( $TEST_PATTERN )
shopt -u globstar nullglob 2>/dev/null
if [ ${#matches[@]} -eq 0 ]; then
  FILES=()
else
  mapfile -t FILES < <(printf '%s\n' "${matches[@]}" | sort)
fi
TOTAL=${#FILES[@]}

if [ "$TOTAL" -eq 0 ]; then
  echo "No test files matched: $TEST_PATTERN" >&2
  echo "(expanded the glob with globstar; needs bash 4+) — check the pattern; refusing to report 'clean' on zero tests." >&2
  exit 2
fi

echo "Found $TOTAL test files"
echo ""

COUNT=0
for TEST_FILE in "${FILES[@]}"; do
  COUNT=$((COUNT + 1))

  # Skip if pollution already exists
  if [ -e "$POLLUTION_CHECK" ]; then
    echo "Pollution already exists before test $COUNT/$TOTAL"
    echo "   Skipping: $TEST_FILE"
    continue
  fi

  echo "[$COUNT/$TOTAL] Testing: $TEST_FILE"

  # Run the test
  npm test "$TEST_FILE" > /dev/null 2>&1 || true

  # Check if pollution appeared
  if [ -e "$POLLUTION_CHECK" ]; then
    echo ""
    echo "Found polluter"
    echo "   Test: $TEST_FILE"
    echo "   Created: $POLLUTION_CHECK"
    echo ""
    echo "Pollution details:"
    ls -la "$POLLUTION_CHECK"
    echo ""
    echo "To investigate:"
    echo "  npm test $TEST_FILE    # Run just this test"
    echo "  cat $TEST_FILE         # Review test code"
    exit 1
  fi
done

echo ""
echo "No polluter found - all tests clean"
exit 0
