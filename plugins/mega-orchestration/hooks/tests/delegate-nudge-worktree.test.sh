#!/usr/bin/env bash
# A review receipt is scoped to the linked worktree's gitdir.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
DIFF_ID="$HERE/../../skills/multi-agent-delegation/scripts/review-diff-id"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP" || exit 1

git init -q main
cd main || exit 1
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'func handler() {}\n' > svc.go
git add svc.go
git commit -qm init
git branch wt-branch
git worktree add -q ../wt wt-branch

pass=0
fail=0
cd "$TMP/wt" || exit 1
[ -f .git ] && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL linked worktree .git is not a file"; }

printf 'func handler() { billing() }\n' > svc.go
TR="$TMP/wt/transcript.jsonl"
: > "$TR"
input="$(printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TR")"

echo "== delegate-nudge linked-worktree receipt tests =="
out="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
printf '%s' "$out" | grep -q '"decision":"block"' && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL unreviewed worktree diff did not block"; }

receipt="$(git rev-parse --git-path megapowers-review-receipt.json)"
case "$receipt" in
  */worktrees/*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL receipt path is not worktree-local: %s\n' "$receipt" ;;
esac
id="$("$DIFF_ID")"
jq -cn --arg id "$id" '{
  schema:"megapowers.review-receipt.v1",
  role:"verify",
  subject:{kind:"worktree-diff",artifact:".",id:$id,claim:"billing is correct"},
  author_vendors:["openai"],
  reviewer:{provider:"claude",vendor:"anthropic",model:"review",tier:"frontier",effort:"high"},
  independent:true,
  result:{verdict:"approve",findings:[],next_steps:[],evidence:{commands:[],screenshots:[]}},
  evidence:{commands:[],screenshots:[]},
  created_at:"2026-07-23T00:00:00Z"
}' > "$receipt"

[ -f "$receipt" ] && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL receipt was not written"; }
[ ! -f "$TMP/main/.git/megapowers-review-receipt.json" ] && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL receipt leaked into main gitdir"; }
grep -q '^gitdir:' "$TMP/wt/.git" && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL .git pointer corrupted"; }

out="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
[ -z "$out" ] && pass=$((pass + 1)) || { fail=$((fail + 1)); echo "  FAIL valid worktree-local receipt did not allow"; }

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
