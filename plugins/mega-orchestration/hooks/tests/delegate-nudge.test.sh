#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
DIFF_ID="$HERE/../../skills/multi-agent-delegation/scripts/review-diff-id"
RESOLVER="$HERE/../../skills/multi-agent-delegation/scripts/delegate-resolve"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin"
for binary in claude codex opencode; do
  printf '#!/bin/sh\nexit 0\n' > "$TMP/bin/$binary"
  chmod +x "$TMP/bin/$binary"
done
export PATH="$TMP/bin:$PATH"
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
verdict_in() {
  local dir="$1" input="$2" out rc
  # The hook exits 0 on every path, so a non-zero status here means the fixture
  # itself is broken. Checking it matters: `cd` failing alone leaves out empty,
  # which maps to ALLOW, so a mistyped fixture path would read as a passing case.
  out="$(cd "$dir" || exit 1; printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
  rc=$?
  if [ "$rc" -ne 0 ]; then echo "FIXTURE-ERROR(rc=$rc,dir=$dir)"; return; fi
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
reason_in() {
  local dir="$1" input="$2"
  (cd "$dir" && printf '%s' "$input" | bash "$HOOK" 2>/dev/null) | jq -r '.reason // ""' 2>/dev/null
}
# The non-blocking channel. Unscannable content is announced here rather than
# blocked, so a case checking only the decision would pass over a gate that had
# gone silent, which is the hole that made every unscannable file a hit.
# The unscanned note is said once per tree state, so a fixture that runs the
# hook twice on one tree resets the dedupe state first: this helper asserts
# what the note SAYS, and the once-only property has its own cases at the end.
notice_in() {
  local dir="$1" input="$2"
  (cd "$dir" && rm -f "$(git rev-parse --git-path megapowers-unscanned-note)" 2>/dev/null
   printf '%s' "$input" | bash "$HOOK" 2>/dev/null) | jq -r '.systemMessage // ""' 2>/dev/null
}
# The same channel WITHOUT the state reset, for the once-only cases themselves.
notice_in_stateful() {
  local dir="$1" input="$2"
  (cd "$dir" && printf '%s' "$input" | bash "$HOOK" 2>/dev/null) | jq -r '.systemMessage // ""' 2>/dev/null
}
says_got() {
  case "$2" in
    *"$1"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL want %s :: %s :: %s\n' "$1" "$3" "$2" ;;
  esac
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
  local role="${1:-verify}" author="${2:-openai}" verifier="${3:-anthropic}" verdict="${4:-approve}" id root base sub
  id="$("$DIFF_ID")"
  # The supplemental submodule fingerprint, exactly as delegate-run records it:
  # empty (and therefore omitted) in a repository with no gitlink, which is every
  # fixture above this line.
  sub="$("$DIFF_ID" --submodules)" || sub=""
  # The absolute worktree root, exactly as delegate-run records it. A receipt is
  # scoped to the tree it was produced in, so a fixture writing "." here would be
  # asserting a shape the launcher never emits.
  root="$(git rev-parse --show-toplevel)"
  # The base the delta is measured from, also exactly as delegate-run records it.
  # Without it the receipt says "this diff, somewhere" rather than "this diff, on
  # this tree", and the gate rejects it.
  base="$(git rev-parse --verify -q HEAD)" || base=""
  [ -n "$base" ] || base="$(git hash-object -t tree /dev/null)"
  jq -cn --arg role "$role" --arg author "$author" --arg verifier "$verifier" \
    --arg verdict "$verdict" --arg id "$id" --arg root "$root" --arg base "$base" --arg sub "$sub" '{
      schema:"megapowers.review-receipt.v2",
      role:$role,
      subject:({kind:"worktree-diff",artifact:$root,id:$id,base:$base,claim:"risky diff is correct"}
               + (if $sub == "" then {} else {submodules:$sub} end)),
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

# A receipt clears the FINDING, not a pending policy edit. The reviewer is a
# delegate model, and a change to .megapowers/enforcement.toml is the one thing
# that needs a human read: it is not honored at this stop, and once committed it
# governs every later one. An approved tree must still say the layer changed.
mkdir -p .megapowers
printf '[rules.risky-logic-review]\nstate = "off"\n' > .megapowers/enforcement.toml
write_receipt
check ALLOW "$(j false "$TR")" "an approved tree with a pending policy edit still allows"
policy_notice="$(printf '%s' "$(j false "$TR")" | bash "$HOOK" 2>/dev/null | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$policy_notice" in
  *".megapowers/enforcement.toml"*"would set state = off."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a receipt must not silence a pending policy edit :: %s\n' "$policy_notice" ;;
esac
rm -rf .megapowers
write_receipt verify openai anthropic approve

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
# citing its own warning text as the risky change. The paths come from
# `self_exclude` in enforcement.toml.
#
# THE PREFIX THEY SIT UNDER HAS TO BE PROVEN. The entries are plugin-relative, so
# something has to say where this plugin lives in the repository being reviewed,
# and a directory NAME is not that something: anyone can mkdir one. What counts is
# a committed .claude-plugin/plugin.json at that prefix naming this plugin, so the
# fixture declares one the way a real checkout does, in a commit.
git reset -q HEAD -- . 2>/dev/null
git checkout -q -- svc.go 2>/dev/null
rm -f "$(receipt_path)"
mkdir -p plugins/mega-orchestration/hooks/tests plugins/mega-orchestration/.claude-plugin
printf '{"name":"mega-orchestration"}\n' > plugins/mega-orchestration/.claude-plugin/plugin.json
git add plugins/mega-orchestration/.claude-plugin/plugin.json
git commit -qm "declare the plugin"
printf 'risky=%s\n' "'authn|billing|concurren'" > plugins/mega-orchestration/hooks/delegate-nudge.sh
check ALLOW "$(j false "$TR")" "editing the guard itself does not trip the guard"
printf 'printf %s > svc.go\n' "'func handler() { billing() }'" \
  > plugins/mega-orchestration/hooks/tests/delegate-nudge-prose.test.sh
check ALLOW "$(j false "$TR")" "the guard's own test fixtures naming billing do not trip the guard"

# ...and the exclusion covers exactly the files the rules file names. A test the
# rules file does not name is source like any other: a test can carry real auth
# logic, and a scan that skips the test tree is a scan with a known hole.
printf 'func chargeCard() { stripe() }\n' > plugins/mega-orchestration/hooks/tests/other.test.sh
check BLOCK "$(j false "$TR")" "a test the rules file does not name is still gated"
rm -f plugins/mega-orchestration/hooks/tests/other.test.sh

# ...and so is a sibling hook carrying the same words.
printf 'func chargeCard() { stripe() }\n' > plugins/mega-orchestration/hooks/other-hook.sh
check BLOCK "$(j false "$TR")" "a sibling hook with risky logic is still gated"
rm -f plugins/mega-orchestration/hooks/other-hook.sh

# THE EXCLUSION IS ANCHORED TO THIS PLUGIN, not matched as a bare suffix. An
# unanchored suffix excluded any path that merely ended the same way, so real
# logic parked at `<anything>/hooks/delegate-nudge.sh`, or an `enforcement.toml`
# at any depth in an unrelated repository, bought silence with a filename. Both
# decoys carry no risky word in the NAME, so only their content can gate.
mkdir -p services/payments/hooks
printf 'func chargeCard() { stripe() }\n' > services/payments/hooks/delegate-nudge.sh
check BLOCK "$(j false "$TR")" "a decoy path ending in a self_exclude entry is still scanned"
rm -rf services
mkdir -p config/deep
printf 'keywords = [%s]\n' '"billing"' > config/deep/enforcement.toml
check BLOCK "$(j false "$TR")" "a nested enforcement.toml is still scanned"
rm -rf config

# A DIRECTORY NAMED AFTER THE PLUGIN IS NOT THE PLUGIN. Anchoring on the name
# alone moved the blind spot up one level instead of closing it: risky code parked
# at `<anything>/mega-orchestration/hooks/delegate-nudge.sh` in an unrelated
# repository went unscanned for the price of an mkdir.
mkdir -p vendor/mega-orchestration/hooks
printf 'func chargeCard() { stripe() }\n' > vendor/mega-orchestration/hooks/delegate-nudge.sh
check BLOCK "$(j false "$TR")" "a directory merely named after the plugin does not anchor the exclusion"
rm -rf vendor
# ...and the declaration that DOES anchor it has to be committed, for the reason
# the project policy layer is read from the base commit: a pending tree that could
# mint its own anchor could exempt itself, which is the bypass in another shape.
mkdir -p decoy/mega-orchestration/.claude-plugin decoy/mega-orchestration/hooks
printf '{"name":"mega-orchestration"}\n' > decoy/mega-orchestration/.claude-plugin/plugin.json
printf 'func chargeCard() { stripe() }\n' > decoy/mega-orchestration/hooks/delegate-nudge.sh
check BLOCK "$(j false "$TR")" "an uncommitted plugin manifest does not anchor the exclusion for its own change"
rm -rf decoy
# ...and a committed manifest naming a DIFFERENT plugin anchors nothing either, so
# the check is about identity rather than about the file merely existing.
mkdir -p other/mega-orchestration/.claude-plugin other/mega-orchestration/hooks
printf '{"name":"some-other-plugin"}\n' > other/mega-orchestration/.claude-plugin/plugin.json
git add other/mega-orchestration/.claude-plugin/plugin.json
git commit -qm "declare an unrelated plugin"
printf 'func chargeCard() { stripe() }\n' > other/mega-orchestration/hooks/delegate-nudge.sh
check BLOCK "$(j false "$TR")" "a committed manifest for a different plugin does not anchor the exclusion"
rm -rf other
git add -A -- other
git commit -qm "drop the unrelated plugin"

# ...and the plugin's own paths are still spared, so the anchoring did not simply
# delete the exclusion.
check ALLOW "$(j false "$TR")" "the plugin's own files remain excluded under the anchored match"
rm -rf plugins
git add -A -- plugins
git commit -qm "drop the plugin fixture"

# A tracked path that is not a regular file must not silence the gate. The Claude
# Code sandbox bind mounts /dev/null over deny-listed paths, so a tracked
# .env.example becomes a character device, and `git diff HEAD` then aborts with
# status 128 and writes nothing at all. Read as output rather than as a status,
# that empty result looks like a clean tree and the guard passes a diff it never
# computed. The risky edit here lives in a later-sorting path, exactly the case
# git never reaches. A fifo stands in for the character device: git rejects both
# with "unsupported file type".
fifo_repo="$TMP/fifo-repo"
mkdir -p "$fifo_repo"
(
  cd "$fifo_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'KEY=1\n' > .env.example
  printf 'func handler() {}\n' > worker.go
  git add .env.example worker.go
  git commit -qm init
  rm .env.example
  mkfifo .env.example
  printf 'func handler() { concurrentQueue() }\n' > worker.go
)
check_got BLOCK "$(verdict_in "$fifo_repo" "$(j false "$TR")")" \
  "non-regular tracked file does not silence the guard"

# The excluded path carries no logic, so dropping it must not invent a demand
# either: a benign tree with a fifo in it still allows.
printf 'func handler() { queue() }\n' > "$fifo_repo/worker.go"
check_got ALLOW "$(verdict_in "$fifo_repo" "$(j false "$TR")")" \
  "non-regular tracked file alone is not a false gate demand"

# Deleting risky logic is risk going away, and under `added_lines_only` a pure
# deletion contributes no added line, so it cannot trip the keyword scan. This
# case asserted BLOCK while the gate grepped the whole raw diff: what it matched
# was the `diff --git a/billing.go` header, not any code, which is the same
# filename-shaped false positive the audit counted 47 of in this repository.
#
# The exclusion loop still must not drop a path merely because it is absent from
# the worktree, and it still does not: `[ -e ]` guards that. It is no longer
# observable from this hook's verdict, because a deletion that reaches the diff
# and a deletion dropped from it now both allow.
(
  cd "$fifo_repo" || exit 1
  printf 'func chargeCard() { stripe() }\n' > billing.go
  git add billing.go
  git commit -qm billing
  rm billing.go
)
check_got ALLOW "$(verdict_in "$fifo_repo" "$(j false "$TR")")" \
  "deleting a risky file is not itself a risky change"
(cd "$fifo_repo" && git checkout -q -- billing.go)
# The CONTROL for the case below, and the reason it is a separate case at all. An
# earlier version untracked billing.go and asserted the block came from re-adding
# risky content; what it actually exercised was the untracked NAME scan matching
# "billing", so a zero-byte file blocked and the same empty file renamed
# neutral.go allowed. The path stays TRACKED here, so nothing about the name can
# gate: a clean tracked billing.go alongside the fifo must allow.
check_got ALLOW "$(verdict_in "$fifo_repo" "$(j false "$TR")")" \
  "a tracked risky filename with no pending change is not a gate demand"
# ...and the risk ARRIVING as an added tracked line is gated, alongside the
# non-regular path that would otherwise abort the diff. Under added_lines_only the
# `diff --git a/billing.go` header never reaches the scan, so this can only be the
# added line.
printf 'func chargeCard() { stripe() }\nfunc refundCard() { stripe(order) }\n' \
  > "$fifo_repo/billing.go"
check_got BLOCK "$(verdict_in "$fifo_repo" "$(j false "$TR")")" \
  "a risky line added to a tracked file is gated alongside a non-regular path"
(cd "$fifo_repo" && git checkout -q -- billing.go)

# The exclusion must be built from repo-root-relative paths, because `:(top,...)`
# resolves against the repo root while `git ls-files` prints paths relative to the
# working directory. Run from a subdirectory, a cwd-relative name excludes nothing,
# the retry fails again, and a benign tree earns a false review demand.
mkdir -p "$fifo_repo/internal/svc"
printf 'func queue() {}\n' > "$fifo_repo/internal/svc/queue.go"
(cd "$fifo_repo" && git add internal/svc/queue.go && git commit -qm sub)
printf 'func queue() { drain() }\n' > "$fifo_repo/internal/svc/queue.go"
check_got ALLOW "$(verdict_in "$fifo_repo/internal/svc" "$(j false "$TR")")" \
  "a benign tree with a non-regular path allows from a subdirectory"
printf 'func queue() { mutexHeld() }\n' > "$fifo_repo/internal/svc/queue.go"
check_got BLOCK "$(verdict_in "$fifo_repo/internal/svc" "$(j false "$TR")")" \
  "a risky change is still gated from a subdirectory"
(cd "$fifo_repo" && git checkout -q -- internal/svc/queue.go)

# A tracked non-regular path whose NAME carries a pathspec metacharacter must
# exclude only itself. Without `literal` the name is matched with wildmatch and
# `*` crosses directory separators, so a fifo called `x*.go` drops every path
# starting with `x` from the diff the guard reads: the risky edit vanishes and the
# hook allows in silence. The name is chosen by whoever adds the file, so the glob
# picks what goes unreviewed. The sibling review-diff-id carries the same fixture
# for the same reason, and the defect was found there first.
metachar_repo() {
  local dir="$1" fifo="$2" victim="$3" body="$4"
  mkdir -p "$dir"
  (
    cd "$dir" || exit 1
    git init -q
    git config user.email test@example.com
    git config user.name test
    git config commit.gpgsign false
    # A regular placeholder first, because git refuses to add a fifo at all.
    printf 'KEY=1\n' > "$fifo"
    printf 'func handler() {}\n' > "$victim"
    git add -A
    git commit -qm init
    rm -- "$fifo"
    mkfifo -- "$fifo"
    printf '%s\n' "$body" > "$victim"
  )
}

# The victim name itself carries no risky token, so these cases turn on the diff
# BODY reaching the gate rather than on a filename in the diff header.
metachar_repo "$TMP/star-repo" 'x*.go' xcharge.go 'func chargeCard() { stripe() }'
check_got BLOCK "$(verdict_in "$TMP/star-repo" "$(j false "$TR")")" \
  "a fifo named with * does not exclude a risky tracked file"
# ...and the exclusion still must not invent a demand on a benign tree.
printf 'func handler() { queue() }\n' > "$TMP/star-repo/xcharge.go"
check_got ALLOW "$(verdict_in "$TMP/star-repo" "$(j false "$TR")")" \
  "a metacharacter fifo alone is not a false gate demand"

metachar_repo "$TMP/question-repo" 'w?.go' wz.go 'func chargeCard() { stripe() }'
check_got BLOCK "$(verdict_in "$TMP/question-repo" "$(j false "$TR")")" \
  "a fifo named with ? does not exclude a risky tracked file"

metachar_repo "$TMP/bracket-repo" 'v[ab].go' va.go 'func handler() { mutexHeld() }'
check_got BLOCK "$(verdict_in "$TMP/bracket-repo" "$(j false "$TR")")" \
  "a fifo named with a character class does not exclude a risky tracked file"

# The untracked scan reuses the same mutated pathspec, so a glob-matched untracked
# file is lost the same way. Here the tracked tree is unchanged and the only risky
# content is untracked, which isolates that leg.
metachar_repo "$TMP/untracked-metachar-repo" 'q*.go' qworker.go 'func handler() {}'
printf 'func chargeCard() { stripe() }\n' > "$TMP/untracked-metachar-repo/qpayment.go"
check_got BLOCK "$(verdict_in "$TMP/untracked-metachar-repo" "$(j false "$TR")")" \
  "a metacharacter fifo does not hide a risky untracked file"

# When the diff stays uncomputable after the non-regular paths are excluded, the
# guard must say so and ask. Here a tracked regular file git cannot open keeps the
# retry failing, and nothing in the tree carries a risky token: an unreadable diff
# is itself the reason to ask, because the guard cannot know what changed.
if [ "$(id -u)" != 0 ]; then
  unreadable_repo="$TMP/unreadable-repo"
  mkdir -p "$unreadable_repo"
  (
    cd "$unreadable_repo" || exit 1
    git init -q
    git config user.email test@example.com
    git config user.name test
    git config commit.gpgsign false
    printf 'KEY=1\n' > .env.example
    printf 'token\n' > locked.txt
    printf 'func handler() {}\n' > worker.go
    git add .env.example locked.txt worker.go
    git commit -qm init
    rm .env.example
    mkfifo .env.example
    printf 'func handler() { queue() }\n' > worker.go
    chmod 000 locked.txt
  )
  check_got BLOCK "$(verdict_in "$unreadable_repo" "$(j false "$TR")")" \
    "unreadable diff asks rather than passes"
  unreadable_reason="$(reason_in "$unreadable_repo" "$(j false "$TR")")"
  case "$unreadable_reason" in
    *"could not compute the pending diff"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL unreadable diff must name the uncomputable diff :: %s\n' "$unreadable_reason" ;;
  esac
  case "$unreadable_reason" in
    *locked.txt*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL unreadable diff must name the offending path :: %s\n' "$unreadable_reason" ;;
  esac
  # This block sits ahead of the receipt check and no id can be computed for a
  # tree git cannot read, so no receipt clears it. The reason must say so, and
  # must not send the agent after a review that changes nothing: that is the
  # pressure that pushes a lead toward claiming a pass it did not get.
  case "$unreadable_reason" in
    *"receipt cannot clear this block"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL unreadable diff must say a receipt cannot clear it :: %s\n' "$unreadable_reason" ;;
  esac
  case "$unreadable_reason" in
    *"cross-vendor review before shipping"*)
      fail=$((fail + 1)); printf '  FAIL unreadable diff must not prescribe a review that cannot clear it\n' ;;
    *) pass=$((pass + 1)) ;;
  esac
  chmod 644 "$unreadable_repo/locked.txt"
