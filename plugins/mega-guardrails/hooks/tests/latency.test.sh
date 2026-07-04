#!/usr/bin/env bash
# Latency regression for deny-destructive.sh. A benign multi-KB heredoc (a routine
# agent file-write) must not pay the O(n^2) per-character parser cost: the cheap
# prefilter should short-circuit a no-trigger command to an instant ALLOW at any size.
# Hard cap 1.0s (generous for CI; real cost is ~0.02s after the prefilter, was ~1.1s
# before). Also pins the verdict: a benign payload stays ALLOW.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../deny-destructive.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

# Build a ~6000-char benign heredoc with no destructive trigger tokens in it.
line="data value item entry row column field record 000 111 222 333 444 555 666"
body=""
while [ "${#body}" -lt 6050 ]; do body="${body}${line}"$'\n'; done
cmd="cat > notes.txt <<'EOF'"$'\n'"${body}"$'\n'"EOF"$'\n'

pass=0; fail=0
json="$(jq -nc --arg c "$cmd" '{tool_input:{command:$c}}')"

start="$(date +%s%N)"
out="$(printf '%s' "$json" | bash "$HOOK" 2>/dev/null)"
end="$(date +%s%N)"
ms=$(( (end - start) / 1000000 ))

if [ -z "$out" ]; then verdict="ALLOW"; else verdict="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.permissionDecision' | tr 'a-z' 'A-Z')"; fi

if [ "${#cmd}" -ge 6000 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL payload only %s chars, want >=6000\n' "${#cmd}"; fi
if [ "$verdict" = "ALLOW" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL verdict=%s want ALLOW\n' "$verdict"; fi
if [ "$ms" -lt 1000 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL latency %sms, want <1000ms\n' "$ms"; fi

echo "== latency: $pass passed, $fail failed (${#cmd} chars, ${ms}ms, $verdict) =="
[ "$fail" -eq 0 ]
