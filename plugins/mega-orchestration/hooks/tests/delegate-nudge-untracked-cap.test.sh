#!/usr/bin/env bash
# Perf regression test: delegate-nudge.sh must not scan every untracked file with
# a per-file grep process. 200 untracked files should complete well under 1s.
#
# The content scan reads EVERY regular untracked file. It once stopped at the
# first 50, which meant path 51 carried risky content past a security scan in
# silence, and the path is named by whoever adds the file. The bound is gone and
# the budget below is what keeps its removal honest: the scan batches paths into
# grep calls rather than spawning one process per file, so dropping the cap cost
# nothing measurable. Regress to per-file greps and this test says so immediately.
# Do not relax the budget to make a slow scan fit.
# Run: plugins/mega-orchestration/hooks/tests/delegate-nudge-untracked-cap.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
TMP="$(mktemp -d)"
cd "$TMP" || exit 1
git init -q
git config user.email t@t; git config user.name t; git config commit.gpgsign false
git commit -q --allow-empty -m init

for i in $(seq 1 200); do printf 'file %d, nothing interesting here\n' "$i" > "file$i.txt"; done

TR="$TMP/transcript.jsonl"
: > "$TR"
input="$(printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TR")"

pass=0; fail=0
started_ns="$(date +%s%N)"
out="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
ended_ns="$(date +%s%N)"
ms=$(( (ended_ns - started_ns) / 1000000 ))

echo "== delegate-nudge untracked-file cap tests =="
if [ "$ms" -lt 1000 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL 200 untracked files took %sms, want <1000ms\n' "$ms"; fi
printf '  (elapsed: %sms)\n' "$ms"

if printf '%s' "$out" | grep -q '"decision":"block"'; then fail=$((fail + 1)); echo "  FAIL 200 benign untracked files should not nudge"; else pass=$((pass + 1)); fi

# A risky untracked file in the first handful of paths must still be caught.
printf 'func newPaymentWebhook() {}\n' > file10.txt
: > "$TR"
out2="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out2" | grep -q '"decision":"block"'; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL risky content in an early untracked path should nudge"; fi

# ...and so must a file PAST the fifty-path bound that used to exist, which is the
# assertion that stops the bound being reintroduced as a way to win the budget
# above. The path is taken from git's own enumeration order rather than guessed
# from a filename, so it stays past the old cap no matter how the fixture is
# renamed. The early file goes back to benign first: leave it risky and the scan
# stops there and this case passes without ever reaching path 51.
printf 'file 10, nothing interesting here\n' > file10.txt
late="$(git ls-files --others --exclude-standard | sed -n '51p')"
if [ -n "$late" ]; then
  printf 'func chargeCard() { stripe() }\n' > "$late"
  : > "$TR"
  out3="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
  if printf '%s' "$out3" | grep -q '"decision":"block"'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL risky content past the old fifty-path cap should nudge (path: %s)\n' "$late"; fi
else
  fail=$((fail + 1)); echo "  FAIL fixture has fewer than 51 untracked paths, the past-the-cap case tested nothing"
fi

echo "== $pass passed, $fail failed =="
rm -rf "$TMP"
[ "$fail" -eq 0 ]