else
  # Say so out loud. This is the task's headline assertion, and a silent skip
  # leaves a lower pass count as the only signal that a root container dropped it.
  printf '  SKIP as root: unreadable diff asks rather than passes (chmod 000 does not block root, 5 assertions)\n'
fi

# A repository with no commits has no diff base at all, so `git diff HEAD` fails
# there for an ordinary reason. That is not an unreadable tree: every file is
# untracked and the untracked scan sees all of it, so a benign one must still
# allow while a risky one still blocks.
fresh_repo="$TMP/fresh-repo"
mkdir -p "$fresh_repo"
(
  cd "$fresh_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'ordinary notes\n' > notes.txt
)
check_got ALLOW "$(verdict_in "$fresh_repo" "$(j false "$TR")")" \
  "a repository with no commits is not an unreadable diff"
printf 'func chargeCard() { stripe() }\n' > "$fresh_repo/billing.go"
check_got BLOCK "$(verdict_in "$fresh_repo" "$(j false "$TR")")" \
  "risky first commit is still gated with no diff base"

# Staging is where treating a missing diff base as an empty diff reopens the hole:
# the index holds the content, so `git ls-files --others` stops reporting the file
# and the untracked scan sees nothing. The guard must diff against the empty tree
# rather than assume the tree is clean.
(cd "$fresh_repo" && git add billing.go)
check_got BLOCK "$(verdict_in "$fresh_repo" "$(j false "$TR")")" \
  "staged risky content with no diff base is still gated"
