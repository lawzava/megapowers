#!/usr/bin/env bash
# Perf regression test: delegate-nudge.sh must not scan every untracked file with
# a per-file grep process. 200 untracked files should complete well under 1s
# (the fix caps the content scan to the first 50, batched into one grep call).
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

# A risky untracked file within the scan cap (first 50) must still be caught.
printf 'func newPaymentWebhook() {}\n' > file10.txt
: > "$TR"
out2="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out2" | grep -q '"decision":"block"'; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL risky content within the scan cap should still nudge"; fi

echo "== $pass passed, $fail failed =="
rm -rf "$TMP"
[ "$fail" -eq 0 ]
