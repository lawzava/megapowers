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

make_minimal_result() {
  result_run=$1 result_node=$2 result_head=$3
  result_blob=$(printf '{"status":"done","run_id":"%s","node":"%s","head":"%s"}\n' \
    "$result_run" "$result_node" "$result_head" | git hash-object -w --stdin)
  result_tree=$(printf '100644 blob %s\tresult.json\n' "$result_blob" | git mktree)
  printf 'test result\n' | git commit-tree "$result_tree"
}

make_blocked_result_tree() {
  result_run=$1 result_node=$2 result_base=$3 result_head=$4 result_branch=$5
  result_blob=$(jq -cn --arg run "$result_run" --arg node "$result_node" \
    --arg base "$result_base" --arg head "$result_head" --arg branch "$result_branch" \
    '{status:"blocked",run_id:$run,node:$node,base:$base,head:$head,branch:$branch,
      verification:[{command:"test -f partial",exit_code:1,evidence_path:"evidence/test.txt"}],
      unresolved:["still blocked"]}' | git hash-object -w --stdin)
  result_tree=$(printf '100644 blob %s\tresult.json\n' "$result_blob" | git mktree)
  printf '%s\n' "$result_tree"
}

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
printf '*.local-artifact\n' >> "$(git rev-parse --git-path info/exclude)"

real_git=$(command -v git)
base_path=$PATH
mkdir -p "$TMP/git-race-wrapper"
cat > "$TMP/git-race-wrapper/git" <<'GIT_WRAPPER'
#!/usr/bin/env bash
set -euo pipefail

command_line=" $* "
if [ -e "$MP_RACE_MARKER" ]; then
  case "$command_line" in
    *" read-tree -u -m "*)
      if [ "${MP_RACE_ACTION:-}" = sync-fail ] && [ ! -e "$MP_RACE_MARKER.sync" ]; then
        : > "$MP_RACE_MARKER.sync"
        exit 98
      fi
      ;;
  esac
  exec "$MP_REAL_GIT" "$@"
fi

transaction=''
intercept=0
case "$command_line" in
  *" update-ref --stdin "*)
    transaction="$MP_RACE_MARKER.transaction"
    command cat > "$transaction"
    case "${MP_RACE_KIND:-}" in
      branch) grep -qF "create refs/heads/mp/$MP_RACE_RUN/nodes/$MP_RACE_NODE/head" "$transaction" && intercept=1 ;;
      target) grep -qF "update refs/heads/$MP_RACE_TARGET " "$transaction" && intercept=1 ;;
    esac
    ;;
  *" merge --ff-only "*)
    [ "${MP_RACE_KIND:-}" = target ] && intercept=1
    ;;
  *" worktree add "*)
    [ "${MP_RACE_KIND:-}" = worktree ] && intercept=1
    ;;
esac

if [ "$intercept" -eq 1 ] && [ ! -e "$MP_RACE_MARKER" ]; then
  : > "$MP_RACE_MARKER"
  case "$MP_RACE_ACTION" in
    generation)
      PATH="$MP_BASE_PATH" "$MP_RUN" owner-heartbeat "$MP_RACE_RUN" \
        --session "$MP_RACE_SESSION" --expected "$MP_RACE_OWNER" >/dev/null
      ;;
    cleanup)
      PATH="$MP_BASE_PATH" "$MP_RUN" close "$MP_RACE_RUN" \
        --owner-session "$MP_RACE_SESSION" --expected "$MP_RACE_OWNER" >/dev/null
      closed=$(PATH="$MP_BASE_PATH" "$MP_REAL_GIT" -C "$MP_REPO" rev-parse \
        "refs/megapowers/runs/$MP_RACE_RUN/closed")
      PATH="$MP_BASE_PATH" "$MP_RUN" cleanup "$MP_RACE_RUN" \
        --expected-closed "$closed" --confirmed >/dev/null
      ;;
    release-slot)
      "$MP_REAL_GIT" -C "$MP_REPO" update-ref -d \
        "refs/megapowers/runs/$MP_RACE_RUN/slots/integration/$MP_RACE_SLOT" \
        "$MP_RACE_SLOT_OID"
      ;;
    advance-head)
      "$MP_REAL_GIT" -C "$MP_REPO" reset --hard "$MP_RACE_HEAD" >/dev/null
      ;;
    sync-fail)
      case "$command_line" in
        *" merge --ff-only "*) "$MP_REAL_GIT" "$@" >/dev/null; exit 98 ;;
      esac
      ;;
    *) exit 97 ;;
  esac
fi

if [ -n "$transaction" ]; then
  exec "$MP_REAL_GIT" "$@" < "$transaction"