# ...and reading the index must not invent a demand either: staged benign content
# produces a real diff with no risky token in it.
(cd "$fresh_repo" && git rm -q -f billing.go && git add notes.txt)
check_got ALLOW "$(verdict_in "$fresh_repo" "$(j false "$TR")")" \
  "staged benign content with no diff base still allows"

# ...and a repository with no commits holds no reviewed content, so it holds no
# trustworthy project policy either. The opt-out layer is honored as of the base
# commit, and there is no base commit here, so the shipped state applies.
mkdir -p "$fresh_repo/.megapowers"
printf '[rules.risky-logic-review]\nstate = "off"\n' > "$fresh_repo/.megapowers/enforcement.toml"
printf 'func chargeCard() { stripe() }\n' > "$fresh_repo/billing.go"
check_got BLOCK "$(verdict_in "$fresh_repo" "$(j false "$TR")")" \
  "a project layer in a repository with no commits cannot switch the gate off"
rm -rf "$fresh_repo/.megapowers" "$fresh_repo/billing.go"

# A receipt is scoped to the worktree it was produced in. review-diff-id
# fingerprints the pending delta and nothing else: not the repository, not the
# base commit. Two unrelated checkouts with different HEADs and the same pending
# change therefore fingerprint identically, so an approve receipt lifted out of
# one gitdir into another authorizes code its reviewer never opened. The receipt
# already records which worktree it covers; the gate has to read it.
portable_repo() {
  local dir="$1" tag="$2"
  mkdir -p "$dir"
  (
    cd "$dir" || exit 1
    git init -q
    git config user.email test@example.com
    git config user.name test
    git config commit.gpgsign false
    printf 'func handler() {}\n' > svc.go
    git add svc.go
    # Distinct commit messages give the two repositories different HEADs while
    # the pending delta stays byte identical, which is the collision itself.
    git commit -qm "init $tag"
    printf 'func handler() { billing() }\n' > svc.go
  )
}
receipt_in() {
  local dir="$1" artifact="$2"
  (
    cd "$dir" || exit 1
    jq -cn --arg id "$("$DIFF_ID")" --arg artifact "$artifact" \
      --arg sub "$("$DIFF_ID" --submodules || true)" \
      --arg base "$(git rev-parse --verify -q HEAD)" '{
      schema:"megapowers.review-receipt.v2",
      role:"verify",
      subject:({kind:"worktree-diff",artifact:$artifact,id:$id,base:$base,claim:"billing is correct"}
               + (if $sub == "" then {} else {submodules:$sub} end)),
      author_vendors:["openai"],
      reviewer:{provider:"reviewer",vendor:"anthropic",model:"review-model",tier:"frontier",effort:"high"},
      independent:true,
      result:{verdict:"approve",findings:[],next_steps:[],evidence:{commands:[],screenshots:[]}},
      evidence:{commands:[],screenshots:[]},
      created_at:"2026-07-23T00:00:00Z"
    }' > "$(git rev-parse --git-path megapowers-review-receipt.json)"
  )
}
portable_repo "$TMP/donor-repo" donor
portable_repo "$TMP/victim-repo" victim
donor_id="$(cd "$TMP/donor-repo" && "$DIFF_ID")"
victim_id="$(cd "$TMP/victim-repo" && "$DIFF_ID")"
# The premise of every assertion below. If the fingerprint ever starts covering
# repository or base identity, these ids diverge and the cases stop testing the
# artifact binding at all, so the collision is asserted rather than assumed.
check_got "$donor_id" "$victim_id" "two repositories with the same pending change fingerprint identically"

receipt_in "$TMP/donor-repo" "$TMP/donor-repo"
check_got ALLOW "$(verdict_in "$TMP/donor-repo" "$(j false "$TR")")" \
  "a receipt naming its own worktree allows"
cp "$TMP/donor-repo/.git/megapowers-review-receipt.json" \
   "$TMP/victim-repo/.git/megapowers-review-receipt.json"
check_got BLOCK "$(verdict_in "$TMP/victim-repo" "$(j false "$TR")")" \
  "a receipt naming another worktree does not approve this one"

# A relative artifact is portable by construction: it names whatever directory it
# happens to be read from, so it would clear the check in every repository at
# once. The launcher never writes one (delegate-run stores an absolute
# `git rev-parse --show-toplevel`), so rejecting it costs nothing real.
receipt_in "$TMP/victim-repo" "."
check_got BLOCK "$(verdict_in "$TMP/victim-repo" "$(j false "$TR")")" \
  "a relative receipt artifact does not approve any worktree"
# ...and a receipt written for this worktree still allows, so the check is not
# simply rejecting everything.
receipt_in "$TMP/victim-repo" "$TMP/victim-repo"
check_got ALLOW "$(verdict_in "$TMP/victim-repo" "$(j false "$TR")")" \
  "a corrected artifact on the same tree allows again"
# A symlinked spelling of the same worktree is the same worktree. Both sides are
# resolved before comparison, so an alias must not read as a mismatch.
ln -s "$TMP/victim-repo" "$TMP/victim-alias"
receipt_in "$TMP/victim-repo" "$TMP/victim-alias"
check_got ALLOW "$(verdict_in "$TMP/victim-repo" "$(j false "$TR")")" \
  "a symlinked spelling of the same worktree still allows"

