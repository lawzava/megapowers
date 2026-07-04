#!/usr/bin/env bash
# Drive the shipped Stop hook with crafted transcripts + a risky git diff, and
# record whether it BLOCKS (nags) or ALLOWS (suppresses) in each case.
#
# Also guards the once-per-diff-state contract: a risky diff that hasn't
# changed since the last block has already been nudged, so a following Stop
# on the SAME diff-state must allow; but once the risky diff actually changes,
# the next Stop must nag again. See delegate-nudge.sh's own
# tests/delegate-nudge.test.sh ("once-per-diff-state tests") for the reference
# semantics this scenario mirrors at the eval layer.
hook="$ROOT/plugins/mega-orchestration/hooks/delegate-nudge.sh"

git init -q repo && cd repo || exit 1
git config user.email t@t; git config user.name t
git config commit.gpgsign false   # hermetic: don't depend on the user's signing setup
printf 'func pay(){ /* billing */ }\n' > billing.go
git add billing.go && git commit -qm init

sentinel="$(git rev-parse --git-path megapowers-delegate-nudge-seen)"
reset_sentinel() { rm -f "$sentinel" 2>/dev/null || true; }

emit() {  # $1 = transcript body
  printf '%s' "$1" > tr.jsonl
  printf '{"transcript_path":"%s/tr.jsonl","stop_hook_active":false}' "$PWD"
}
verdict() { if printf '%s' "$1" | grep -q '"decision":"block"'; then echo NAG; else echo ALLOW; fi; }

# -- delegate-call suppression path (unchanged contract): these transcripts
# match a real delegate invocation and exit before ever touching the
# once-per-diff-state sentinel, so they suppress regardless of diff history.
printf 'func pay(){ /* billing changed */ }\n' > billing.go   # risky, uncommitted
r1="$(emit '{"type":"tool_use","name":"mcp__codex__codex-reply","input":{}}' | bash "$hook")"
r2="$(emit '{"subagent_type":"codex-delegate"}' | bash "$hook")"
r3="$(emit 'prose only: I thought about codex exec but did not run it.' | bash "$hook")"

# -- notdelegate false-positive guard: a fresh (never-before-nudged) diff-state
# so a NAG here proves the hook fell through to the risky-diff check rather
# than being suppressed by "notdelegate" merely containing the word "delegate".
reset_sentinel
printf 'func pay(){ /* billing changed, notdelegate check */ }\n' > billing.go
r4="$(emit '{"subagent_type":"notdelegate"}' | bash "$hook")"

# -- once-per-diff-state regression guard (the new contract): first stop on a
# risky diff nags; the immediately-following stop on the SAME diff-state must
# allow (this is the assertion that fails against the pre-fix, always-reblock
# hook); once the diff actually changes, the next stop must nag again.
reset_sentinel
printf 'func pay(){ /* billing changed, once-per-diff-state phase */ }\n' > billing.go
r5="$(emit 'prose only, no delegate' | bash "$hook")"   # first stop on this risky diff-state
r6="$(emit 'prose only, no delegate' | bash "$hook")"   # second stop, SAME diff-state
printf 'func pay(){ /* billing changed; stripe() too */ }\n' > billing.go   # risky diff changes
r7="$(emit 'prose only, no delegate' | bash "$hook")"   # third stop, diff changed

cd ..
{
  echo "codex_reply=$(verdict "$r1")"
  echo "codex_subagent=$(verdict "$r2")"
  echo "prose_only=$(verdict "$r3")"
  echo "notdelegate=$(verdict "$r4")"
  echo "first_stop=$(verdict "$r5")"
  echo "second_stop_same_diff=$(verdict "$r6")"
  echo "third_stop_diff_changed=$(verdict "$r7")"
} > nudge.out
cat nudge.out
