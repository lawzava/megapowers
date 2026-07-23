#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
DIFF_ID="$HERE/../../skills/multi-agent-delegation/scripts/review-diff-id"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP" || exit 1
git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'func handler() {}\n' > svc.go
git add svc.go
git commit -qm init
TR="$TMP/transcript.jsonl"
: > "$TR"

pass=0
fail=0
verdict() {
  local out
  out="$(printf '%s' "$1" | bash "$HOOK" 2>/dev/null)"
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
verdict_env() {
  local name="$1" value="$2" input="$3" out
  out="$(printf '%s' "$input" | env "$name=$value" bash "$HOOK" 2>/dev/null)"
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
j() {
  printf '{"stop_hook_active":%s,"transcript_path":"%s","permission_mode":"%s"}' \
    "${1:-false}" "${2:-$TR}" "${3:-default}"
}
check() {
  local got
  got="$(verdict "$2")"
  if [ "$1" = "$got" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$got" "$3"; fi
}
check_got() {
  if [ "$1" = "$2" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$2" "$3"; fi
}
receipt_path() { git rev-parse --git-path megapowers-review-receipt.json; }
write_receipt() {
  local role="${1:-verify}" author="${2:-openai}" verifier="${3:-anthropic}" verdict="${4:-approve}" id
  id="$("$DIFF_ID")"
  jq -cn --arg role "$role" --arg author "$author" --arg verifier "$verifier" \
    --arg verdict "$verdict" --arg id "$id" '{
      schema:"megapowers.review-receipt.v1",
      role:$role,
      subject:{kind:"worktree-diff",artifact:".",id:$id,claim:"risky diff is correct"},
      author_vendors:[$author],
      reviewer:{provider:"reviewer",vendor:$verifier,model:"review-model",tier:"frontier",effort:"high"},
      independent:true,
      result:{verdict:$verdict,findings:[],next_steps:[],evidence:{commands:[],screenshots:[]}},
      evidence:{commands:[],screenshots:[]},
      created_at:"2026-07-23T00:00:00Z"
    }' > "$(receipt_path)"
}

echo "== delegate-nudge receipt tests =="
check ALLOW "$(j true "$TR")" "stop-hook recursion guard"
check ALLOW "$(j false "$TR")" "clean tree"

printf 'func handler() { billing() }\n' > svc.go
check BLOCK "$(j false "$TR")" "risky diff without receipt blocks"
check BLOCK "$(j false "$TR")" "repeated stop remains blocked without receipt"

printf '{"type":"tool_use","name":"mcp__codex__codex","input":{"prompt":"translate hello"}}\n' > "$TR"
check BLOCK "$(j false "$TR")" "unrelated delegate invocation is not review proof"
printf '{"type":"tool_use","name":"Bash","input":{"command":"claude -p review"}}\n' > "$TR"
check BLOCK "$(j false "$TR")" "review-looking invocation is not a receipt"

write_receipt
check ALLOW "$(j false "$TR")" "valid current independent approval allows"

printf '// benign follow-up\nfunc handler() { billing() }\n' > svc.go
check BLOCK "$(j false "$TR")" "any later change stales receipt"
write_receipt plan_review
check BLOCK "$(j false "$TR")" "wrong review role does not approve risky diff"
write_receipt verify openai openai
check BLOCK "$(j false "$TR")" "same-vendor receipt is not independent"
write_receipt verify openai anthropic needs_attention
check BLOCK "$(j false "$TR")" "needs-attention receipt does not approve"
write_receipt verify openai anthropic approve
check ALLOW "$(j false "$TR")" "fresh corrected receipt allows"

printf 'func handler() { billing(\"staged-one\") }\n' > svc.go
git add svc.go
printf 'func handler() { billing(\"same-worktree\") }\n' > svc.go
write_receipt
check ALLOW "$(j false "$TR")" "receipt binds divergent index and worktree"
printf 'func handler() { billing(\"staged-two\") }\n' > svc.go
git add svc.go
printf 'func handler() { billing(\"same-worktree\") }\n' > svc.go
check BLOCK "$(j false "$TR")" "index-only change stales receipt"

rm -f "$(receipt_path)"
check ALLOW "$(j false "$TR" plan)" "plan permission is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_ROLE code_review "$(j false "$TR")")" "code-review role is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_ROLE visual_verify "$(j false "$TR")")" "visual-review role is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_PRESET read_only "$(j false "$TR")")" "read-only preset is exempt"
check_got ALLOW "$(verdict_env MEGAPOWERS_EXACT_OUTPUT 1 "$(j false "$TR")")" "exact-output session is exempt"

git reset -q HEAD -- svc.go
git checkout -q -- svc.go
printf 'func paymentWebhook() {}\n' > payment_handler.go
check BLOCK "$(j false "$TR")" "untracked risky file blocks"
rm -f payment_handler.go
printf 'ordinary notes\n' > notes.txt
check ALLOW "$(j false "$TR")" "benign untracked file allows"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