fi
exec "$MP_REAL_GIT" "$@"
GIT_WRAPPER
chmod +x "$TMP/git-race-wrapper/git"

expect_ok "$RUN" init wt --plan plan.md --root coordinator --root second-root --target feature/worktree --session owner --harness codex --max-depth 2 --agent-budget 8 --writers 3 --integrations 1 --allow-task-commits
base=$(git rev-parse HEAD)
unrelated=$(printf 'test: unrelated history\n' |
  git commit-tree "$(git rev-parse "$base^{tree}")")
owner_oid=$(git rev-parse refs/megapowers/runs/wt/owner)

# Branch creation must be bound to the exact generation it read.
expect_ok "$RUN" init generation-race --plan plan.md --root generation-root \
  --target feature/worktree --session generation-owner --harness codex \
  --max-depth 1 --agent-budget 1 --writers 1 --integrations 1 --allow-task-commits
generation_owner=$(git rev-parse refs/megapowers/runs/generation-race/owner)
generation_marker="$TMP/generation-race"
expect_fail env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=branch MP_RACE_ACTION=generation \
  MP_RACE_MARKER="$generation_marker" MP_RACE_RUN=generation-race \
  MP_RACE_NODE=generation-root MP_RACE_SESSION=generation-owner MP_RACE_OWNER="$generation_owner" \
  "$WT" branch-init generation-race generation-root "$base"
[ -e "$generation_marker" ] && ok || bad "generation race did not run"
git show-ref --verify --quiet refs/heads/mp/generation-race/nodes/generation-root/head &&
  bad "stale branch-init ignored a generation change" || ok
git update-ref -d refs/heads/mp/generation-race/nodes/generation-root/head 2>/dev/null || true

# A branch-init paused across close and cleanup must not resurrect run refs.
expect_ok "$RUN" init cleanup-race --plan plan.md --root cleanup-root \
  --target feature/worktree --session cleanup-owner --harness codex \
  --max-depth 1 --agent-budget 1 --writers 1 --integrations 1 --allow-task-commits
cleanup_owner=$(git rev-parse refs/megapowers/runs/cleanup-race/owner)
cleanup_result=$(make_minimal_result cleanup-race cleanup-root "$base")
git update-ref refs/megapowers/runs/cleanup-race/nodes/cleanup-root/result "$cleanup_result"
cleanup_marker="$TMP/cleanup-race"
expect_fail env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=branch MP_RACE_ACTION=cleanup \
  MP_RACE_MARKER="$cleanup_marker" MP_RACE_RUN=cleanup-race MP_RACE_NODE=cleanup-root \
  MP_RACE_SESSION=cleanup-owner MP_RACE_OWNER="$cleanup_owner" \
  "$WT" branch-init cleanup-race cleanup-root "$base"
[ -e "$cleanup_marker" ] && ok || bad "cleanup race did not run"
git show-ref --verify --quiet refs/heads/mp/cleanup-race/nodes/cleanup-root/head &&
  bad "stale branch-init resurrected a cleaned run branch" || ok
git show-ref --verify --quiet refs/megapowers/runs/cleanup-race/manifest &&
  bad "cleanup race left the run manifest" || ok
git update-ref -d refs/heads/mp/cleanup-race/nodes/cleanup-root/head 2>/dev/null || true

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
git update-ref refs/heads/mp/wt/nodes/second-root/head "$unrelated" "$base"
expect_fail "$WT" branch-init wt second-root "$base"
[ "$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)" = "$unrelated" ] && ok ||
  bad "unrelated branch-init changed the node branch"
git update-ref refs/heads/mp/wt/nodes/second-root/head "$base" "$unrelated"

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
divergent_candidate=$("$WT" candidate-add wt coordinator divergent --base-ref refs/heads/mp/wt/nodes/coordinator/head --slot "$integration_slot" --expected-slot "$integration_oid")
git -C "$divergent_candidate" reset --hard "$unrelated" >/dev/null
expect_fail "$WT" candidate-promote wt coordinator divergent --slot "$integration_slot" --expected-slot "$integration_oid" --expected-head "$base"
[ "$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)" = "$base" ] && ok ||
  bad "divergent candidate changed the coordinator branch"
git update-ref refs/heads/mp/wt/nodes/coordinator/head "$base" "$unrelated" 2>/dev/null || true
expect_ok "$WT" candidate-remove wt coordinator divergent --purpose node
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
printf 'keep candidate artifact\n' > "$failed_candidate/keep.local-artifact"
expect_fail "$WT" candidate-remove wt coordinator failed-check --purpose node
[ -f "$failed_candidate/keep.local-artifact" ] && ok || bad "candidate removal deleted an ignored artifact"
if [ -d "$failed_candidate" ]; then
  rm "$failed_candidate/keep.local-artifact"
  expect_ok "$WT" candidate-remove wt coordinator failed-check --purpose node