# IDENTITY, NOT LOCATION. Binding the worktree path above says where a receipt was
# earned, not what it reviewed. A path is a slot that anything can occupy: move the
# reviewed repository away, drop an unrelated one at the same path, hand it the
# receipt, and the artifact matches, the fingerprint matches (the pending delta is
# byte identical), and nothing about the code is the same. What separates them is
# the base commit the delta applies to, which is why the receipt records it.
slot="$TMP/slot"
portable_repo "$TMP/slot-src-a" alpha
portable_repo "$TMP/slot-src-b" beta
mv "$TMP/slot-src-a" "$slot"
receipt_in "$slot" "$slot"
check_got ALLOW "$(verdict_in "$slot" "$(j false "$TR")")" \
  "a receipt earned in the repository at this path allows"
cp "$slot/.git/megapowers-review-receipt.json" "$TMP/slot-receipt.json"
mv "$slot" "$TMP/slot-parked-a"
mv "$TMP/slot-src-b" "$slot"
cp "$TMP/slot-receipt.json" "$slot/.git/megapowers-review-receipt.json"
# Both premises asserted rather than assumed. If either stops holding, the case
# below is no longer reproducing the substitution it is named for.
check_got "$(cd "$TMP/slot-parked-a" && "$DIFF_ID")" "$(cd "$slot" && "$DIFF_ID")" \
  "the substituted repository fingerprints identically to the reviewed one"
check_got "$slot" "$(jq -r '.subject.artifact' "$slot/.git/megapowers-review-receipt.json")" \
  "the substituted repository sits at exactly the path the receipt names"
check_got BLOCK "$(verdict_in "$slot" "$(j false "$TR")")" \
  "an unrelated repository at the reviewed path does not inherit the receipt"

# The same binding from the other side: one repository, one unchanged pending
# delta, a rewritten base. `--amend -m` moves HEAD without touching the tree, so
# the fingerprint cannot see it and only the recorded base can.
receipt_in "$slot" "$slot"
check_got ALLOW "$(verdict_in "$slot" "$(j false "$TR")")" \
  "a receipt naming the current base allows"
amend_id_before="$(cd "$slot" && "$DIFF_ID")"
(cd "$slot" && git commit -q --amend -m "rewritten history")
check_got "$amend_id_before" "$(cd "$slot" && "$DIFF_ID")" \
  "rewriting the base leaves the pending delta fingerprint unchanged"
check_got BLOCK "$(verdict_in "$slot" "$(j false "$TR")")" \
  "a receipt does not survive a rewritten base"

# v1 RECEIPTS ARE REJECTED. v1 records no base, so it cannot say which tree state
# its reviewer read and it clears any checkout carrying the same delta. There is no
# way to recover the missing base after the fact, so accepting one would keep the
# hole open. Derived by downgrading a receipt that allows, so the schema marker and
# the absent base are the only differences between ALLOW and BLOCK here.
receipt_in "$slot" "$slot"
check_got ALLOW "$(verdict_in "$slot" "$(j false "$TR")")" \
  "the v2 form of this receipt allows"
jq -c '.schema = "megapowers.review-receipt.v1" | del(.subject.base)' \
  "$slot/.git/megapowers-review-receipt.json" > "$TMP/downgraded.json"
cp "$TMP/downgraded.json" "$slot/.git/megapowers-review-receipt.json"
check_got BLOCK "$(verdict_in "$slot" "$(j false "$TR")")" \
  "an otherwise identical v1 receipt does not allow"
# A v2 receipt with the base merely deleted must not pass either, so the rejection
# is about the missing binding rather than about the version string alone.
jq -c 'del(.subject.base)' "$TMP/slot-receipt.json" > "$TMP/baseless.json"
jq -c --arg root "$slot" '.subject.artifact = $root' "$TMP/baseless.json" \
  > "$slot/.git/megapowers-review-receipt.json"
check_got BLOCK "$(verdict_in "$slot" "$(j false "$TR")")" \
  "a v2 receipt with no base does not allow"
# A receipt that quietly stops working reads as a gate bug and invites routing
# around it, so the block has to name the version and the reason.
cp "$TMP/downgraded.json" "$slot/.git/megapowers-review-receipt.json"
legacy_reason="$(reason_in "$slot" "$(j false "$TR")")"
case "$legacy_reason" in
  *"review-receipt.v1"*base*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a rejected v1 receipt must be named in the block reason :: %s\n' "$legacy_reason" ;;
esac

# `git ls-files --others` QUOTES a name carrying a newline, tab, quote, or a
# non-ASCII byte under the default core.quotePath, and that display form names no
# file on disk: the `[ -f ]` test fails, the file is never opened, and risky
# content inside it is never read. None of the metacharacters in this name is a
# risky token, so a block can only come from the file's CONTENT being scanned.
quoted_repo="$TMP/quoted-name-repo"
mkdir -p "$quoted_repo"
(
  cd "$quoted_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  git commit -q --allow-empty -m init
)
quoted_name=$'we\nir\td "q"\xc3\xa9na.go'
printf 'func chargeCard() { stripe() }\n' > "$quoted_repo/$quoted_name"
check_got BLOCK "$(verdict_in "$quoted_repo" "$(j false "$TR")")" \
  "a quoted untracked filename does not hide risky content"
printf 'ordinary notes\n' > "$quoted_repo/$quoted_name"
check_got ALLOW "$(verdict_in "$quoted_repo" "$(j false "$TR")")" \
  "a quoted untracked filename alone is not a false gate demand"

# Every regular untracked file is opened. A bound on a security scan means the
# path past it carries risky content through in silence, and the path is chosen
# by whoever adds the file. The risky name here sorts last, so it lands well past
# the old fifty-path cap, and the name itself carries no risky token.
late_repo="$TMP/late-untracked-repo"
mkdir -p "$late_repo"
(
  cd "$late_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  git commit -q --allow-empty -m init
)
for i in $(seq -w 0 59); do printf 'ordinary notes\n' > "$late_repo/f$i.txt"; done
printf 'func chargeCard() { stripe() }\n' > "$late_repo/zzz-late.go"
check_got BLOCK "$(verdict_in "$late_repo" "$(j false "$TR")")" \
  "risky content past the sixtieth untracked path is still scanned"
printf 'ordinary notes\n' > "$late_repo/zzz-late.go"
check_got ALLOW "$(verdict_in "$late_repo" "$(j false "$TR")")" \
  "sixty benign untracked files do not gate"

