#!/usr/bin/env bash
# Drive the shipped Stop hook with crafted transcripts + a risky git diff, and
# record whether it BLOCKS (nags) or ALLOWS (suppresses) in each case.
hook="$ROOT/plugins/mega-orchestration/hooks/delegate-nudge.sh"

git init -q repo && cd repo || exit 1
git config user.email t@t; git config user.name t
git config commit.gpgsign false   # hermetic: don't depend on the user's signing setup
printf 'func pay(){ /* billing */ }\n' > billing.go
git add billing.go && git commit -qm init
printf 'func pay(){ /* billing changed */ }\n' > billing.go   # risky, uncommitted

emit() {  # $1 = transcript body
  printf '%s' "$1" > tr.jsonl
  printf '{"transcript_path":"%s/tr.jsonl","stop_hook_active":false}' "$PWD"
}
verdict() { if printf '%s' "$1" | grep -q '"decision":"block"'; then echo NAG; else echo ALLOW; fi; }

r1="$(emit '{"type":"tool_use","name":"mcp__codex__codex-reply","input":{}}' | bash "$hook")"
r2="$(emit '{"subagent_type":"codex-delegate"}' | bash "$hook")"
r3="$(emit 'prose only: I thought about codex exec but did not run it.' | bash "$hook")"
# a subagent whose type merely contains "delegate" (e.g. "notdelegate") must NOT
# suppress — guards the token-bounding fix from a cross-model pass.
r4="$(emit '{"subagent_type":"notdelegate"}' | bash "$hook")"

cd ..
{
  echo "codex_reply=$(verdict "$r1")"
  echo "codex_subagent=$(verdict "$r2")"
  echo "prose_only=$(verdict "$r3")"
  echo "notdelegate=$(verdict "$r4")"
} > nudge.out
cat nudge.out