fi

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
expect_ok "$WT" branch-init wt coordinator "$base"
[ "$(git rev-parse refs/heads/mp/wt/nodes/coordinator/head)" = "$coordinator_head" ] && ok ||
  bad "descendant branch-init changed the coordinator branch"

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
printf 'candidate intermediate\n' > "$target_candidate/target-local.txt"
git -C "$target_candidate" add target-local.txt
git -C "$target_candidate" commit -qm 'test: add target intermediate'
target_intermediate=$(git -C "$target_candidate" rev-parse HEAD)
printf 'candidate final\n' > "$target_candidate/target-final.txt"
git -C "$target_candidate" add target-final.txt
git -C "$target_candidate" commit -qm 'test: add target final'
[ "$(git rev-parse refs/heads/feature/worktree)" = "$base" ] && ok || bad "candidate changed checked-out target"
expect_fail "$WT" target-promote wt coordinator final --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session intruder --expected-owner "$owner_oid" --expected-head "$base"

# A checked-out target sync failure must restore the exact ref and worktree.
target_sync_marker="$TMP/target-sync-race"
env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=target MP_RACE_ACTION=sync-fail \
  MP_RACE_MARKER="$target_sync_marker" MP_RACE_RUN=wt MP_RACE_TARGET=feature/worktree \
  "$WT" target-promote wt coordinator final --slot "$target_integration_slot" \
  --expected-slot "$target_integration_oid" --owner-session owner \
  --expected-owner "$owner_oid" --expected-head "$base" >/dev/null 2>&1
target_sync_status=$?
[ -e "$target_sync_marker.sync" ] && ok || bad "target sync failure did not run"
[ "$target_sync_status" -ne 0 ] && ok || bad "target sync failure reported success"
[ "$(git rev-parse refs/heads/feature/worktree)" = "$base" ] && ok ||
  bad "target sync failure did not restore the target ref"
[ ! -e target-local.txt ] && [ ! -e target-final.txt ] && ok ||
  bad "target sync failure did not restore the target worktree"
git reset --hard "$base" >/dev/null

# Authorization released immediately before the ref advance must block promotion.
target_slot_marker="$TMP/target-slot-race"
env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=target MP_RACE_ACTION=release-slot \
  MP_RACE_MARKER="$target_slot_marker" MP_RACE_RUN=wt MP_RACE_TARGET=feature/worktree \
  MP_RACE_SLOT="$target_integration_slot" MP_RACE_SLOT_OID="$target_integration_oid" \
  MP_RACE_SESSION=owner \
  "$WT" target-promote wt coordinator final --slot "$target_integration_slot" \
  --expected-slot "$target_integration_oid" --owner-session owner \
  --expected-owner "$owner_oid" --expected-head "$base" >/dev/null 2>&1
target_slot_status=$?
[ -e "$target_slot_marker" ] && ok || bad "target slot race did not run"
[ "$target_slot_status" -ne 0 ] && ok || bad "target promotion ignored released authorization"
[ "$(git rev-parse refs/heads/feature/worktree)" = "$base" ] && ok ||
  bad "stale authorization advanced the target"