# The third way untracked content goes unread: a regular file the scan cannot
# open. grep writes an error to a stream nobody reads and reports no match, so
# unknown content is indistinguishable from benign content and passes.
#
# UNREADABLE IS UNSCANNABLE, AND ONE RULE GOVERNS ALL UNSCANNABLE CONTENT. This
# leg blocked outright while the binary leg had already been corrected to announce,
# so the same tree was judged two ways depending on which door the unread bytes
# came through: an ordinary mode-000 notes.md demanded a full cross-vendor review
# with no risky word and no risky path anywhere in it. THE ASSERTIONS BELOW WERE
# INVERTED FROM BLOCK TO ALLOW, and each one grew a check that the notice names the
# path, because an ALLOW alone would also pass over the silence that started all of
# this.
if [ "$(id -u)" != 0 ]; then
  locked_repo="$TMP/locked-untracked-repo"
  mkdir -p "$locked_repo"
  (
    cd "$locked_repo" || exit 1
    git init -q
    git config user.email test@example.com
    git config user.name test
    git config commit.gpgsign false
    git commit -q --allow-empty -m init
  )
  printf 'func chargeCard() { stripe() }\n' > "$locked_repo/secret.go"
  chmod 000 "$locked_repo/secret.go"
  check_got ALLOW "$(verdict_in "$locked_repo" "$(j false "$TR")")" \
    "an untracked file the scan cannot open does not demand a review on its own"
  locked_notice="$(notice_in "$locked_repo" "$(j false "$TR")")"
  says_got secret.go "$locked_notice" "the notice must name the unreadable path"
  says_got "could not be opened at all" "$locked_notice" \
    "the notice must say the gate never read the file, not that it read undecided bytes"
  chmod 644 "$locked_repo/secret.go"
  rm -f "$locked_repo/secret.go"
  # ...and the exclusion must not narrow that rule. exclude_globs says prose does
  # not execute, which is a claim about content the gate could READ. A file it
  # cannot open is not known to be prose, only known to be unread, and the name is
  # picked by whoever adds the file: applying the exclusion first lets `notes.md`
  # at mode 000 go unmentioned while the same bytes as `notes.go` are announced.
  # The ordering property is now visible in the NOTICE rather than in a block, so
  # it needs the notice asserted or an ALLOW would satisfy it either way.
  printf 'ordinary notes\n' > "$locked_repo/notes.md"
  chmod 000 "$locked_repo/notes.md"
  check_got ALLOW "$(verdict_in "$locked_repo" "$(j false "$TR")")" \
    "an unreadable untracked file with an excluded name is not a review demand"
  says_got notes.md "$(notice_in "$locked_repo" "$(j false "$TR")")" \
    "an unreadable untracked file with an excluded name is not passed in silence"
  # ...and the exclusion still spares a file the gate CAN read, so the case above
  # is about the unread content rather than about the extension. It says NOTHING
  # either, so the notice is about content that went unread rather than about the
  # repository holding excluded files at all.
  chmod 644 "$locked_repo/notes.md"
  check_got ALLOW "$(verdict_in "$locked_repo" "$(j false "$TR")")" \
    "a readable excluded untracked file still allows"
  check_got "" "$(notice_in "$locked_repo" "$(j false "$TR")")" \
    "a readable excluded untracked file produces no notice"
  rm -f "$locked_repo/notes.md"
  # THE PATH IS THE ONE SIGNAL BYTES NOBODY CAN READ STILL LEAVE, exactly as for a
  # binary. This is the pairing that makes the ALLOW cases above mean something: it
  # is the content being unread AND the path being ordinary that spares them, not
  # unread alone.
  printf 'ordinary notes\n' > "$locked_repo/payment_processor.go"
  chmod 000 "$locked_repo/payment_processor.go"
  check_got BLOCK "$(verdict_in "$locked_repo" "$(j false "$TR")")" \
    "an unreadable untracked file whose path names a risky category still blocks"
  says_got payment_processor.go "$(reason_in "$locked_repo" "$(j false "$TR")")" \
    "the block must name the risky unreadable path"
  chmod 644 "$locked_repo/payment_processor.go"
  rm -f "$locked_repo/payment_processor.go"
  # ...and the exclusion must not spare THAT either, which is the same ordering
  # property read from the blocking side. An excluded name plus a mode the scan
  # cannot open would otherwise be a two-step exemption for a risky-named file,
  # bought with a rename.
  printf 'ordinary notes\n' > "$locked_repo/billing_notes.md"
  chmod 000 "$locked_repo/billing_notes.md"
  check_got BLOCK "$(verdict_in "$locked_repo" "$(j false "$TR")")" \
    "an unreadable risky-named file under an excluded name still blocks"
  says_got billing_notes.md "$(reason_in "$locked_repo" "$(j false "$TR")")" \
    "the block must name the risky unreadable path under the excluded name"
  chmod 644 "$locked_repo/billing_notes.md"
  rm -f "$locked_repo/billing_notes.md"
else
  printf '  SKIP as root: unreadable untracked files are announced or blocked by path (chmod 000 does not block root, 11 assertions)\n'
fi

# REPLACEMENT OBJECTS DEFEAT THE BASE BINDING. `git replace X Y` makes every
# object read return Y where X was asked for and moves NO ref, so
# `git rev-parse HEAD` still prints X while `git diff HEAD` renders against Y.
# The receipt binds subject.base to the HEAD ref OID, so a repository holding X
# plus a replacement to Y, with a worktree matching Y's tree, reproduces the
# approved delta byte for byte and satisfies artifact, base and id although its
# tracked content is one the reviewer never saw. The whole point of recording the
# base was that base plus delta determines the complete tracked content; a
# replacement object breaks exactly that.
replaced="$TMP/replaced-repo"
mkdir -p "$replaced"
(
  cd "$replaced" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  # The tree the reviewer is NOT shown, committed first so it can be replaced in.
  printf 'func chargeCard() { stripe(secret) }\n' > svc.go
  git add svc.go
  git commit -qm hidden
  # The tree the reviewer IS shown. HEAD stays here for the whole case.
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm reviewed
)
hidden_commit="$(cd "$replaced" && git rev-parse HEAD~1)"
head_commit="$(cd "$replaced" && git rev-parse HEAD)"
# The pending delta: one untracked file, identical on both sides of the swap. It
# carries the risky token, so the gate reaches the receipt check at all.
printf 'func billing() { invoice() }\n' > "$replaced/note.go"
check_got BLOCK "$(verdict_in "$replaced" "$(j false "$TR")")" \
  "the reviewed tree gates before a receipt exists"
receipt_in "$replaced" "$replaced"
check_got ALLOW "$(verdict_in "$replaced" "$(j false "$TR")")" \
  "a receipt earned on the reviewed tree allows"
reviewed_id="$(cd "$replaced" && "$DIFF_ID")"

# Now the substitution, in place: same path, same receipt, same HEAD ref OID, same
# untracked delta. Only the effective base moves, through refs/replace, and the
# worktree is set to match the replaced base so the pending delta stays identical
# under a replacement-aware read.
(
  cd "$replaced" || exit 1
  git replace "$head_commit" "$hidden_commit"
  printf 'func chargeCard() { stripe(secret) }\n' > svc.go
)
check_got "$head_commit" "$(cd "$replaced" && git rev-parse HEAD)" \
  "the substituted tree still resolves HEAD to the OID the receipt binds"
# Both premises asserted rather than assumed. If either stops holding, the case
# below is no longer reproducing the substitution it is named for.
check_got "" "$(cd "$replaced" && git diff HEAD)" \
  "a replacement-aware read sees the same empty tracked delta the receipt covered"
check_got "yes" \
  "$(cd "$replaced" && [ -n "$(git --no-replace-objects diff HEAD)" ] && echo yes || echo no)" \
  "the real objects carry a tracked change the reviewer never saw"
check_got "yes" \
  "$([ "$reviewed_id" != "$(cd "$replaced" && "$DIFF_ID")" ] && echo yes || echo no)" \
  "the fingerprint reads the real objects, so the substituted tree does not match"
check_got BLOCK "$(verdict_in "$replaced" "$(j false "$TR")")" \
  "a replacement object does not carry a receipt onto a tree nobody reviewed"
# ...and the block is not simply "any repository with a replace ref": drop the
# replacement, restore the reviewed content, and the same receipt allows again.
(
  cd "$replaced" || exit 1
  git replace -d "$head_commit" >/dev/null 2>&1
  printf 'func handler() {}\n' > svc.go
)
check_got ALLOW "$(verdict_in "$replaced" "$(j false "$TR")")" \
  "the receipt still allows once the real tree is back"

# A SUBMODULE DIFFS AS A FLAG, NOT AS CONTENT. A gitlink renders as a bare
# `Subproject commit <sha>` line, so at a neutral path like vendor/lib the risky
# scan matches nothing and the gate exits clean over a pointer bump that pulled in
# arbitrary code. A manually dispatched reviewer gets the same SHAs.
sup="$TMP/superproject"
subsrc="$TMP/submodule-src"
mkdir -p "$subsrc"
(
  cd "$subsrc" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func helper() {}\n' > lib.go
  git add lib.go
  git commit -qm clean
  printf 'func chargeCard() { stripe(secret) }\n' > lib.go
  git add lib.go
  git commit -qm risky
)
sub_clean="$(cd "$subsrc" && git rev-parse HEAD~1)"
sub_risky="$(cd "$subsrc" && git rev-parse HEAD)"
mkdir -p "$sup"
(
  cd "$sup" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  # NO `diff.ignoreSubmodules` here. Under the DEFAULT configuration git omits an
  # untracked-only dirty submodule from every diff, which is the weakest case and
  # the one this gate has to handle. Setting the variable would make these cases
  # pass against a stricter git than anyone actually runs.
  git -c protocol.file.allow=always submodule add -q "$subsrc" vendor/lib
  (cd vendor/lib && git checkout -q "$sub_clean")
  git add -A
  git commit -qm init
)
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "a clean submodule is not a gate demand"
# The premise: the pointer bump carries risky code and yet shows the superproject
# nothing but SHAs, which is why the risky scan alone cannot see it.
(cd "$sup/vendor/lib" && git checkout -q "$sub_risky")
check_got "" "$(cd "$sup" && git diff HEAD | grep -iE 'stripe|chargeCard' || true)" \
  "the superproject diff of a pointer bump carries no risky token"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "a submodule pointer bump reaches the gate"
receipt_in "$sup" "$sup"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "a receipt binding the submodule snapshot allows"
# Content inside the submodule. A dirty submodule appends `-dirty` to the pointer
# line, which is a FLAG rather than content: every distinct dirty content renders
# the same superproject diff, so the subject id cannot tell two of them apart and
# only the supplemental binding can.
printf 'func chargeCard() { stripe(one) }\n' > "$sup/vendor/lib/lib.go"
receipt_in "$sup" "$sup"
dirty_id_one="$(cd "$sup" && "$DIFF_ID")"
dirty_sub_one="$(cd "$sup" && "$DIFF_ID" --submodules)"
printf 'func chargeCard() { stripe(two) }\n' > "$sup/vendor/lib/lib.go"
check_got "$dirty_id_one" "$(cd "$sup" && "$DIFF_ID")" \
  "two different dirty submodule contents share one subject id"
