#!/usr/bin/env bash
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN="$HERE/../sdd-run"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf 'FAIL %s\n' "$1"; }
expect_ok() { "$@" >/dev/null 2>&1 && ok || bad "expected success: $*"; }
expect_fail() { "$@" >/dev/null 2>&1 && bad "expected failure: $*" || ok; }
expect_eq() { [ "$1" = "$2" ] && ok || bad "expected '$1' = '$2'"; }

if grep -Eq '(^|[[:space:]])declare[[:space:]]+-A([[:space:]]|$)' "$RUN"; then
  bad "sdd-run requires Bash 4 associative arrays"
else
  ok
fi

repo="$TMP/repo"
git init -q "$repo"
git -C "$repo" config user.name Test
git -C "$repo" config user.email test@example.invalid
git -C "$repo" config commit.gpgsign false
printf 'base\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -qm 'test: seed'
git -C "$repo" switch -qc feature/multi-writer
cat > "$repo/plan.md" <<'PLAN'
# Fixture plan
### Task 1: Root A
**Blocked by:** None
**Parallel safety:** Safe
**Ownership:** `a/`
**May decompose:** Yes
PLAN
git -C "$repo" add plan.md
git -C "$repo" commit -qm 'test: add plan fixture'

cd "$repo" || exit 2
expect_fail "$RUN" init Bad_ID --plan plan.md --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
expect_fail "$RUN" init no-commits --plan plan.md --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8
expect_fail "$RUN" init duplicate-roots --plan plan.md --root root-a --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
git branch release
git switch -q release
expect_fail "$RUN" init protected --plan plan.md --root root-a --target release --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
git switch -q feature/multi-writer
expect_ok "$RUN" init demo --plan plan.md --root root-a --root root-b --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 9 --writers 3 --integrations 1 --allow-task-commits
expect_ok "$RUN" init defaults --plan plan.md --root default-root --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
defaults_base=refs/megapowers/runs/defaults
expect_eq "$(git cat-file blob "$defaults_base/manifest" | jq -r '.writer_limit')" 3
expect_eq "$(git cat-file blob "$defaults_base/manifest" | jq -r '.integration_limit')" 1

set +e
"$RUN" init race --plan plan.md --root race-a --target feature/multi-writer --session session-a --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits > "$TMP/race-a.out" 2>&1 & rp1=$!
"$RUN" init race --plan plan.md --root race-b --target feature/multi-writer --session session-b --harness claude --max-depth 2 --agent-budget 8 --allow-task-commits > "$TMP/race-b.out" 2>&1 & rp2=$!
wait "$rp1"; rr1=$?
wait "$rp2"; rr2=$?
set -e
[ $((rr1 == 0 ? 1 : 0)) -eq $((rr2 == 0 ? 0 : 1)) ] && ok || bad "run creation race did not produce one winner"
race_root=$(git cat-file blob refs/megapowers/runs/race/manifest | jq -r '.roots[0]')
race_owner=$(git cat-file blob refs/megapowers/runs/race/owner | jq -r '.session')
case "$race_root:$race_owner" in race-a:session-a|race-b:session-b) ok ;; *) bad "run race mixed manifest and owner" ;; esac

base=refs/megapowers/runs/demo
expect_eq "$(git for-each-ref --format='%(refname)' "$base" | wc -l | tr -d ' ')" 3
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.target_branch')" feature/multi-writer
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.roots | join(",")')" root-a,root-b
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.writer_limit')" 3
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.integration_limit')" 1
expect_eq "$(git cat-file blob "$base/owner" | jq -r '.session')" codex-1
git config --get-regexp '^remote\..*\.push$' 2>/dev/null | grep -q 'refs/megapowers' && bad "private refs entered a push refspec" || ok
expect_ok "$RUN" join demo --plan plan.md --target feature/multi-writer --harness claude
printf '\nchanged\n' >> plan.md
expect_fail "$RUN" join demo --plan plan.md --target feature/multi-writer --harness codex

git checkout -- plan.md
head=$(git rev-parse HEAD)
cat > brief.json <<EOF
{
  "version": 1,
  "run_id": "demo",
  "node": "root-a",
  "parent": null,
  "base": "$head",
  "branch": "mp/demo/nodes/root-a/head",
  "task": "Implement root A",
  "acceptance": ["test -f README.md"],
  "blocked_by": [],
  "parallel_safety": "Safe",
  "ownership": ["a/"],
  "may_decompose": true,
  "remaining_depth": 2,
  "descendant_budget": 7,
  "writer_budget": 3,
  "integration_budget": 1
}
EOF
expect_ok "$RUN" brief-put demo root-a brief.json --session codex-1
expect_fail "$RUN" brief-put demo root-a brief.json --session codex-1
expect_fail "$RUN" brief-put demo ../escape brief.json --session codex-1

set +e
"$RUN" claim demo root-a --session codex-a --harness codex > claim-a.out 2>&1 & p1=$!
"$RUN" claim demo root-a --session claude-b --harness claude > claim-b.out 2>&1 & p2=$!
wait "$p1"; r1=$?
wait "$p2"; r2=$?
set -e
[ $((r1 == 0 ? 1 : 0)) -eq $((r2 == 0 ? 0 : 1)) ] && ok || bad "claim race did not produce one winner"
claim_line=$(cat claim-a.out claim-b.out | grep -E '^[0-9a-f]+ ' | head -1)
claim_oid=${claim_line%% *}
claim_session=$(git cat-file blob "$claim_oid" | jq -r '.session')
new_oid=$("$RUN" heartbeat demo root-a --session "$claim_session" --expected "$claim_oid")
expect_fail "$RUN" release-claim demo root-a --session "$claim_session" --expected "$claim_oid"
expect_ok "$RUN" release-claim demo root-a --session "$claim_session" --expected "$new_oid"