git reset --hard "$base" >/dev/null
target_integration=$("$RUN" slot-acquire wt integration @target --session owner --harness codex --expected-owner "$owner_oid")
target_integration_slot=${target_integration%% *}; target_integration_oid=${target_integration#* }

# An expected-head advance immediately before the ref transaction must win.
target_head_marker="$TMP/target-head-race"
env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=target MP_RACE_ACTION=advance-head \
  MP_RACE_MARKER="$target_head_marker" MP_RACE_RUN=wt MP_RACE_TARGET=feature/worktree \
  MP_RACE_HEAD="$target_intermediate" \
  "$WT" target-promote wt coordinator final --slot "$target_integration_slot" \
  --expected-slot "$target_integration_oid" --owner-session owner \
  --expected-owner "$owner_oid" --expected-head "$base" >/dev/null 2>&1
target_head_status=$?
[ -e "$target_head_marker" ] && ok || bad "target head race did not run"
[ "$target_head_status" -ne 0 ] && ok || bad "target promotion ignored the stale expected head"
[ "$(git rev-parse refs/heads/feature/worktree)" = "$target_intermediate" ] && ok ||
  bad "stale-head promotion overwrote the competing target head"
git reset --hard "$base" >/dev/null

# An ignored target file must never be overwritten by the candidate tree.
printf 'target-local.txt\n' >> "$(git rev-parse --git-path info/exclude)"
printf 'user target data\n' > target-local.txt
expect_fail "$WT" target-promote wt coordinator final --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session owner --expected-owner "$owner_oid" --expected-head "$base"
[ "$(cat target-local.txt)" = 'user target data' ] && ok || bad "target promotion overwrote ignored user data"
[ "$(git rev-parse refs/heads/feature/worktree)" = "$base" ] && ok || bad "ignored target data did not block the ref advance"
rm -f target-local.txt
git reset --hard "$base" >/dev/null
expect_ok "$WT" target-promote wt coordinator final --slot "$target_integration_slot" --expected-slot "$target_integration_oid" --owner-session owner --expected-owner "$owner_oid" --expected-head "$base"
expect_ok "$WT" candidate-remove wt coordinator final --purpose target
expect_ok "$RUN" slot-release wt integration "$target_integration_slot" --session owner --expected "$target_integration_oid"
owner_oid=$("$RUN" owner-heartbeat wt --session owner --expected "$owner_oid")

parent_status=$(git status --porcelain=v1)
[ -z "$parent_status" ] && ok || bad "parent checkout changed"

printf 'keep node artifact\n' > "$first_path/keep.local-artifact"
expect_fail "$WT" node-remove wt coordinator/child-a
[ -f "$first_path/keep.local-artifact" ] && ok || bad "node removal deleted an ignored artifact"
if [ -d "$first_path" ]; then
  rm "$first_path/keep.local-artifact"
  expect_ok "$WT" node-remove wt coordinator/child-a
fi
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

# A valid harness-owned root may live outside the repository. Generated paths
# must remain beneath its canonical location and reject nested symlink escapes.
harness_root="$TMP/harness-root"
harness_path="$harness_root/wt/nodes/second-root"
if harness_output=$("$WT" node-add wt second-root --slot "$second_writer_slot" \
  --expected-slot "$second_writer_oid" --harness-owned-root "$harness_root" 2>&1); then
  ok
else
  bad "valid harness-owned root was rejected: $harness_output"
fi
if git worktree list --porcelain | grep -qF "worktree $harness_path"; then
  ok
  expect_fail "$WT" node-add wt second-root --slot "$second_writer_slot" \
    --expected-slot "$second_writer_oid" --harness-owned-root "$harness_path"
  expect_ok "$WT" node-remove wt second-root --harness-owned-root "$harness_root"
else
  bad "valid harness-owned root did not register the node worktree"
fi

harness_escape_root="$TMP/harness-escape-root"
harness_escape_outside="$TMP/harness-escape-outside"
mkdir -p "$harness_escape_root" "$harness_escape_outside"
ln -s "$harness_escape_outside" "$harness_escape_root/wt"
harness_branch_before=$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)
expect_fail "$WT" node-add wt second-root --slot "$second_writer_slot" \
  --expected-slot "$second_writer_oid" --harness-owned-root "$harness_escape_root"
[ -z "$(find "$harness_escape_outside" -mindepth 1 -print -quit)" ] && ok ||
  bad "nested harness-owned symlink redirected generated worktree data"
git worktree list --porcelain | grep -qF "worktree $harness_escape_outside" &&
  bad "nested harness-owned symlink registered an outside worktree" || ok
[ "$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)" = "$harness_branch_before" ] &&
  ok || bad "nested harness-owned symlink changed the run branch"

post_add_marker="$TMP/post-add-generation-race"
expect_fail env PATH="$TMP/git-race-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_BASE_PATH="$base_path" \
  MP_RUN="$RUN" MP_REPO="$repo" MP_RACE_KIND=worktree MP_RACE_ACTION=generation \
  MP_RACE_MARKER="$post_add_marker" MP_RACE_RUN=wt MP_RACE_SESSION=owner \
  MP_RACE_OWNER="$owner_oid" \
  "$WT" node-add wt second-root --slot "$second_writer_slot" \
  --expected-slot "$second_writer_oid"
[ -e "$post_add_marker" ] && ok || bad "post-add generation race did not run"
[ ! -e "$repo/.worktrees/wt/nodes/second-root" ] && ok ||
  bad "stale node-add left a worktree after generation change"
git worktree list --porcelain | grep -qF "$repo/.worktrees/wt/nodes/second-root" &&
  bad "stale node-add left a registered worktree" || ok
