#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
DIFF_ID="$HERE/../../skills/multi-agent-delegation/scripts/review-diff-id"
RESOLVER="$HERE/../../skills/multi-agent-delegation/scripts/delegate-resolve"
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

# Single-vendor degradation: with only one reachable vendor no --author-vendor
# choice can route away from the author, so the launcher command would exit 3.
# The gate must still fire (the risk is unchanged) but must stop prescribing a
# command that cannot succeed.
rm -f notes.txt
printf 'func handler() { billing() }\n' > svc.go
cat > "$TMP/one-vendor.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.solo]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.solo.tiers]
strong = "solo-1"
[roles]
verify = "solo"
EOF
: > "$TMP/empty-catalog.toml"

reason_with_env() {
  printf '%s' "$2" | env DELEGATES_TOML="$1" MODELS_TOML="$TMP/empty-catalog.toml"     bash "$HOOK" 2>/dev/null | jq -r '.reason // ""' 2>/dev/null
}
decision_with_env() {
  printf '%s' "$2" | env DELEGATES_TOML="$1" MODELS_TOML="$TMP/empty-catalog.toml"     bash "$HOOK" 2>/dev/null | jq -r '.decision // "allow"' 2>/dev/null
}

solo_reason="$(reason_with_env "$TMP/one-vendor.toml" "$(j false "$TR")")"
check_got block "$(decision_with_env "$TMP/one-vendor.toml" "$(j false "$TR")")" \
  "single-vendor setup still gates a risky change"
case "$solo_reason" in
  *"no independent reviewer is reachable"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL single-vendor reason must say no reviewer is reachable :: %s\n' "$solo_reason" ;;
esac
case "$solo_reason" in
  *delegate-run*) fail=$((fail + 1)); printf '  FAIL single-vendor reason must not prescribe the unresolvable launcher\n' ;;
  *) pass=$((pass + 1)) ;;
esac
case "$solo_reason" in
  *"go-ahead"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL single-vendor reason must name an achievable remedy\n' ;;
esac

# ZERO reachable vendors must take the same degraded path as one. `grep -c .`
# exits 1 on no matches, so a naive count discards a legitimate zero and falls
# back to the strict default, prescribing a launcher that cannot resolve.
cat > "$TMP/no-vendor.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.ghost]
vendor = "acme"
binary = "definitely-not-an-installed-binary-xyz"
channel = "cli"
default_tier = "strong"
[providers.ghost.tiers]
strong = "ghost-1"
[roles]
verify = "ghost"
EOF
zero_reason="$(reason_with_env "$TMP/no-vendor.toml" "$(j false "$TR")")"
check_got block "$(decision_with_env "$TMP/no-vendor.toml" "$(j false "$TR")")" \
  "zero-vendor setup still gates a risky change"
case "$zero_reason" in
  *"no independent reviewer is reachable"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL zero reachable vendors must take the degraded path :: %s\n' "$zero_reason" ;;
esac
case "$zero_reason" in
  *delegate-run*) fail=$((fail + 1)); printf '  FAIL zero-vendor reason must not prescribe the unresolvable launcher\n' ;;
  *) pass=$((pass + 1)) ;;
esac

# Two vendors reachable THROUGH THE VERIFY CHAIN: the normal path keeps
# prescribing the launcher. The chain must actually list both, and the assertion
# must be that resolution succeeds for the author vendor, not merely that the
# message mentions delegate-run. An earlier version of this test declared
# verify = "solo" with no fallbacks, so with author vendor acme resolution exited
# 3 while the test still passed: it was asserting the wrong thing.
cat > "$TMP/two-vendor.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.solo]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.solo.tiers]
strong = "solo-1"
[providers.duo]
vendor = "globex"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.duo.tiers]
strong = "duo-1"
[roles]
verify = "solo"
[fallbacks]
verify = ["solo", "duo"]
EOF
duo_reason="$(reason_with_env "$TMP/two-vendor.toml" "$(j false "$TR")")"
case "$duo_reason" in
  *delegate-run*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL two-vendor setup must still prescribe the launcher :: %s\n' "$duo_reason" ;;
esac
# the prescribed command must genuinely resolve for either author vendor
for av in acme globex; do
  if DELEGATES_TOML="$TMP/two-vendor.toml" MODELS_TOML="$TMP/empty-catalog.toml" \
     "$RESOLVER" verify --author-vendor "$av" >/dev/null 2>&1; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL launcher path prescribed but verify cannot resolve for author %s\n' "$av"
  fi
done

# A chain that reaches two vendors GLOBALLY but only one through verify must take
# the degraded path: the role, not the machine, decides whether a review resolves.
cat > "$TMP/role-scoped.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.solo]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.solo.tiers]
strong = "solo-1"
[providers.duo]
vendor = "globex"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.duo.tiers]
strong = "duo-1"
[roles]
verify = "solo"
EOF
scoped_reason="$(reason_with_env "$TMP/role-scoped.toml" "$(j false "$TR")")"
case "$scoped_reason" in
  *"no independent reviewer is reachable"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a second vendor outside the verify chain must not count as reachable :: %s\n' "$scoped_reason" ;;
esac

# Self-exclusion: the guard must not police edits to its own source or tests.
# Those files necessarily contain the risky keyword list verbatim, so without the
# exclusion any edit to the guard trips the guard and the review request ends up
# citing its own warning text as the risky change.
git reset -q HEAD -- . 2>/dev/null
git checkout -q -- svc.go 2>/dev/null
rm -f "$(receipt_path)"
mkdir -p plugins/mega-orchestration/hooks/tests
printf 'risky=%s\n' "'authn|billing|concurren'" > plugins/mega-orchestration/hooks/delegate-nudge.sh
check ALLOW "$(j false "$TR")" "editing the guard itself does not trip the guard"
printf 'printf %s > svc.go\n' "'func handler() { billing() }'" \
  > plugins/mega-orchestration/hooks/tests/fixture.test.sh
check ALLOW "$(j false "$TR")" "guard test fixtures naming billing do not trip the guard"

# ...but a sibling hook carrying the same words is still gated.
printf 'func chargeCard() { stripe() }\n' > plugins/mega-orchestration/hooks/other-hook.sh
check BLOCK "$(j false "$TR")" "a sibling hook with risky logic is still gated"
rm -rf plugins

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