check_got "yes" \
  "$([ "$dirty_sub_one" != "$(cd "$sup" && "$DIFF_ID" --submodules)" ] && echo yes || echo no)" \
  "the supplemental submodule fingerprint separates them"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "swapping submodule content under an unchanged subject id stales the receipt"
printf 'func chargeCard() { stripe(one) }\n' > "$sup/vendor/lib/lib.go"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "restoring the reviewed submodule content restores the receipt"
# An untracked file inside a submodule leaves the superproject diff BYTE-IDENTICAL:
# it contributes no hunk of its own and the pointer already carries `-dirty`.
sup_diff_before="$(cd "$sup" && git diff HEAD -- vendor/lib)"
printf 'func billing() { invoice() }\n' > "$sup/vendor/lib/dropped.go"
check_got "$sup_diff_before" "$(cd "$sup" && git diff HEAD -- vendor/lib)" \
  "an untracked file inside a submodule leaves the superproject diff unchanged"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "an untracked file inside a submodule stales the receipt"
rm -f "$sup/vendor/lib/dropped.go"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "removing it restores the receipt"

# The same, from a fully clean submodule, where git omits the dirt from the diff
# ENTIRELY rather than flagging it. This is the default configuration, so it is
# what the gate meets in the field: nothing reaches the superproject diff, and
# only asking the submodule itself finds it.
(cd "$sup/vendor/lib" && git checkout -q -- lib.go && git checkout -q "$sub_clean")
(cd "$sup" && git add -A && git commit -qm "settle the pointer") >/dev/null 2>&1
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "a settled clean submodule stops clean"
printf 'func chargeCard() { stripe(hidden) }\n' > "$sup/vendor/lib/dropped.go"
check_got "" "$(cd "$sup" && git diff HEAD)$(cd "$sup" && git diff --cached)" \
  "test premise: an untracked file in a clean submodule reaches no superproject diff at all"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "an untracked file in an otherwise clean submodule still reaches the gate"
rm -f "$sup/vendor/lib/dropped.go"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "removing it stops clean again"

# `submodule.<name>.ignore = all` is a setting the REVIEWED repository ships, and
# it removes the submodule from every diff including the pointer bump. What this
# gate reads must not be the reviewed repository's choice.
(cd "$sup" && git config submodule.vendor/lib.ignore all)
(cd "$sup/vendor/lib" && git checkout -q "$sub_risky")
check_got "" "$(cd "$sup" && git diff HEAD)" \
  "test premise: 'ignore = all' hides the pointer bump from git diff"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "'ignore = all' does not hide a submodule pointer bump from the gate"
(cd "$sup" && git config --unset submodule.vendor/lib.ignore)
(cd "$sup/vendor/lib" && git checkout -q "$sub_clean")
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "restoring the pointer stops clean again"

# git refuses to look inside a gitlink path at all, so an UNINITIALIZED submodule
# directory is a place the superproject's own untracked scan never descends into.
# Empty is the ordinary case and must stay silent; occupied is content that
# reaches no diff, no status and no untracked listing.
mv "$sup/vendor/lib" "$TMP/parked-lib"
mkdir "$sup/vendor/lib"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "an empty uninitialized submodule is not a gate demand"
printf 'func chargeCard() { stripe(secret) }\n' > "$sup/vendor/lib/dropped.go"
check_got "" "$(cd "$sup" && git status --porcelain)$(cd "$sup" && git ls-files --others --exclude-standard)" \
  "test premise: content in an uninitialized submodule reaches no git read at all"
check_got BLOCK "$(verdict_in "$sup" "$(j false "$TR")")" \
  "content in an uninitialized submodule directory still reaches the gate"
rm -f "$sup/vendor/lib/dropped.go"
check_got ALLOW "$(verdict_in "$sup" "$(j false "$TR")")" \
  "emptying it stops clean again"
rmdir "$sup/vendor/lib"
mv "$TMP/parked-lib" "$sup/vendor/lib"

# THE INDEX CAN HIDE A TRACKED EDIT FROM EVERYTHING. `assume-unchanged` tells git
# to trust the index entry and stop stat'ing the worktree, so the edit lands in no
# diff, no status, no risky scan and no fingerprint: an existing receipt survives
# a tracked-file change nobody read.
au="$TMP/assume-unchanged-repo"
mkdir -p "$au"
(
  cd "$au" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() { billing() }\n' > svc.go
  git add svc.go
  git commit -qm init
)
printf 'func billing() { invoice() }\n' > "$au/note.go"
check_got BLOCK "$(verdict_in "$au" "$(j false "$TR")")" \
  "the risky pending change gates before a receipt exists"
receipt_in "$au" "$au"
check_got ALLOW "$(verdict_in "$au" "$(j false "$TR")")" \
  "the reviewed risky change allows"
# The bit alone hides nothing while the content still matches the index, and a
# gate that fires on an unused feature is a gate that gets worked around. The bit
# is set over a path with no pending modification, which is how the feature is
# actually used: setting it over an ALREADY divergent path hides that divergence
# and is reported, which the next case relies on.
(cd "$au" && git update-index --assume-unchanged svc.go)
check_got ALLOW "$(verdict_in "$au" "$(j false "$TR")")" \
  "setting assume-unchanged over unchanged content is not a gate demand"
printf 'func handler() { stripe(secret) }\n' > "$au/svc.go"
check_got "" "$(cd "$au" && git diff HEAD)$(cd "$au" && git status --porcelain -- svc.go)" \
  "an assume-unchanged edit is invisible to diff and status"
check_got "" "$(cd "$au" && "$DIFF_ID" 2>/dev/null)" \
  "the fingerprint prints nothing rather than reporting the tree as unchanged"
check_got "yes" \
  "$(cd "$au" && "$DIFF_ID" >/dev/null 2>&1 && echo no || echo yes)" \
  "the fingerprint fails rather than handing back the pre-edit id"
check_got BLOCK "$(verdict_in "$au" "$(j false "$TR")")" \
  "an assume-unchanged edit does not ride an existing receipt"
au_reason="$(reason_in "$au" "$(j false "$TR")")"
case "$au_reason" in
  *assume-unchanged*svc.go*|*svc.go*assume-unchanged*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL the block must name the bit and the path :: %s\n' "$au_reason" ;;
esac
(cd "$au" && git update-index --no-assume-unchanged svc.go)
# Clearing the bit exposes a real content change, so the edit was never a no-op:
# what the bit hid was an unreviewed rewrite of a tracked file.
check_got "yes" \
  "$(cd "$au" && git diff HEAD -- svc.go | grep -q 'stripe(secret)' && echo yes || echo no)" \
  "clearing the bit exposes the hidden risky rewrite"
check_got BLOCK "$(verdict_in "$au" "$(j false "$TR")")" \
  "the exposed edit still gates on its own merits"

# SKIP-WORKTREE IS A DIFFERENT BIT AND STAYS SUPPORTED. Sparse checkout sets it on
# every path outside the cone, so refusing it with assume-unchanged would block
# every stop in a sparse checkout. `git ls-files -v` separates them: lowercase for
# assume-unchanged, uppercase 'S' for skip-worktree.
sw="$TMP/skip-worktree-repo"
mkdir -p "$sw"
(
  cd "$sw" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  printf 'outside the cone\n' > far.txt
  git add svc.go far.txt
  git commit -qm init
  git update-index --skip-worktree far.txt
)
check_got ALLOW "$(verdict_in "$sw" "$(j false "$TR")")" \
  "a skip-worktree path present and matching the index is not a gate demand"
(cd "$sw" && git config core.sparseCheckout true && rm far.txt)
check_got ALLOW "$(verdict_in "$sw" "$(j false "$TR")")" \
  "a sparse checkout that removed the path from disk still stops clean"
# ...but a skip-worktree path that IS materialized and changed hides the same edit
# assume-unchanged does, so content is what decides.
(cd "$sw" && git config --unset core.sparseCheckout)
printf 'func chargeCard() { stripe(secret) }\n' > "$sw/far.txt"
check_got "" "$(cd "$sw" && git diff HEAD)" \
  "a materialized skip-worktree edit is invisible to diff"
check_got BLOCK "$(verdict_in "$sw" "$(j false "$TR")")" \
  "a materialized changed skip-worktree path is not passed in silence"
(cd "$sw" && printf 'outside the cone\n' > far.txt)
check_got ALLOW "$(verdict_in "$sw" "$(j false "$TR")")" \
  "restoring the content stops clean again"