jq '.node="root-b" | .branch="mp/demo/nodes/root-b/head" | .task="Implement root B" |
    .ownership=["b/"] | .may_decompose=false | .remaining_depth=0 |
    .descendant_budget=0 | .writer_budget=0 | .integration_budget=0' \
  brief.json > brief-b.json
expect_fail "$RUN" brief-put demo root-b brief-b.json --session intruder
expect_ok "$RUN" brief-put demo root-b brief-b.json --session codex-1
# shellcheck disable=SC2034
root_a_claim=$("$RUN" claim demo root-a --session codex-session --harness codex)
expect_fail "$RUN" claim demo root-b --session codex-session --harness codex
# shellcheck disable=SC2034
root_b_claim=$("$RUN" claim demo root-b --session claude-session --harness claude)
expect_fail "$RUN" claim demo root-a --session third-session --harness claude

git update-ref refs/heads/mp/demo/nodes/root-a/head "$head"

jq '.node="root-a/claim" | .parent="root-a" |
    .branch="mp/demo/nodes/root-a/claim/head" | .task="Reject reserved node" |
    .ownership=["a/reserved/"] | .may_decompose=false | .remaining_depth=0 |
    .descendant_budget=0 | .writer_budget=0 | .integration_budget=0' \
  brief.json > reserved.json
expect_fail "$RUN" brief-put demo root-a/claim reserved.json --session codex-session

jq '.node="root-a/conflict" | .parent="root-a" |
    .branch="mp/demo/nodes/root-a/conflict/head" | .task="Reject overlapping ownership" |
    .ownership=["b/"] | .may_decompose=false | .remaining_depth=0 |
    .descendant_budget=0 | .writer_budget=0 | .integration_budget=0' \
  brief.json > conflict.json
expect_ok "$RUN" brief-put demo root-a/conflict conflict.json --session codex-session
expect_fail "$RUN" claim demo root-a/conflict --session conflict-session --harness codex

jq '.node="root-a/sequential" | .parent="root-a" |
    .branch="mp/demo/nodes/root-a/sequential/head" | .task="Reject sequential peer" |
    .parallel_safety="Sequential" | .ownership=["a/sequential/"] |
    .may_decompose=false | .remaining_depth=0 | .descendant_budget=0 |
    .writer_budget=0 | .integration_budget=0' brief.json > sequential.json
expect_ok "$RUN" brief-put demo root-a/sequential sequential.json --session codex-session
expect_fail "$RUN" claim demo root-a/sequential --session sequential-session --harness codex

jq '.node="root-a/blocked" | .parent="root-a" |
    .branch="mp/demo/nodes/root-a/blocked/head" | .task="Reject unmet dependency" |
    .blocked_by=["root-b"] | .ownership=["a/blocked/"] | .may_decompose=false |
    .remaining_depth=0 | .descendant_budget=0 | .writer_budget=0 |
    .integration_budget=0' brief.json > blocked-brief.json
expect_ok "$RUN" brief-put demo root-a/blocked blocked-brief.json --session codex-session
expect_fail "$RUN" claim demo root-a/blocked --session blocked-session --harness codex

for name in a b c d; do
  node="root-a/slot-$name"
  jq --arg node "$node" --arg branch "mp/demo/nodes/$node/head" --arg task "Exercise slot $name" --arg owner "a/slot-$name/" \
    '.node=$node | .parent="root-a" | .branch=$branch | .task=$task | .ownership=[$owner] |
     .may_decompose=false | .remaining_depth=0 | .descendant_budget=0 |
     .writer_budget=1 | .integration_budget=0' brief.json > "slot-$name.json"
  expect_ok "$RUN" brief-put demo "$node" "slot-$name.json" --session codex-session
  "$RUN" claim demo "$node" --session "slot-$name-session" --harness codex >/dev/null || bad "claim $node"
done
for name in a b c; do
  "$RUN" slot-acquire demo writer "root-a/slot-$name" --session "slot-$name-session" --harness codex > "slot-$name.out" || bad "writer slot $name"
done
expect_fail "$RUN" slot-acquire demo writer root-a/slot-d --session slot-d-session --harness codex
slot_line=$(cat slot-a.out)
slot_n=${slot_line%% *}
slot_oid=${slot_line#* }
expect_fail "$RUN" slot-release demo writer "$slot_n" --session wrong --expected "$slot_oid"
expect_ok "$RUN" slot-release demo writer "$slot_n" --session slot-a-session --expected "$slot_oid"
expect_ok "$RUN" release-claim demo root-a/slot-a --session slot-a-session --expected "$(git rev-parse refs/megapowers/runs/demo/nodes/root-a/slot-a/claim)"
owner_oid=$(git rev-parse refs/megapowers/runs/demo/owner)
expect_fail "$RUN" slot-acquire demo integration @target --session intruder --harness claude --expected-owner "$owner_oid"
target_slot_line=$("$RUN" slot-acquire demo integration @target --session codex-1 --harness codex --expected-owner "$owner_oid")
target_slot_n=${target_slot_line%% *}
target_slot_oid=${target_slot_line#* }
expect_ok "$RUN" slot-release demo integration "$target_slot_n" --session codex-1 --expected "$target_slot_oid"

printf '== sdd-run tests: %d passed, %d failed ==\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
