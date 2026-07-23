#!/usr/bin/env bash
set -euo pipefail

hook="$ROOT/plugins/mega-orchestration/hooks/delegate-nudge.sh"
diff_id="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/scripts/review-diff-id"

git init -q repo
cd repo
git config user.email t@t
git config user.name t
git config commit.gpgsign false
printf 'func pay(){ /* billing */ }\n' > billing.go
git add billing.go
git commit -qm init
printf 'func pay(){ /* billing changed */ }\n' > billing.go
printf '{"type":"tool_use","name":"mcp__codex__codex-reply","input":{}}\n' > tr.jsonl

emit() {
  printf '{"transcript_path":"%s/tr.jsonl","stop_hook_active":false}' "$PWD" | bash "$hook"
}
verdict() {
  if printf '%s' "$1" | grep -q '"decision":"block"'; then echo NAG; else echo ALLOW; fi
}
write_receipt() {
  local result="$1" id receipt
  id="$("$diff_id")"
  receipt="$(git rev-parse --git-path megapowers-review-receipt.json)"
  jq -cn --arg id "$id" --arg result "$result" '{
    schema:"megapowers.review-receipt.v1",
    role:"verify",
    subject:{kind:"worktree-diff",artifact:".",id:$id,claim:"billing change is correct"},
    author_vendors:["openai"],
    reviewer:{provider:"claude",vendor:"anthropic",model:"claude-fable-5",tier:"frontier",effort:"high"},
    independent:true,
    result:{verdict:$result,findings:[],next_steps:[],evidence:{commands:[],screenshots:[]}},
    evidence:{commands:[],screenshots:[]},
    created_at:"2026-07-23T00:00:00Z"
  }' > "$receipt"
}

without_receipt="$(verdict "$(emit)")"
repeated_without_receipt="$(verdict "$(emit)")"
write_receipt approve
current_approval="$(verdict "$(emit)")"
printf 'func pay(){ /* billing changed again */ }\n' > billing.go
stale_approval="$(verdict "$(emit)")"
write_receipt needs_attention
needs_attention="$(verdict "$(emit)")"

cd ..
{
  echo "delegate_call_without_receipt=$without_receipt"
  echo "repeated_without_receipt=$repeated_without_receipt"
  echo "current_approval=$current_approval"
  echo "stale_approval=$stale_approval"
  echo "needs_attention=$needs_attention"
} > nudge.out
cat nudge.out
