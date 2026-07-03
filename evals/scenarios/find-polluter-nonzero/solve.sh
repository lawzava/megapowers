#!/usr/bin/env bash
# Run the shipped find-polluter twice: once with a matching glob (should find 2
# files, report clean because npm creates nothing), once with a non-matching glob
# (should error loudly, never 'clean').
export PATH="$PWD/bin:$PATH"
fp="$ROOT/plugins/megapowers/skills/systematic-debugging/find-polluter.sh"
{
  echo "=== MATCHING GLOB (glob terminal, direct + nested) ==="
  bash "$fp" '.no-such-marker' 'src/**/*.test.ts'; echo "rc_match=$?"
  echo "=== MATCHING LITERAL (literal terminal, direct + nested) ==="
  bash "$fp" '.no-such-marker' 'tests/**/foo.test.ts'; echo "rc_lit=$?"
  echo "=== NON-MATCHING GLOB ==="
  bash "$fp" '.marker' 'nope/**/*.spec.ts'; echo "rc_nomatch=$?"
} > polluter.out 2>&1
cat polluter.out