# THE INDEX IS PART OF PENDING. `git diff HEAD` renders HEAD against the WORKTREE
# and says nothing about what is staged, so staging risky content and then
# restoring the worktree copy to its committed bytes made the two cancel: the gate
# read an empty diff, exited clean, and the risky content sat in the index one
# plain `git commit` from shipping. A `--cached` check had been added for the
# policy file alone, which closed one instance and left the general hole open.
index_repo="$TMP/index-repo"
mkdir -p "$index_repo"
(
  cd "$index_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  printf 'ordinary notes\n' > other.txt
  git add -A
  git commit -qm init
)
# THE CONTROL FIRST, so every block below is the staged content and not the
# fixture: index and worktree both clean, and nothing untracked.
check_got ALLOW "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "a repository with a clean index and a clean worktree still stops clean"
printf 'func handler() { billing(charge) }\n' > "$index_repo/svc.go"
(cd "$index_repo" && git add svc.go)
check_got BLOCK "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "a staged risky change is pending and gates"
# THE MASKED CASE. Same index, worktree returned to its committed bytes.
(cd "$index_repo" && git show HEAD:svc.go > svc.go)
check_got "" "$(cd "$index_repo" && git diff HEAD)" \
  "test premise: HEAD against the worktree is empty while the index holds the change"
check_got BLOCK "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "a staged risky change masked by a restored worktree copy is not a clean tree"
# ...and a non-empty worktree hop is not what saves it either. With an unrelated
# benign edit alongside, the old gate computed a diff that was not empty at all,
# scanned it clean, and allowed: the masking is about WHICH tree is read, not about
# there being nothing to read.
printf 'more ordinary notes\n' > "$index_repo/other.txt"
check_got BLOCK "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "an unrelated unstaged edit does not unmask or excuse the staged risky change"
(cd "$index_repo" && git checkout -q -- other.txt)
# Both hops carrying risk at once, which is the ordinary shape of an edit made
# after `git add`.
printf 'func handler() { billing(charge) }\nfunc settle() { stripe() }\n' > "$index_repo/svc.go"
check_got BLOCK "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "a staged risky change edited further in the worktree gates"
# ...and clearing BOTH hops clears the gate, so the union is a union and not a
# latch that never lets go.
(cd "$index_repo" && git reset -q HEAD -- svc.go && git checkout -q -- svc.go)
check_got ALLOW "$(verdict_in "$index_repo" "$(j false "$TR")")" \
  "unstaging the change and restoring the worktree stops clean again"

# THE EXCLUSION PATHSPEC CAN BE TOO LONG TO RUN. Excluded paths are dropped from a
# second diff by handing git one `:(top,exclude,literal)` operand per excluded
# changed path, and that list is unbounded. Past the execve argument limit the
# command never runs and git writes nothing, which is indistinguishable from a
# clean tree: the risky tracked line below then ships with the hook emitting
# nothing at all. The paths are long rather than numerous so the fixture costs a
# couple of seconds instead of megabytes of content.
#
# Last, because the limit is min(ARG_MAX, RLIMIT_STACK/4) on Linux and the fixture
# is sized against the ordinary 8MB stack. Pinning it here keeps the case honest
# on a runner that inherited an unlimited stack, where the ceiling would be 6MB.
ulimit -s 8192 2>/dev/null || true
argmax_repo="$TMP/argmax-repo"
mkdir -p "$argmax_repo"
(
  cd "$argmax_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  seg="$(printf '%0250d' 0 | tr 0 d)"
  deep="$seg/$seg/$seg/$seg/$seg/$seg/$seg/$seg/$seg/$seg/$seg"
  mkdir -p "$deep"
  # ~2.8KB per pathspec operand, so a thousand excluded paths is ~2.8MB of argv
  # against a 2MB ceiling. Committed first, because only a CHANGED path reaches
  # the drop list.
  for i in $(seq 1 1000); do printf 'notes\n' > "$deep/f$i.md"; done
  git add -A
  git commit -qm init
  for i in $(seq 1 1000); do printf 'more notes\n' > "$deep/f$i.md"; done
)
# The premise, asserted rather than assumed: the fixture really does exceed the
# limit. Without this a runner with a bigger ceiling would compute the reduced
# diff, block for the ordinary reason, and report a green case that exercised
# nothing.
argmax_drop_runs() {
  local -a spec=()
  local p
  while IFS= read -r -d '' p; do
    case "$p" in *.md) spec+=(":(top,exclude,literal)$p") ;; esac
  done < <(cd "$argmax_repo" && git diff HEAD --name-only -z 2>/dev/null)
  (cd "$argmax_repo" && git diff HEAD --no-ext-diff -- ':/' "${spec[@]}" >/dev/null 2>&1) \
    && echo ran || echo failed
}
check_got failed "$(argmax_drop_runs)" \
  "test premise: a thousand excluded paths exceed the execve argument limit"
# Benign first: the fallback scans the excluded prose again, so it must not gate on
# the markdown alone or the block below says nothing about svc.go.
check_got ALLOW "$(verdict_in "$argmax_repo" "$(j false "$TR")")" \
  "a thousand changed excluded paths alone are not a gate demand"
printf 'func h() { chargeStripe() }\n' > "$argmax_repo/svc.go"
check_got BLOCK "$(verdict_in "$argmax_repo" "$(j false "$TR")")" \
  "a risky tracked line still gates when the exclusion pathspec cannot be run"

# SCRATCH FILES ARE REMOVED FROM A TRAP, NOT AFTER THEIR LAST READ. Every temp
# file was freed inline, which leaves one behind in TMPDIR whenever the hook stops
# before reaching that line, and an interrupted stop is exactly that. The two
# holding the index enumeration were the ones the hook's own comment claimed were
# covered and were not, so each temp gets a case of its own here: a claim about
# EVERY scratch file is only worth what its least covered file is worth.
#
# The signal has to arrive while the scratch file is on disk, so the fixture does
# not guess at the timing: a `git` shim on PATH hangs one named call, which is a
# call that runs with the file already created. The hook is signalled there, the
# shim is released, and what is left in a private TMPDIR is the answer.
leak_dir="$TMP/leak"
shim_dir="$TMP/shim"
mkdir -p "$shim_dir"
real_git="$(command -v git)"
cat > "$shim_dir/git" <<EOF
#!/usr/bin/env bash
# WHICH call hangs is per case, because the temps are made at different points and
# only a call running with the file already on disk proves anything. The index
# enumerations are matched on the whole argument line, and the two forms differ by
# \`-z\` alone: the prefilter listing is live while index_lines is, the NUL form
# while index_list is.
hang=0
case "\${LEAK_HANG:-show}" in
  # Only an INDEX read hangs. \`git show HEAD:<path>\` runs earlier and must not.
  show) [ "\${1:-}" = show ] && [ "\${2#:}" != "\${2:-}" ] && hang=1 ;;
  index) case "\$*" in *"ls-files -s -v --full-name"*) hang=1 ;; esac ;;
  indexz) case "\$*" in *"ls-files -s -v -z --full-name"*) hang=1 ;; esac ;;
esac
if [ "\$hang" = 1 ]; then
  : > "\$LEAK_MARK"
  i=0
  while [ ! -e "\$LEAK_RELEASE" ] && [ "\$i" -lt 200 ]; do sleep 0.05; i=\$((i + 1)); done
fi
exec "$real_git" "\$@"
EOF
chmod +x "$shim_dir/git"
leak_probe() {
  local dir="$1" hang="${2:-show}" pid i=0
  rm -rf "$leak_dir" "$TMP/leak-mark" "$TMP/leak-release"
  mkdir -p "$leak_dir"
  # `exec` inside `bash -c`, so the pid `$!` reports is the hook itself and the
  # signal below is not delivered to a wrapper the hook never hears about.
  printf '%s' "$(j false "$TR")" | env PATH="$shim_dir:$PATH" TMPDIR="$leak_dir" \
    LEAK_MARK="$TMP/leak-mark" LEAK_RELEASE="$TMP/leak-release" LEAK_HANG="$hang" \
    bash -c 'cd "$1" && exec bash "$2"' _ "$dir" "$HOOK" >/dev/null 2>&1 &
  pid=$!
  while [ ! -e "$TMP/leak-mark" ] && [ "$i" -lt 200 ]; do sleep 0.05; i=$((i + 1)); done
  if [ ! -e "$TMP/leak-mark" ]; then
    : > "$TMP/leak-release"
    wait "$pid" 2>/dev/null
    echo "FIXTURE-ERROR(the hook never made the $hang call, so no temp file existed)"
    return
  fi
  # Bash holds a signal until the foreground command returns, so the release is what
  # actually runs the trap. Killing without it would only prove the shim hangs.
  kill -TERM "$pid" 2>/dev/null
  : > "$TMP/leak-release"
  wait "$pid" 2>/dev/null
  printf '%s' "$(ls -A "$leak_dir" 2>/dev/null | tr '\n' ' ')"
}