owner_oid=$(git rev-parse refs/megapowers/runs/wt/owner)
second_path=$("$WT" node-add wt second-root --slot "$second_writer_slot" --expected-slot "$second_writer_oid")
mkdir -p "$second_path/second-root"
printf 'partial second root\n' > "$second_path/second-root/result.txt"
git -C "$second_path" add second-root/result.txt
git -C "$second_path" commit -qm 'test: partial second root'
second_head=$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)
mkdir -p "$TMP/second-evidence"
printf 'second root blocked\n' > "$TMP/second-evidence/second.txt"
cat > "$TMP/second-result.json" <<EOF
{"status":"blocked","run_id":"wt","node":"second-root","base":"$base",
 "head":"$second_head","branch":"mp/wt/nodes/second-root/head",
 "verification":[{"command":"test -f second-root/complete.txt","exit_code":1,
 "evidence_path":"evidence/second.txt"}],"unresolved":["completion pending"]}
EOF
blocked_second_oid=$("$RUN" result-put wt second-root "$TMP/second-result.json" "$TMP/second-evidence" --session second-root-session)
expect_ok "$WT" node-remove wt second-root
expect_ok "$RUN" slot-release wt writer "$second_writer_slot" --session second-root-session --expected "$second_writer_oid"
expect_ok "$RUN" release-claim wt second-root --session second-root-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/second-root/claim)"
expect_ok "$RUN" claim wt second-root --session recovered-session --harness codex
recovered_writer=$("$RUN" slot-acquire wt writer second-root --session recovered-session --harness codex)
recovered_writer_slot=${recovered_writer%% *}; recovered_writer_oid=${recovered_writer#* }

alternative=$(printf 'test: alternate descendant\n' |
  git commit-tree "$(git rev-parse "$base^{tree}")" -p "$base")
git update-ref refs/heads/mp/wt/nodes/second-root/head "$alternative" "$second_head"
expect_fail "$WT" node-add wt second-root --slot "$recovered_writer_slot" --expected-slot "$recovered_writer_oid"
[ ! -e "$repo/.worktrees/wt/nodes/second-root" ] && ok ||
  bad "mismatched blocked result created a worktree"

unrelated_result=$(make_blocked_result_tree wt second-root "$base" "$unrelated" mp/wt/nodes/second-root/head)
git update-ref refs/heads/mp/wt/nodes/second-root/head "$unrelated" "$alternative"
git update-ref refs/megapowers/runs/wt/nodes/second-root/result "$unrelated_result" "$blocked_second_oid"
expect_fail "$WT" node-add wt second-root --slot "$recovered_writer_slot" --expected-slot "$recovered_writer_oid"
[ ! -e "$repo/.worktrees/wt/nodes/second-root" ] && ok ||
  bad "unrelated blocked result created a worktree"
git update-ref refs/heads/mp/wt/nodes/second-root/head "$second_head" "$unrelated"
git update-ref refs/megapowers/runs/wt/nodes/second-root/result "$blocked_second_oid" "$unrelated_result"

if second_path=$("$WT" node-add wt second-root --slot "$recovered_writer_slot" --expected-slot "$recovered_writer_oid"); then
  ok
else
  bad "advanced blocked writer could not recreate its worktree"
  second_path="$repo/.worktrees/wt/nodes/second-root"
  git worktree add "$second_path" mp/wt/nodes/second-root/head >/dev/null
fi
[ "$(git -C "$second_path" rev-parse HEAD)" = "$second_head" ] && ok ||
  bad "recreated writer did not resume at the blocked head"
printf 'second root complete\n' > "$second_path/second-root/complete.txt"
git -C "$second_path" add second-root/complete.txt
git -C "$second_path" commit -qm 'test: complete second root'
second_head=$(git rev-parse refs/heads/mp/wt/nodes/second-root/head)
printf 'second root verified\n' > "$TMP/second-evidence/second.txt"
cat > "$TMP/second-result.json" <<EOF
{"status":"done","run_id":"wt","node":"second-root","base":"$base",
 "head":"$second_head","branch":"mp/wt/nodes/second-root/head",
 "verification":[{"command":"test -f second-root/complete.txt","exit_code":0,
 "evidence_path":"evidence/second.txt"}],"unresolved":[]}
EOF
expect_ok "$RUN" result-put wt second-root "$TMP/second-result.json" "$TMP/second-evidence" --session recovered-session --expected "$blocked_second_oid"
expect_ok "$WT" node-remove wt second-root
expect_ok "$RUN" slot-release wt writer "$recovered_writer_slot" --session recovered-session --expected "$recovered_writer_oid"
expect_ok "$RUN" release-claim wt second-root --session recovered-session --expected "$(git rev-parse refs/megapowers/runs/wt/nodes/second-root/claim)"

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
