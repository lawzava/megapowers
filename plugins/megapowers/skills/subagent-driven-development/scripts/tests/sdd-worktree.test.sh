#!/usr/bin/env bash
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN="$HERE/../sdd-run"
WT="$HERE/../sdd-worktree"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
pass=0
fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf 'FAIL %s\n' "$1"; }
expect_ok() { "$@" >/dev/null 2>&1 && ok || bad "expected success: $*"; }
expect_fail() { "$@" >/dev/null 2>&1 && bad "expected failure: $*" || ok; }

repo="$TMP/repo"
git init -q "$repo"
git -C "$repo" config user.name Test
git -C "$repo" config user.email test@example.invalid
git -C "$repo" config commit.gpgsign false
printf '.worktrees/\n' > "$repo/.gitignore"
printf 'base\n' > "$repo/README.md"
cat > "$repo/plan.md" <<'PLAN'
# Worktree fixture
### Task 1: Coordinator
**Blocked by:** None
**Parallel safety:** Safe
**Ownership:** `children/`
**May decompose:** Yes
PLAN
git -C "$repo" add .gitignore README.md plan.md
git -C "$repo" commit -qm 'test: seed worktree fixture'
git -C "$repo" switch -qc feature/worktree
cd "$repo" || exit 2
expect_ok "$RUN" init wt --plan plan.md --root coordinator --root second-root --target feature/worktree --session owner --harness codex --max-depth 2 --agent-budget 8 --writers 3 --integrations 1 --allow-task-commits
base=$(git rev-parse HEAD)
owner_oid=$(git rev-parse refs/megapowers/runs/wt/owner)

make_brief() {
  key=$1
  node="coordinator/$key"
  cat > "$TMP/$key.json" <<EOF
{"version":1,"run_id":"wt","node":"$node","parent":"coordinator","base":"$base",
 "branch":"mp/wt/nodes/$node/head","task":"Implement $key",
 "acceptance":["test -f README.md"],"blocked_by":[],"parallel_safety":"Safe",
 "ownership":["children/$key/"],"may_decompose":false,"remaining_depth":0,
 "descendant_budget":0,"writer_budget":1,"integration_budget":0}
EOF
  expect_ok "$RUN" brief-put wt "$node" "$TMP/$key.json" --session coordinator-session
  expect_ok "$RUN" claim wt "$node" --session "$key-session" --harness codex
}

cat > "$TMP/coordinator.json" <<EOF
{"version":1,"run_id":"wt","node":"coordinator","parent":null,"base":"$base",
 "branch":"mp/wt/nodes/coordinator/head","task":"Integrate children",
 "acceptance":["test -f README.md"],"blocked_by":[],"parallel_safety":"Safe",
 "ownership":["children/"],"may_decompose":true,"remaining_depth":2,
 "descendant_budget":4,"writer_budget":3,"integration_budget":1}
EOF
expect_ok "$RUN" brief-put wt coordinator "$TMP/coordinator.json" --session owner
expect_ok "$RUN" claim wt coordinator --session coordinator-session --harness codex
expect_ok "$WT" branch-init wt coordinator "$base"

cat > "$TMP/second-root.json" <<EOF
{"version":1,"run_id":"wt","node":"second-root","parent":null,"base":"$base",
 "branch":"mp/wt/nodes/second-root/head","task":"Implement second root",
 "acceptance":["test -f second-root/result.txt"],"blocked_by":[],
 "parallel_safety":"Safe","ownership":["second-root/"],"may_decompose":false,
 "remaining_depth":0,"descendant_budget":0,"writer_budget":1,
 "integration_budget":0}
EOF
expect_ok "$RUN" brief-put wt second-root "$TMP/second-root.json" --session owner
expect_ok "$RUN" claim wt second-root --session second-root-session --harness claude
expect_ok "$WT" branch-init wt second-root "$base"