# The staged-blob temp: a staged binary whose worktree copy is gone is classified
# from the index, which materializes the blob first.
leak_blob="$TMP/leak-blob-repo"
mkdir -p "$leak_blob"
(
  cd "$leak_blob" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  git commit -q --allow-empty -m init
  printf 'v1\000\001\002data\n' > blob.bin
  git add blob.bin
  rm -f blob.bin
)
check_got "" "$(leak_probe "$leak_blob")" \
  "an interrupted stop leaves no staged-blob scratch file behind"

# The policy-layer temp: a staged edit whose worktree copy reads as the committed
# one is read from the index so the notice can say which direction it goes.
leak_policy="$TMP/leak-policy-repo"
mkdir -p "$leak_policy"
(
  cd "$leak_policy" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  mkdir -p .megapowers
  printf '[rules.risky-logic-review]\nstate = "enforced"\n' > .megapowers/enforcement.toml
  git add .megapowers/enforcement.toml
  git commit -qm rules
  printf '[rules.risky-logic-review]\nstate = "off"\n' > .megapowers/enforcement.toml
  git add .megapowers/enforcement.toml
  git show HEAD:.megapowers/enforcement.toml > .megapowers/enforcement.toml
)
check_got "" "$(leak_probe "$leak_policy")" \
  "an interrupted stop leaves no policy-layer scratch file behind"

# The two INDEX enumeration temps, which the trap did not know about at all. Both
# are made immediately before their `git ls-files` call, so hanging that call
# catches each one on disk. One fixture serves both: the prefilter has to find
# something for the NUL listing to be made at all, and an assume-unchanged path is
# what makes it look.
leak_index="$TMP/leak-index-repo"
mkdir -p "$leak_index"
(
  cd "$leak_index" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  git update-index --assume-unchanged svc.go
  printf 'func handler() { billing() }\n' > svc.go
)
check_got "" "$(leak_probe "$leak_index" index)" \
  "an interrupted stop leaves no index-prefilter scratch file behind"
check_got "" "$(leak_probe "$leak_index" indexz)" \
  "an interrupted stop leaves no index-listing scratch file behind"

# --- the block must not eat the deliverable -----------------------------------
# On both harnesses the agent's reply to this block is what the user or the
# dispatching session reads instead of the answer it just wrote (a Codex Stop
# hook lands as a prompt after the final message). The reason has to say both
# things: carry the answer forward, and a dispatched reviewer reports rather
# than satisfies.
deliver_repo="$TMP/deliver-repo"
mkdir -p "$deliver_repo"
(
  cd "$deliver_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'func handler() { billing() }\n' > svc.go
)
deliver_reason="$(reason_in "$deliver_repo" "$(j false "$TR")")"
says_got "restate that reply" "$deliver_reason" \
  "the block tells the agent to carry its answer forward"
says_got "dispatched only to review" "$deliver_reason" \
  "the block tells a dispatched reviewer the receipt is not its to obtain"

# --- the cap-aware remedy ------------------------------------------------------
# When the round ledger already sits at the cap for this branch, prescribing the
# launcher prescribes a command that exits 10: gate and launcher would pin the
# session between them. The remedy switches to the human hand-off instead.
cap_repo="$TMP/cap-repo"
mkdir -p "$cap_repo"
(
  cd "$cap_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  git checkout -qb capbranch
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'func handler() { billing() }\n' > svc.go
  ledger="$(git rev-parse --git-path megapowers-review-rounds.json)"
  jq -nc '{schema:"megapowers.review-rounds.v2",
           rounds:{verify:{capbranch:3}},
           open:{verify:{capbranch:[]}},
           unapproved:{verify:{capbranch:true}},
           updated_at:"2026-08-15T00:00:00Z"}' > "$ledger"
)
cap_reason="$(reason_in "$cap_repo" "$(j false "$TR")")"
says_got "round cap" "$cap_reason" \
  "at the cap the reason names the exhausted review loop"
case "$cap_reason" in
  *"delegate-run --role verify"*)
    fail=$((fail + 1))
    printf '  FAIL at the cap the reason must not prescribe the launcher :: %s\n' "$cap_reason" ;;
  *) pass=$((pass + 1)) ;;
esac
below_reason="$(reason_in "$deliver_repo" "$(j false "$TR")")"
says_got "delegate-run" "$below_reason" \
  "below the cap the launcher remedy stays"
# The remedy prescribes --role verify, so only the verify ledger can exhaust
# it: a capped code_review loop with verify rounds remaining keeps the
# launcher remedy (cross-vendor review round 1 caught the conflation).
mixed_repo="$TMP/mixed-cap-repo"
mkdir -p "$mixed_repo"
(
  cd "$mixed_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  git checkout -qb mixedbranch
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'func handler() { billing() }\n' > svc.go
  ledger="$(git rev-parse --git-path megapowers-review-rounds.json)"
  jq -nc '{schema:"megapowers.review-rounds.v2",
           rounds:{code_review:{mixedbranch:3}},
           open:{code_review:{mixedbranch:[]}},
           unapproved:{code_review:{mixedbranch:true}},
           updated_at:"2026-08-15T00:00:00Z"}' > "$ledger"
)
mixed_reason="$(reason_in "$mixed_repo" "$(j false "$TR")")"
says_got "delegate-run" "$mixed_reason" \
  "a capped code_review loop does not exhaust the verify remedy"

# --- the human-recorded false-positive dismissal -------------------------------
# A confirmed false positive re-fires on every stop while the matched word sits
# in the pending diff. review-ack records the human's dismissal bound to the
# exact pending tree: the same tree passes with a notice, one more edit re-arms.
ACK="$HERE/../../skills/multi-agent-delegation/scripts/review-ack"
ack_repo="$TMP/ack-repo"
mkdir -p "$ack_repo"
(
  cd "$ack_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'func handler() { chargeWebhook() }\n' > svc.go
)
check_got BLOCK "$(verdict_in "$ack_repo" "$(j false "$TR")")" \
  "a risky diff blocks before any dismissal"
ack_reason="$(reason_in "$ack_repo" "$(j false "$TR")")"
says_got "review-ack" "$ack_reason" \
  "the false-positive sentence names the dismissal tool"
[ -x "$ACK" ] || { fail=$((fail + 1)); printf '  FAIL review-ack script exists and is executable\n'; }
ack_rc=0
(cd "$ack_repo" && "$ACK" --human "Ok. I authorize, the webhook word is a docs example" >/dev/null 2>&1) || ack_rc=$?
check_got 0 "$ack_rc" "review-ack records a dismissal for the current tree"
check_got ALLOW "$(verdict_in "$ack_repo" "$(j false "$TR")")" \
  "a dismissed tree stops blocking"
ack_notice="$(notice_in "$ack_repo" "$(j false "$TR")")"
says_got "dismissal" "$ack_notice" \
  "the pass over a dismissed tree is announced, not silent"
(cd "$ack_repo" && printf 'func handler() { chargeWebhook(); newEdit() }\n' > svc.go)
check_got BLOCK "$(verdict_in "$ack_repo" "$(j false "$TR")")" \
  "any further edit re-arms a dismissed gate"
ack_empty_rc=0
(cd "$ack_repo" && "$ACK" --human "" >/dev/null 2>&1) || ack_empty_rc=$?
check_got 2 "$ack_empty_rc" "review-ack refuses an empty human authorization"

# --- the unscanned-content notice does not repeat on an unchanged tree ---------
# Three identical notices per session about the same committed dist bundle is
# how a notice trains its reader to skip it. Same tree, same note: said once.
# The tree moving (or the note changing) says it again.
bin_repo="$TMP/binary-note-repo"
mkdir -p "$bin_repo"
(
  cd "$bin_repo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'data\x00blob\n' > asset.bin
)
first_note="$(notice_in_stateful "$bin_repo" "$(j false "$TR")")"
says_got "Unscanned content notice" "$first_note" \
  "an unscannable untracked file is announced on the first stop"
second_note="$(notice_in_stateful "$bin_repo" "$(j false "$TR")")"
case "$second_note" in
  *"Unscanned content notice"*)
    fail=$((fail + 1))
    printf '  FAIL the same unscanned note must not repeat on an unchanged tree :: %s\n' "$second_note" ;;
  *) pass=$((pass + 1)) ;;
esac
(cd "$bin_repo" && printf 'data\x00blob2\n' > asset.bin)
third_note="$(notice_in_stateful "$bin_repo" "$(j false "$TR")")"
says_got "Unscanned content notice" "$third_note" \
  "a changed tree re-announces the unscanned content"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
