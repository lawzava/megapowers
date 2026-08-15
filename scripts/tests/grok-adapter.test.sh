#!/usr/bin/env bash
# The Grok adapter, driven by Grok's REAL payload shapes. The 2026-08 audit
# found both Stop wrappers dead on the installed machine: Grok's Stop payload
# carries stopHookActive and lastAssistantMessage but no `reason`, the
# normalizer fabricated reason:"", and the end_turn guard read "" as "not an
# end turn", so answer-style and delegate-nudge never ran in a live Grok turn.
# Every case here feeds the documented Grok payload, not a Claude one.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
WRAP_STOP="$ROOT/templates/grok/hooks/bin/wrap-stop.sh"
WRAP_DENY="$ROOT/templates/grok/hooks/bin/wrap-deny-destructive.sh"
export MEGAPOWERS_PLUGIN_DIR="$ROOT/plugins"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0; fail=0
ok()  { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }

command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }
[ -x "$WRAP_STOP" ] && ok || bad "wrap-stop.sh ships in templates/grok and is executable"
[ -x "$WRAP_DENY" ] && ok || bad "wrap-deny-destructive.sh ships in templates/grok and is executable"

grok_stop() {  # $1=stopHookActive $2=lastAssistantMessage [$3=extra jq object]
  jq -nc --argjson a "$1" --arg m "$2" \
    '{hookEventName:"Stop", stopHookActive:$a, lastAssistantMessage:$m}'"${3:+ | . + $3}"
}

echo "== grok wrap-stop: answer-style =="
out="$(grok_stop false 'The verdict is clear — ship it.' | "$WRAP_STOP" megapowers/hooks/answer-style.sh)"
printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q block && ok \
  || bad "a reason-less Grok payload with a dash must reach the dash gate (got '$out')"
printf '%s' "$out" | grep -q 'Answer contract' && ok \
  || bad "the block carries the answer-contract reason"

out="$(grok_stop true 'Still has a dash — but the guard is active.' | "$WRAP_STOP" megapowers/hooks/answer-style.sh)"
[ -z "$out" ] && ok || bad "stopHookActive must pass through the recursion guard (got '$out')"

out="$(grok_stop false 'A clean reply with no dash at all.' | "$WRAP_STOP" megapowers/hooks/answer-style.sh)"
[ -z "$out" ] && ok || bad "a clean reply stays silent (got '$out')"

out="$(jq -nc '{hookEventName:"Stop", stopHookActive:false, reason:"user_interrupt", lastAssistantMessage:"dash — here"}' \
  | "$WRAP_STOP" megapowers/hooks/answer-style.sh)"
[ -z "$out" ] && ok || bad "an explicit non-end_turn reason still skips the gate (got '$out')"

echo "== grok wrap-stop: delegate-nudge =="
repo="$TMP/repo"
mkdir -p "$repo"
(
  cd "$repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'func handler() { billing() }\n' > svc.go
) >/dev/null 2>&1
out="$(cd "$repo" && grok_stop false 'Implemented the billing change.' | "$WRAP_STOP" mega-orchestration/hooks/delegate-nudge.sh)"
printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q block && ok \
  || bad "a risky pending diff must block through the Grok Stop adapter (got '${out:0:120}')"

echo "== grok wrap-deny-destructive =="
out="$(jq -nc '{hookEventName:"PreToolUse", toolName:"run_terminal_command", toolInput:{command:"rm -rf /home/z"}}' \
  | "$WRAP_DENY")"
printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q deny && ok \
  || bad "a camelCase catastrophic command must be denied (got '$out')"
out="$(jq -nc '{hookEventName:"PreToolUse", toolName:"run_terminal_command", toolInput:{command:"ls -la"}}' \
  | "$WRAP_DENY")"
[ -z "$out" ] && ok || bad "a benign command emits nothing (got '$out')"
# Claude ASK is not a Grok decision: reversible-risk commands fall through to
# Grok's own permission prompt rather than being force-allowed or denied.
out="$(jq -nc '{hookEventName:"PreToolUse", toolName:"run_terminal_command", toolInput:{command:"git reset --hard"}}' \
  | "$WRAP_DENY")"
[ -z "$out" ] && ok || bad "an ASK-tier command passes through to Grok's own prompt (got '$out')"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