first_path=''
for key in child-a child-b child-c; do
  make_brief "$key"
  line=$("$RUN" slot-acquire wt writer "coordinator/$key" --session "$key-session" --harness codex)
  slot=${line%% *}; oid=${line#* }
  printf '%s %s\n' "$slot" "$oid" > "$TMP/$key.slot"
  if [ "$key" = child-b ]; then
    path=$(cd "$first_path" && "$WT" node-add wt "coordinator/$key" --slot "$slot" --expected-slot "$oid")
    case "$path" in "$repo"/.worktrees/*) ok ;; *) bad "linked invocation escaped primary worktree root" ;; esac
  else
    path=$("$WT" node-add wt "coordinator/$key" --slot "$slot" --expected-slot "$oid")
  fi
  [ -d "$path" ] && ok || bad "missing node worktree $key"
  [ "$key" = child-a ] && first_path=$path
done

integration=$("$RUN" slot-acquire wt integration coordinator --session coordinator-session --harness codex)
integration_slot=${integration%% *}; integration_oid=${integration#* }
candidate=$("$WT" candidate-add wt coordinator first --base-ref refs/heads/mp/wt/nodes/coordinator/head --slot "$integration_slot" --expected-slot "$integration_oid")
[ -d "$candidate" ] && ok || bad "missing candidate worktree"
make_brief child-d
expect_fail "$RUN" slot-acquire wt writer coordinator/child-d --session child-d-session --harness codex
expect_count=$(find .worktrees/wt -type d -name .git -prune -o -type f -name .git -print | wc -l | tr -d ' ')
[ "$expect_count" -eq 4 ] && ok || bad "expected four temporary worktrees, got $expect_count"

worker_pids=()
worker_keys=()
for key in child-a child-b child-c; do
  child_path=$(git worktree list --porcelain | awk -v suffix="$key" \
    '$1 == "worktree" && $2 ~ suffix "$" {print $2; exit}')
  (
    set -e
    mkdir -p "$child_path/$key"
    printf '%s\n' "$key" > "$child_path/$key/result.txt"
    git -C "$child_path" add "$key/result.txt"
    git -C "$child_path" commit -qm "test: $key"
  ) > "$TMP/$key.worker.out" 2>&1 &
  worker_pids+=("$!")
  worker_keys+=("$key")
done
for i in "${!worker_pids[@]}"; do
  wait "${worker_pids[$i]}" && ok || bad "concurrent writer failed: ${worker_keys[$i]}"
done

git -C "$candidate" merge --no-edit mp/wt/nodes/coordinator/child-a/head >/dev/null
[ -f "$candidate/child-a/result.txt" ] && ok || bad "candidate missing child result"
expect_ok "$WT" candidate-promote wt coordinator first --slot "$integration_slot" --expected-slot "$integration_oid" --expected-head "$base"
expect_fail "$WT" candidate-promote wt coordinator first --slot "$integration_slot" --expected-slot "$integration_oid" --expected-head "$base"
coordinator_head=$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)
expect_ok "$WT" candidate-remove wt coordinator first --purpose node

failed_candidate=$("$WT" candidate-add wt coordinator failed-check --base-ref refs/heads/mp/wt/nodes/coordinator/head --slot "$integration_slot" --expected-slot "$integration_oid")
git -C "$failed_candidate" merge --no-edit mp/wt/nodes/coordinator/child-b/head >/dev/null
[ "$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)" = "$coordinator_head" ] && ok || bad "unpromoted candidate advanced coordinator"
expect_ok "$WT" candidate-remove wt coordinator failed-check --purpose node

resumed_candidate=$("$WT" candidate-add wt coordinator resumed --base-ref refs/heads/mp/wt/nodes/coordinator/head --slot "$integration_slot" --expected-slot "$integration_oid")
git -C "$resumed_candidate" merge --no-edit mp/wt/nodes/coordinator/child-b/head >/dev/null
expect_ok "$WT" candidate-promote wt coordinator resumed --slot "$integration_slot" --expected-slot "$integration_oid" --expected-head "$coordinator_head"
expect_ok "$WT" candidate-remove wt coordinator resumed --purpose node
coordinator_head=$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)

child_c_candidate=$("$WT" candidate-add wt coordinator child-c --base-ref refs/heads/mp/wt/nodes/coordinator/head --slot "$integration_slot" --expected-slot "$integration_oid")
git -C "$child_c_candidate" merge --no-edit mp/wt/nodes/coordinator/child-c/head >/dev/null
expect_ok "$WT" candidate-promote wt coordinator child-c --slot "$integration_slot" --expected-slot "$integration_oid" --expected-head "$coordinator_head"
expect_ok "$WT" candidate-remove wt coordinator child-c --purpose node
coordinator_head=$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)

mkdir -p "$TMP/evidence"
printf 'coordinator verified\n' > "$TMP/evidence/coordinator.txt"
cat > "$TMP/coordinator-result.json" <<EOF
{"status":"done","run_id":"wt","node":"coordinator","base":"$base",
 "head":"$coordinator_head","branch":"mp/wt/nodes/coordinator/head",
 "verification":[{"command":"test -f child-a/result.txt && test -f child-b/result.txt && test -f child-c/result.txt","exit_code":0,
 "evidence_path":"evidence/coordinator.txt"}],"unresolved":[]}
EOF
expect_ok "$RUN" result-put wt coordinator "$TMP/coordinator-result.json" "$TMP/evidence" --session coordinator-session
expect_ok "$RUN" release-claim wt coordinator --session coordinator-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/coordinator/claim)"

expect_ok "$RUN" slot-release wt integration "$integration_slot" --session coordinator-session --expected "$integration_oid"
target_integration=$("$RUN" slot-acquire wt integration @target --session owner --harness codex --expected-owner "$owner_oid")
target_integration_slot=${target_integration%% *}; target_integration_oid=${target_integration#* }

target_candidate=$("$WT" candidate-add wt coordinator final --base-ref refs/heads/feature/worktree --slot "$target_integration_slot" --expected-slot "$target_integration_oid")
git -C "$target_candidate" merge --no-edit mp/wt/nodes/coordinator/head >/dev/null
[ "$(git rev-parse refs/heads/feature/worktree)" = "$base" ] && ok || bad "candidate changed checked-out target"
expect_fail "$WT" target-promote wt coordinator final --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session intruder --expected-owner "$owner_oid" --expected-head "$base"
expect_ok "$WT" target-promote wt coordinator final --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session owner --expected-owner "$owner_oid" --expected-head "$base"
expect_ok "$WT" candidate-remove wt coordinator final --purpose target
expect_ok "$RUN" slot-release wt integration "$target_integration_slot" --session owner --expected "$target_integration_oid"
owner_oid=$("$RUN" owner-heartbeat wt --session owner --expected "$owner_oid")

parent_status=$(git status --porcelain=v1)
[ -z "$parent_status" ] && ok || bad "parent checkout changed"

expect_ok "$WT" node-remove wt coordinator/child-a
line=$(cat "$TMP/child-a.slot"); slot=${line%% *}; oid=${line#* }
expect_ok "$RUN" slot-release wt writer "$slot" --session child-a-session --expected "$oid"
replacement=$("$RUN" slot-acquire wt writer coordinator/child-a --session child-a-session --harness codex)
replacement_slot=${replacement%% *}; replacement_oid=${replacement#* }
expect_fail "$WT" node-add wt coordinator/child-a --slot "$replacement_slot" --expected-slot "$replacement_oid" --root not-ignored
mkdir -p "$TMP/outside"
ln -s "$TMP/outside" .worktrees/escape
expect_fail "$WT" node-add wt coordinator/child-a --slot "$replacement_slot" --expected-slot "$replacement_oid" --root .worktrees/escape
[ -z "$(find "$TMP/outside" -mindepth 1 -print -quit)" ] && ok || bad "symlink root escaped tool-owned directory"
unlink .worktrees/escape
expect_ok "$RUN" slot-release wt writer "$replacement_slot" --session child-a-session --expected "$replacement_oid"
expect_ok "$RUN" release-claim wt coordinator/child-a --session child-a-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/coordinator/child-a/claim)"
post_failure_status=$(git status --porcelain=v1)
[ -z "$post_failure_status" ] && ok || bad "failure injection changed parent checkout"

for key in child-b child-c; do
  expect_ok "$WT" node-remove wt "coordinator/$key"
  line=$(cat "$TMP/$key.slot"); slot=${line%% *}; oid=${line#* }
  expect_ok "$RUN" slot-release wt writer "$slot" --session "$key-session" --expected "$oid"
  expect_ok "$RUN" release-claim wt "coordinator/$key" --session "$key-session" --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/coordinator/$key/claim)"
done
expect_ok "$RUN" release-claim wt coordinator/child-d --session child-d-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/coordinator/child-d/claim)"

second_writer=$("$RUN" slot-acquire wt writer second-root --session second-root-session --harness claude)
second_writer_slot=${second_writer%% *}; second_writer_oid=${second_writer#* }
second_path=$("$WT" node-add wt second-root --slot "$second_writer_slot" --expected-slot "$second_writer_oid")
mkdir -p "$second_path/second-root"
printf 'second root\n' > "$second_path/second-root/result.txt"
git -C "$second_path" add second-root/result.txt
git -C "$second_path" commit -qm 'test: second root'
second_head=$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)
mkdir -p "$TMP/second-evidence"
printf 'second root verified\n' > "$TMP/second-evidence/second.txt"
cat > "$TMP/second-result.json" <<EOF
{"status":"done","run_id":"wt","node":"second-root","base":"$base",
 "head":"$second_head","branch":"mp/wt/nodes/second-root/head",
 "verification":[{"command":"test -f second-root/result.txt","exit_code":0,
 "evidence_path":"evidence/second.txt"}],"unresolved":[]}
EOF
expect_ok "$RUN" result-put wt second-root "$TMP/second-result.json" "$TMP/second-evidence" --session second-root-session
expect_ok "$WT" node-remove wt second-root
expect_ok "$RUN" slot-release wt writer "$second_writer_slot" --session second-root-session --expected "$second_writer_oid"
expect_ok "$RUN" release-claim wt second-root --session second-root-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/second-root/claim)"

second_target_base=$(git rev-parse HEAD)
target_integration=$("$RUN" slot-acquire wt integration @target --session owner --harness codex --expected-owner "$owner_oid")
target_integration_slot=${target_integration%% *}; target_integration_oid=${target_integration#* }
second_target_candidate=$("$WT" candidate-add wt second-root second --base-ref refs/heads/feature/worktree --slot "$target_integration_slot" --expected-slot "$target_integration_oid")
git -C "$second_target_candidate" merge --no-edit mp/wt/nodes/second-root/head >/dev/null
expect_ok "$WT" target-promote wt second-root second --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session owner --expected-owner "$owner_oid" --expected-head "$second_target_base"
expect_ok "$WT" candidate-remove wt second-root second --purpose target
expect_ok "$RUN" slot-release wt integration "$target_integration_slot" --session owner --expected "$target_integration_oid"
owner_oid=$("$RUN" owner-heartbeat wt --session owner --expected "$owner_oid")
closing_oid=$(printf '{"version":1,"test":true}\n' | git hash-object -w --stdin)
git update-ref refs/megapowers/runs/wt/closing "$closing_oid"
expect_fail "$WT" branch-init wt late "$base"
git update-ref -d refs/megapowers/runs/wt/closing "$closing_oid"
expect_ok "$RUN" close wt --owner-session owner --expected "$owner_oid"
expect_fail "$WT" branch-init wt late "$base"

printf '== sdd-worktree tests: %d passed, %d failed ==\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
