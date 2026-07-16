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
expect_ne() { [ "$1" != "$2" ] && ok || bad "expected '$1' != '$2'"; }
wait_for_file() {
  local file=$1 attempts=${2:-200} count=0
  while [ ! -e "$file" ] && [ "$count" -lt "$attempts" ]; do
    sleep 0.01
    count=$((count + 1))
  done
  [ -e "$file" ]
}
expect_not_before_release() {
  if wait_for_file "$1" 100; then
    bad "$2"
  else
    ok
  fi
}
write_root_brief() {
  local run=$1 node=$2 ownership=$3 remaining=$4 descendant=$5 writers=$6 integrations=$7 output=$8
  local may_decompose=false
  [ "$descendant" -gt 0 ] && may_decompose=true
  jq -n --arg run "$run" --arg node "$node" --arg base "$head" \
    --arg branch "mp/$run/nodes/$node/head" --arg ownership "$ownership" \
    --argjson may_decompose "$may_decompose" --argjson remaining "$remaining" \
    --argjson descendant "$descendant" --argjson writers "$writers" \
    --argjson integrations "$integrations" \
    '{version:1,run_id:$run,node:$node,parent:null,base:$base,branch:$branch,
      task:("Implement " + $node),acceptance:["test -f README.md"],blocked_by:[],
      parallel_safety:"Safe",ownership:[$ownership],may_decompose:$may_decompose,
      remaining_depth:$remaining,descendant_budget:$descendant,
      writer_budget:$writers,integration_budget:$integrations}' > "$output"
}
write_child_brief() {
  local run=$1 node=$2 parent=$3 ownership=$4 remaining=$5 descendant=$6 output=$7
  local may_decompose=false base
  [ "$descendant" -gt 0 ] && may_decompose=true
  base=$(git rev-parse "refs/heads/mp/$run/nodes/$parent/head")
  jq -n --arg run "$run" --arg node "$node" --arg parent "$parent" --arg base "$base" \
    --arg branch "mp/$run/nodes/$node/head" --arg ownership "$ownership" \
    --argjson may_decompose "$may_decompose" --argjson remaining "$remaining" \
    --argjson descendant "$descendant" \
    '{version:1,run_id:$run,node:$node,parent:$parent,base:$base,branch:$branch,
      task:("Implement " + $node),acceptance:["test -f README.md"],blocked_by:[],
      parallel_safety:"Safe",ownership:[$ownership],may_decompose:$may_decompose,
      remaining_depth:$remaining,descendant_budget:$descendant,
      writer_budget:0,integration_budget:0}' > "$output"
}

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
git -C "$repo" config core.hooksPath "$repo/.git/hooks"
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
cat > "$repo/.git/hooks/reference-transaction" <<'HOOK'
#!/bin/sh
state=$1
match=0
if [ "$state" = prepared ] && [ -n "${MP_RACE_GATE:-}" ] && [ -n "${MP_RACE_REF_MATCH:-}" ]; then
  while read -r old_oid new_oid ref; do
    case "$ref" in
      *"$MP_RACE_REF_MATCH"*) match=1 ;;
    esac
  done
  if [ "$match" -eq 1 ]; then
    : > "$MP_RACE_GATE.ready.$MP_RACE_ID"
    count=0
    while [ ! -e "$MP_RACE_GATE.release" ] && [ "$count" -lt 1000 ]; do
      sleep 0.01
      count=$((count + 1))
    done
    [ -e "$MP_RACE_GATE.release" ] || exit 1
  fi
fi
exit 0
HOOK
chmod +x "$repo/.git/hooks/reference-transaction"

real_git=$(command -v git)
real_jq=$(command -v jq)
mkdir -p "$TMP/git-wrapper" "$TMP/jq-wrapper" "$TMP/date-wrapper"
cat > "$TMP/git-wrapper/git" <<'WRAPPER'
#!/bin/sh
if [ "${1:-}" = hash-object ] && [ -n "${MP_MUTATE_BRIEF:-}" ]; then
  for argument in "$@"; do
    if [ "$argument" = "$MP_MUTATE_BRIEF" ]; then
      "$MP_REAL_GIT" "$@"
      status=$?
      command cp "$MP_MUTATE_REPLACEMENT" "$MP_MUTATE_BRIEF"
      : > "$MP_MUTATE_MARKER"
      exit "$status"
    fi
  done
fi
if [ "${1:-}" = update-ref ] && [ "${2:-}" = --stdin ] &&
   [ -n "${MP_MOVE_REF:-}" ]; then
  transaction=$(mktemp)
  trap 'rm -f "$transaction"' EXIT HUP INT TERM
  command cat > "$transaction"
  "$MP_REAL_GIT" update-ref "$MP_MOVE_REF" "$MP_MOVE_TO" "$MP_MOVE_FROM" || exit 99
  "$MP_REAL_GIT" update-ref --stdin < "$transaction"
  exit $?
fi
exec "$MP_REAL_GIT" "$@"
WRAPPER
cat > "$TMP/jq-wrapper/jq" <<'WRAPPER'
#!/bin/sh
"$MP_REAL_JQ" "$@"
status=$?
case " $* " in
  *" ${MP_MUTATE_BRIEF:-__no_brief__} "*)
    if [ ! -e "$MP_MUTATE_MARKER" ]; then
      command cp "$MP_MUTATE_REPLACEMENT" "$MP_MUTATE_BRIEF"
      : > "$MP_MUTATE_MARKER"
    fi
    ;;
esac
exit "$status"
WRAPPER
cat > "$TMP/date-wrapper/date" <<'WRAPPER'
#!/bin/sh
printf '%s\n' '2026-07-16T12:00:00Z'
WRAPPER
chmod +x "$TMP/git-wrapper/git" "$TMP/jq-wrapper/jq" "$TMP/date-wrapper/date"

cd "$repo" || exit 2
expect_fail "$RUN" init Bad_ID --plan plan.md --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
expect_fail "$RUN" init no-commits --plan plan.md --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8
expect_fail "$RUN" init duplicate-roots --plan plan.md --root root-a --root root-a --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
expect_fail "$RUN" init ancestor-roots --plan plan.md --root root --root root/child --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
git branch release
git switch -q release
expect_fail "$RUN" init protected --plan plan.md --root root-a --target release --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
git switch -q feature/multi-writer
expect_ok "$RUN" init demo --plan plan.md --root root-a --root root-b --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 9 --writers 3 --integrations 1 --allow-task-commits
expect_ok "$RUN" init defaults --plan plan.md --root default-root --target feature/multi-writer --session codex-1 --harness codex --max-depth 2 --agent-budget 8 --allow-task-commits
expect_ok "$RUN" init budget-root --plan plan.md --root budget-a --root budget-b --target feature/multi-writer --session budget-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init budget-child --plan plan.md --root parent --target feature/multi-writer --session child-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init claim-overlap --plan plan.md --root overlap-a --root overlap-b --target feature/multi-writer --session overlap-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init claim-session --plan plan.md --root session-a --root session-b --target feature/multi-writer --session session-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init slot-duplicate --plan plan.md --root slot-node --target feature/multi-writer --session slot-owner --harness codex --max-depth 2 --agent-budget 1 --writers 3 --allow-task-commits
expect_ok "$RUN" init stale-root --plan plan.md --root stale-root --target feature/multi-writer --session stale-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init stale-parent --plan plan.md --root stale-parent --target feature/multi-writer --session stale-parent-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init stale-slot-claim --plan plan.md --root stale-slot --target feature/multi-writer --session stale-slot-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init stale-slot-owner --plan plan.md --root unused-root --target feature/multi-writer --session target-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init ownership-alias --plan plan.md --root alias-a --root alias-b --target feature/multi-writer --session alias-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init ownership-interface-segment --plan plan.md --root interface-path-a --root interface-path-b --target feature/multi-writer --session interface-path-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init ownership-unsafe --plan plan.md --root unsafe-root --target feature/multi-writer --session unsafe-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init ownership-root --plan plan.md --root root-all --root root-part --target feature/multi-writer --session root-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init snapshot-brief --plan plan.md --root snapshot-node --target feature/multi-writer --session snapshot-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init claim-aba --plan plan.md --root aba-node --target feature/multi-writer --session aba-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init slot-aba --plan plan.md --root aba-slot --target feature/multi-writer --session aba-slot-owner --harness codex --max-depth 2 --agent-budget 1 --writers 1 --allow-task-commits
expect_ok "$RUN" init movement --plan plan.md --root branch-node --root dep-node --root done-node --target feature/multi-writer --session movement-owner --harness codex --max-depth 2 --agent-budget 3 --allow-task-commits
expect_ok "$RUN" init lock-tz --plan plan.md --root tz-node --target feature/multi-writer --session tz-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init lock-inaccessible --plan plan.md --root inaccessible-node --target feature/multi-writer --session inaccessible-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
expect_ok "$RUN" init generation-crash --plan plan.md --root crash-node --root crash-peer --target feature/multi-writer --session crash-owner --harness codex --max-depth 2 --agent-budget 2 --allow-task-commits
expect_ok "$RUN" init duplicate-blocked --plan plan.md --root duplicate-blocked-node --target feature/multi-writer --session duplicate-blocked-owner --harness codex --max-depth 2 --agent-budget 1 --allow-task-commits
defaults_base=refs/megapowers/runs/defaults
expect_eq "$(git cat-file blob "$defaults_base/manifest" | jq -r '.writer_limit')" 3
expect_eq "$(git cat-file blob "$defaults_base/manifest" | jq -r '.integration_limit')" 1
expect_ok git show-ref --verify --quiet "$defaults_base/generation"
expect_eq "$(git cat-file blob "$defaults_base/generation" 2>/dev/null | jq -r '.operation' 2>/dev/null || true)" init

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
expect_eq "$(git for-each-ref --format='%(refname)' "$base" | wc -l | tr -d ' ')" 4
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.target_branch')" feature/multi-writer
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.roots | join(",")')" root-a,root-b
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.writer_limit')" 3
expect_eq "$(git cat-file blob "$base/manifest" | jq -r '.integration_limit')" 1
expect_eq "$(git cat-file blob "$base/owner" | jq -r '.session')" codex-1
expect_ok git show-ref --verify --quiet "$base/generation"
git config --get-regexp '^remote\..*\.push$' 2>/dev/null | grep -q 'refs/megapowers' && bad "private refs entered a push refspec" || ok
expect_ok "$RUN" join demo --plan plan.md --target feature/multi-writer --harness claude
printf '\nchanged\n' >> plan.md
expect_fail "$RUN" join demo --plan plan.md --target feature/multi-writer --harness codex

git checkout -- plan.md
head=$(git rev-parse HEAD)

# Ownership aliases must be canonical before immutable storage and peer comparison.
write_root_brief ownership-alias alias-a /src 0 0 0 0 alias-a.json
expect_fail "$RUN" brief-put ownership-alias alias-a alias-a.json --session alias-owner
write_root_brief ownership-alias alias-a src/../lib 0 0 0 0 alias-a.json
expect_fail "$RUN" brief-put ownership-alias alias-a alias-a.json --session alias-owner
write_root_brief ownership-unsafe unsafe-root C:/src 0 0 0 0 unsafe-root.json
expect_fail "$RUN" brief-put ownership-unsafe unsafe-root unsafe-root.json --session unsafe-owner
write_root_brief ownership-alias alias-a ./src/ 0 0 0 0 alias-a.json
write_root_brief ownership-alias alias-b src//lib/ 0 0 0 0 alias-b.json
expect_ok "$RUN" brief-put ownership-alias alias-a alias-a.json --session alias-owner
expect_ok "$RUN" brief-put ownership-alias alias-b alias-b.json --session alias-owner
expect_eq "$(git cat-file blob refs/megapowers/runs/ownership-alias/nodes/alias-a/brief | jq -r '.ownership[0]')" src
expect_eq "$(git cat-file blob refs/megapowers/runs/ownership-alias/nodes/alias-b/brief | jq -r '.ownership[0]')" src/lib
expect_ok "$RUN" claim ownership-alias alias-a --session alias-a-session --harness codex
expect_fail "$RUN" claim ownership-alias alias-b --session alias-b-session --harness claude

# A normal relative path containing ':interface:' remains a path for overlap checks.
write_root_brief ownership-interface-segment interface-path-a src:interface:api 0 0 0 0 interface-path-a.json
write_root_brief ownership-interface-segment interface-path-b src:interface:api/lib 0 0 0 0 interface-path-b.json
expect_ok "$RUN" brief-put ownership-interface-segment interface-path-a interface-path-a.json --session interface-path-owner
expect_ok "$RUN" brief-put ownership-interface-segment interface-path-b interface-path-b.json --session interface-path-owner
expect_ok "$RUN" claim ownership-interface-segment interface-path-a --session interface-path-a-session --harness codex
expect_fail "$RUN" claim ownership-interface-segment interface-path-b --session interface-path-b-session --harness claude

# Duplicate dependency entries are rejected before an immutable brief is created.
write_root_brief duplicate-blocked duplicate-blocked-node duplicate-blocked/ 0 0 0 0 duplicate-blocked.json
jq '.blocked_by=["dependency","dependency"]' duplicate-blocked.json > duplicate-blocked-with-deps.json
mv duplicate-blocked-with-deps.json duplicate-blocked.json
if duplicate_blocked_output=$("$RUN" brief-put duplicate-blocked duplicate-blocked-node duplicate-blocked.json --session duplicate-blocked-owner 2>&1); then
  bad "duplicate blocked_by entries were accepted"
else
  duplicate_blocked_status=$?
  if [ "$duplicate_blocked_status" -eq 2 ] &&
     [ "$duplicate_blocked_output" = "sdd-run: blocked_by must not contain duplicate nodes" ]; then
    ok
  else
    bad "duplicate blocked_by rejection did not return the documented validation error"
  fi
fi

# Dot is the canonical whole-repository ownership root and overlaps every path.
write_root_brief ownership-root root-all . 0 0 0 0 root-all.json
write_root_brief ownership-root root-part ./other// 0 0 0 0 root-part.json
expect_ok "$RUN" brief-put ownership-root root-all root-all.json --session root-owner
expect_ok "$RUN" brief-put ownership-root root-part root-part.json --session root-owner
expect_eq "$(git cat-file blob refs/megapowers/runs/ownership-root/nodes/root-all/brief | jq -r '.ownership[0]')" .
expect_eq "$(git cat-file blob refs/megapowers/runs/ownership-root/nodes/root-part/brief | jq -r '.ownership[0]')" other
expect_ok "$RUN" claim ownership-root root-all --session root-all-session --harness codex
expect_fail "$RUN" claim ownership-root root-part --session root-part-session --harness claude

# The caller's brief file is snapshotted before validation and never read again.
write_root_brief snapshot-brief snapshot-node snapshot/ 0 0 0 0 snapshot.json
jq '.task="Validated snapshot"' snapshot.json > snapshot-valid.json
mv snapshot-valid.json snapshot.json
jq '.task="Mutable replacement" | .ownership=["replacement/"]' \
  snapshot.json > snapshot-replacement.json
snapshot_marker="$TMP/snapshot-mutated"
expect_ok env PATH="$TMP/git-wrapper:$TMP/jq-wrapper:$PATH" MP_REAL_GIT="$real_git" MP_REAL_JQ="$real_jq" \
  MP_MUTATE_BRIEF=snapshot.json MP_MUTATE_REPLACEMENT=snapshot-replacement.json \
  MP_MUTATE_MARKER="$snapshot_marker" \
  "$RUN" brief-put snapshot-brief snapshot-node snapshot.json --session snapshot-owner
expect_ok test -e "$snapshot_marker"
expect_eq "$(git cat-file blob refs/megapowers/runs/snapshot-brief/nodes/snapshot-node/brief | jq -r '.task')" "Validated snapshot"
expect_eq "$(git cat-file blob refs/megapowers/runs/snapshot-brief/nodes/snapshot-node/brief | jq -r '.ownership[0]')" snapshot

# Foreign, malformed, and timezone-skewed legacy lock refs are inert data.
foreign_lock_ref=refs/megapowers/runs/lock-tz/locks/registry
foreign_lock_oid=$(jq -cn \
  '{pid:1,host:"foreign.example",process_started:"unknown",token:"foreign",acquired_at:"1900-01-01T00:00:00+14:00"}' |
  git hash-object -w --stdin)
git update-ref "$foreign_lock_ref" "$foreign_lock_oid"
write_root_brief lock-tz tz-node tz/ 0 0 0 0 tz-node.json
expect_ok env TZ=Pacific/Honolulu "$RUN" brief-put lock-tz tz-node tz-node.json --session tz-owner
expect_eq "$(git rev-parse "$foreign_lock_ref")" "$foreign_lock_oid"

inaccessible_lock_ref=refs/megapowers/runs/lock-inaccessible/locks/registry
inaccessible_lock_oid=$(printf 'not-json\n' | git hash-object -w --stdin)
git update-ref "$inaccessible_lock_ref" "$inaccessible_lock_oid"
write_root_brief lock-inaccessible inaccessible-node inaccessible/ 0 0 0 0 inaccessible-node.json
expect_ok "$RUN" brief-put lock-inaccessible inaccessible-node inaccessible-node.json --session inaccessible-owner
expect_eq "$(git rev-parse "$inaccessible_lock_ref")" "$inaccessible_lock_oid"

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
demo_generation_before_brief=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ok "$RUN" brief-put demo root-a brief.json --session codex-1
demo_generation_after_brief=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ne "$demo_generation_before_brief" "$demo_generation_after_brief"
expect_eq "$(git cat-file blob "$demo_generation_after_brief" | jq -r '.previous')" "$demo_generation_before_brief"
expect_eq "$(git cat-file blob "$demo_generation_after_brief" | jq -r '.operation')" brief-put
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
demo_generation_after_claim=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ne "$demo_generation_after_brief" "$demo_generation_after_claim"
expect_eq "$(git cat-file blob "$claim_oid" | jq -r '.generation')" "$demo_generation_after_claim"
new_oid=$("$RUN" heartbeat demo root-a --session "$claim_session" --expected "$claim_oid")
demo_generation_after_heartbeat=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ne "$demo_generation_after_claim" "$demo_generation_after_heartbeat"
expect_eq "$(git cat-file blob "$new_oid" | jq -r '.generation')" "$demo_generation_after_heartbeat"
expect_fail "$RUN" release-claim demo root-a --session "$claim_session" --expected "$claim_oid"
expect_ok "$RUN" release-claim demo root-a --session "$claim_session" --expected "$new_oid"
demo_generation_after_release=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ne "$demo_generation_after_heartbeat" "$demo_generation_after_release"

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
demo_generation_before_slot_release=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_fail "$RUN" slot-release demo writer "$slot_n" --session wrong --expected "$slot_oid"
expect_ok "$RUN" slot-release demo writer "$slot_n" --session slot-a-session --expected "$slot_oid"
demo_generation_after_slot_release=$(git rev-parse refs/megapowers/runs/demo/generation)
expect_ne "$demo_generation_before_slot_release" "$demo_generation_after_slot_release"
expect_ok "$RUN" release-claim demo root-a/slot-a --session slot-a-session --expected "$(git rev-parse refs/megapowers/runs/demo/nodes/root-a/slot-a/claim)"
owner_oid=$(git rev-parse refs/megapowers/runs/demo/owner)
expect_fail "$RUN" slot-acquire demo integration @target --session intruder --harness claude --expected-owner "$owner_oid"
target_slot_line=$("$RUN" slot-acquire demo integration @target --session codex-1 --harness codex --expected-owner "$owner_oid")
target_slot_n=${target_slot_line%% *}
target_slot_oid=${target_slot_line#* }
expect_ok "$RUN" slot-release demo integration "$target_slot_n" --session codex-1 --expected "$target_slot_oid"

# A release and reacquire in one second must not recreate an ownership object ID.
write_root_brief claim-aba aba-node aba/ 0 0 0 0 aba-node.json
expect_ok "$RUN" brief-put claim-aba aba-node aba-node.json --session aba-owner
aba_claim_one=$(env PATH="$TMP/date-wrapper:$PATH" "$RUN" claim claim-aba aba-node --session aba-session --harness codex)
aba_claim_one_oid=${aba_claim_one%% *}
aba_claim_one_generation=$(git rev-parse refs/megapowers/runs/claim-aba/generation 2>/dev/null || true)
expect_eq "$(git cat-file blob "$aba_claim_one_oid" | jq -r '.generation')" "$aba_claim_one_generation"
expect_ok env PATH="$TMP/date-wrapper:$PATH" "$RUN" release-claim claim-aba aba-node --session aba-session --expected "$aba_claim_one_oid"
aba_claim_two=$(env PATH="$TMP/date-wrapper:$PATH" "$RUN" claim claim-aba aba-node --session aba-session --harness codex)
aba_claim_two_oid=${aba_claim_two%% *}
expect_ne "$aba_claim_one_oid" "$aba_claim_two_oid"
if env PATH="$TMP/date-wrapper:$PATH" "$RUN" release-claim claim-aba aba-node --session aba-session --expected "$aba_claim_one_oid" >/dev/null 2>&1; then
  bad "stale claim object released its same-second successor"
else
  ok
fi
if git show-ref --verify --quiet refs/megapowers/runs/claim-aba/nodes/aba-node/claim; then
  expect_ok env PATH="$TMP/date-wrapper:$PATH" "$RUN" release-claim claim-aba aba-node --session aba-session --expected "$aba_claim_two_oid"
fi

write_root_brief slot-aba aba-slot aba-slot/ 0 0 1 0 aba-slot.json
expect_ok "$RUN" brief-put slot-aba aba-slot aba-slot.json --session aba-slot-owner
aba_slot_claim=$(env PATH="$TMP/date-wrapper:$PATH" "$RUN" claim slot-aba aba-slot --session aba-slot-session --harness codex)
aba_slot_claim_oid=${aba_slot_claim%% *}
aba_slot_one=$(env PATH="$TMP/date-wrapper:$PATH" "$RUN" slot-acquire slot-aba writer aba-slot --session aba-slot-session --harness codex)
aba_slot_one_oid=${aba_slot_one#* }
aba_slot_one_generation=$(git rev-parse refs/megapowers/runs/slot-aba/generation 2>/dev/null || true)
expect_eq "$(git cat-file blob "$aba_slot_one_oid" | jq -r '.generation')" "$aba_slot_one_generation"
expect_ok env PATH="$TMP/date-wrapper:$PATH" "$RUN" slot-release slot-aba writer 1 --session aba-slot-session --expected "$aba_slot_one_oid"
aba_slot_two=$(env PATH="$TMP/date-wrapper:$PATH" "$RUN" slot-acquire slot-aba writer aba-slot --session aba-slot-session --harness codex)
aba_slot_two_oid=${aba_slot_two#* }
expect_ne "$aba_slot_one_oid" "$aba_slot_two_oid"
if env PATH="$TMP/date-wrapper:$PATH" "$RUN" slot-release slot-aba writer 1 --session aba-slot-session --expected "$aba_slot_one_oid" >/dev/null 2>&1; then
  bad "stale slot object released its same-second successor"
else
  ok
fi
if git show-ref --verify --quiet refs/megapowers/runs/slot-aba/slots/writer/1; then
  expect_ok env PATH="$TMP/date-wrapper:$PATH" "$RUN" slot-release slot-aba writer 1 --session aba-slot-session --expected "$aba_slot_two_oid"
fi
expect_ok env PATH="$TMP/date-wrapper:$PATH" "$RUN" release-claim slot-aba aba-slot --session aba-slot-session --expected "$aba_slot_claim_oid"

# Claim authorization includes the exact scope branch and dependency result refs.
write_root_brief movement branch-node branch/ 0 0 0 0 branch-node.json
expect_ok "$RUN" brief-put movement branch-node branch-node.json --session movement-owner
replacement_head=$(printf 'replacement\n' | git commit-tree "$(git rev-parse "$head^{tree}")" -p "$head")
expect_fail env PATH="$TMP/git-wrapper:$PATH" MP_REAL_GIT="$real_git" \
  MP_MOVE_REF=refs/heads/feature/multi-writer MP_MOVE_FROM="$head" MP_MOVE_TO="$replacement_head" \
  "$RUN" claim movement branch-node --session branch-session --harness codex
git update-ref refs/heads/feature/multi-writer "$head"
if branch_claim_oid=$(git rev-parse --verify refs/megapowers/runs/movement/nodes/branch-node/claim 2>/dev/null); then
  expect_ok "$RUN" release-claim movement branch-node --session branch-session --expected "$branch_claim_oid"
fi

write_root_brief movement dep-node dep/ 0 0 0 0 dep-node.json
jq '.blocked_by=["done-node"]' dep-node.json > dep-node-blocked.json
mv dep-node-blocked.json dep-node.json
expect_ok "$RUN" brief-put movement dep-node dep-node.json --session movement-owner
done_ref=refs/megapowers/runs/movement/nodes/done-node/result
done_oid_one=$(jq -cn --arg head "$head" '{status:"done",head:$head,attempt:1}' | git hash-object -w --stdin)
done_oid_two=$(jq -cn --arg head "$head" '{status:"done",head:$head,attempt:2}' | git hash-object -w --stdin)
git update-ref "$done_ref" "$done_oid_one"
expect_fail env PATH="$TMP/git-wrapper:$PATH" MP_REAL_GIT="$real_git" \
  MP_MOVE_REF="$done_ref" MP_MOVE_FROM="$done_oid_one" MP_MOVE_TO="$done_oid_two" \
  "$RUN" claim movement dep-node --session dep-session --harness codex
if dep_claim_oid=$(git rev-parse --verify refs/megapowers/runs/movement/nodes/dep-node/claim 2>/dev/null); then
  expect_ok "$RUN" release-claim movement dep-node --session dep-session --expected "$dep_claim_oid"
fi

set +e

# Aggregate root budgets must be checked in the same transaction as brief creation.
write_root_brief budget-root budget-a budget-a/ 1 1 0 0 budget-a.json
write_root_brief budget-root budget-b budget-b/ 1 1 0 0 budget-b.json
gate="$TMP/budget-root"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put budget-root budget-a budget-a.json --session budget-owner > "$gate.1.out" 2>&1 & br1=$!
wait_for_file "$gate.ready.1" || bad "root budget race did not reach the first brief mutation"
MP_RACE_GATE="$gate" MP_RACE_ID=2 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put budget-root budget-b budget-b.json --session budget-owner > "$gate.2.out" 2>&1 & br2=$!
expect_not_before_release "$gate.ready.2" "root budget contenders both passed the aggregate check"
: > "$gate.release"
wait "$br1"; brr1=$?
wait "$br2"; brr2=$?
expect_eq "$(( (brr1 == 0 ? 1 : 0) + (brr2 == 0 ? 1 : 0) ))" 1

# Aggregate sibling budgets have the same cross-ref race as root budgets.
write_root_brief budget-child parent parent/ 2 1 0 0 parent.json
expect_ok "$RUN" brief-put budget-child parent parent.json --session child-owner
expect_ok "$RUN" claim budget-child parent --session parent-session --harness codex
git update-ref refs/heads/mp/budget-child/nodes/parent/head "$head"
write_child_brief budget-child parent/one parent parent/one/ 0 0 child-one.json
write_child_brief budget-child parent/two parent parent/two/ 0 0 child-two.json
gate="$TMP/budget-child"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put budget-child parent/one child-one.json --session parent-session > "$gate.1.out" 2>&1 & bc1=$!
wait_for_file "$gate.ready.1" || bad "sibling budget race did not reach the first brief mutation"
MP_RACE_GATE="$gate" MP_RACE_ID=2 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put budget-child parent/two child-two.json --session parent-session > "$gate.2.out" 2>&1 & bc2=$!
expect_not_before_release "$gate.ready.2" "sibling budget contenders both passed the aggregate check"
: > "$gate.release"
wait "$bc1"; bcr1=$?
wait "$bc2"; bcr2=$?
expect_eq "$(( (bcr1 == 0 ? 1 : 0) + (bcr2 == 0 ? 1 : 0) ))" 1

# Different-node claims with overlapping ownership must not both pass an empty peer scan.
write_root_brief claim-overlap overlap-a shared/ 0 0 0 0 overlap-a.json
write_root_brief claim-overlap overlap-b shared/ 0 0 0 0 overlap-b.json
expect_ok "$RUN" brief-put claim-overlap overlap-a overlap-a.json --session overlap-owner
expect_ok "$RUN" brief-put claim-overlap overlap-b overlap-b.json --session overlap-owner
gate="$TMP/claim-overlap"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/claim \
  "$RUN" claim claim-overlap overlap-a --session overlap-a-session --harness codex > "$gate.1.out" 2>&1 & co1=$!
wait_for_file "$gate.ready.1" || bad "overlap race did not reach the first claim mutation"
MP_RACE_GATE="$gate" MP_RACE_ID=2 MP_RACE_REF_MATCH=/claim \
  "$RUN" claim claim-overlap overlap-b --session overlap-b-session --harness claude > "$gate.2.out" 2>&1 & co2=$!
expect_not_before_release "$gate.ready.2" "overlapping claim contenders both passed the peer scan"
: > "$gate.release"
wait "$co1"; cor1=$?
wait "$co2"; cor2=$?
expect_eq "$(( (cor1 == 0 ? 1 : 0) + (cor2 == 0 ? 1 : 0) ))" 1

# One session must not claim two nodes through concurrent empty peer scans.
write_root_brief claim-session session-a session-a/ 0 0 0 0 session-a.json
write_root_brief claim-session session-b session-b/ 0 0 0 0 session-b.json
expect_ok "$RUN" brief-put claim-session session-a session-a.json --session session-owner
expect_ok "$RUN" brief-put claim-session session-b session-b.json --session session-owner
gate="$TMP/claim-session"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/claim \
  "$RUN" claim claim-session session-a --session shared-session --harness codex > "$gate.1.out" 2>&1 & cs1=$!
wait_for_file "$gate.ready.1" || bad "same-session race did not reach the first claim mutation"
MP_RACE_GATE="$gate" MP_RACE_ID=2 MP_RACE_REF_MATCH=/claim \
  "$RUN" claim claim-session session-b --session shared-session --harness codex > "$gate.2.out" 2>&1 & cs2=$!
expect_not_before_release "$gate.ready.2" "same-session claim contenders both passed the peer scan"
: > "$gate.release"
wait "$cs1"; csr1=$?
wait "$cs2"; csr2=$?
expect_eq "$(( (csr1 == 0 ? 1 : 0) + (csr2 == 0 ? 1 : 0) ))" 1

# Two acquisitions for one node must not install distinct writer slots.
write_root_brief slot-duplicate slot-node slot-node/ 0 0 1 0 slot-node.json
expect_ok "$RUN" brief-put slot-duplicate slot-node slot-node.json --session slot-owner
expect_ok "$RUN" claim slot-duplicate slot-node --session slot-session --harness codex
gate="$TMP/slot-duplicate"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/slots/writer/ \
  "$RUN" slot-acquire slot-duplicate writer slot-node --session slot-session --harness codex > "$gate.1.out" 2>&1 & sd1=$!
wait_for_file "$gate.ready.1" || bad "duplicate slot race did not reach the first slot mutation"
MP_RACE_GATE="$gate" MP_RACE_ID=2 MP_RACE_REF_MATCH=/slots/writer/ \
  "$RUN" slot-acquire slot-duplicate writer slot-node --session slot-session --harness codex > "$gate.2.out" 2>&1 & sd2=$!
expect_not_before_release "$gate.ready.2" "same-node slot contenders both passed the duplicate scan"
: > "$gate.release"
wait "$sd1"; sdr1=$?
wait "$sd2"; sdr2=$?
expect_eq "$(( (sdr1 == 0 ? 1 : 0) + (sdr2 == 0 ? 1 : 0) ))" 1
expect_eq "$(git for-each-ref --format='%(refname)' refs/megapowers/runs/slot-duplicate/slots/writer/ | wc -l | tr -d ' ')" 1

# Root owner and target-head validation must remain current through brief creation.
write_root_brief stale-root stale-root stale-root/ 0 0 0 0 stale-root.json
stale_owner_ref=refs/megapowers/runs/stale-root/owner
stale_owner_oid=$(git rev-parse "$stale_owner_ref")
replacement_owner_oid=$(jq -cn '{session:"replacement",harness:"claude",claimed_at:"now",last_activity:"now"}' | git hash-object -w --stdin)
replacement_head=$(printf 'replacement\n' | git commit-tree "$(git rev-parse "$head^{tree}")" -p "$head")
expect_fail env PATH="$TMP/git-wrapper:$PATH" MP_REAL_GIT="$real_git" \
  MP_MOVE_REF=refs/heads/feature/multi-writer MP_MOVE_FROM="$head" MP_MOVE_TO="$replacement_head" \
  "$RUN" brief-put stale-root stale-root stale-root.json --session stale-owner
git update-ref refs/heads/feature/multi-writer "$head"
gate="$TMP/stale-root"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put stale-root stale-root stale-root.json --session stale-owner > "$gate.1.out" 2>&1 & srp=$!
wait_for_file "$gate.ready.1" || bad "stale root test did not reach brief mutation"
(git update-ref "$stale_owner_ref" "$replacement_owner_oid" "$stale_owner_oid" 2>/dev/null && : > "$gate.owner-done") & sro=$!
(git update-ref refs/heads/feature/multi-writer "$replacement_head" "$head" 2>/dev/null && : > "$gate.head-done") & srh=$!
expect_not_before_release "$gate.owner-done" "owner changed after validation but before root brief creation"
expect_not_before_release "$gate.head-done" "target head changed after validation but before root brief creation"
: > "$gate.release"
wait "$srp"; srr=$?
wait "$sro" || true
wait "$srh" || true
expect_eq "$srr" 0
git update-ref refs/heads/feature/multi-writer "$head"

# A parent claim release must not invalidate an in-flight child brief authorization.
write_root_brief stale-parent stale-parent stale-parent/ 2 1 0 0 stale-parent.json
expect_ok "$RUN" brief-put stale-parent stale-parent stale-parent.json --session stale-parent-owner
stale_parent_claim=$("$RUN" claim stale-parent stale-parent --session stale-parent-session --harness codex)
stale_parent_oid=${stale_parent_claim%% *}
git update-ref refs/heads/mp/stale-parent/nodes/stale-parent/head "$head"
write_child_brief stale-parent stale-parent/child stale-parent stale-parent/child/ 0 0 stale-child.json
gate="$TMP/stale-parent"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/brief \
  "$RUN" brief-put stale-parent stale-parent/child stale-child.json --session stale-parent-session > "$gate.1.out" 2>&1 & spb=$!
wait_for_file "$gate.ready.1" || bad "stale parent test did not reach child brief mutation"
("$RUN" release-claim stale-parent stale-parent --session stale-parent-session --expected "$stale_parent_oid" && : > "$gate.release-done") & spr=$!
expect_not_before_release "$gate.release-done" "parent claim was released before child brief creation"
: > "$gate.release"
wait "$spb"; spbr=$?
wait "$spr"; sprr=$?
expect_eq "$spbr:$sprr" 0:0

# Node-claim validation and writer slot creation must share one transaction boundary.
write_root_brief stale-slot-claim stale-slot stale-slot/ 0 0 1 0 stale-slot.json
expect_ok "$RUN" brief-put stale-slot-claim stale-slot stale-slot.json --session stale-slot-owner
stale_slot_claim=$("$RUN" claim stale-slot-claim stale-slot --session stale-slot-session --harness codex)
stale_slot_oid=${stale_slot_claim%% *}
gate="$TMP/stale-slot-claim"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/slots/writer/ \
  "$RUN" slot-acquire stale-slot-claim writer stale-slot --session stale-slot-session --harness codex > "$gate.1.out" 2>&1 & ssa=$!
wait_for_file "$gate.ready.1" || bad "stale slot claim test did not reach slot mutation"
("$RUN" release-claim stale-slot-claim stale-slot --session stale-slot-session --expected "$stale_slot_oid" && : > "$gate.release-done") & ssr=$!
expect_not_before_release "$gate.release-done" "node claim was released before writer slot creation"
: > "$gate.release"
wait "$ssa"; ssar=$?
wait "$ssr"; ssrr=$?
expect_eq "$ssar:$ssrr" 0:0

# Target-owner validation must remain current through integration slot creation.
target_owner_ref=refs/megapowers/runs/stale-slot-owner/owner
target_owner_oid=$(git rev-parse "$target_owner_ref")
replacement_target_owner_oid=$(jq -cn '{session:"replacement",harness:"claude",claimed_at:"now",last_activity:"now"}' | git hash-object -w --stdin)
gate="$TMP/stale-slot-owner"
MP_RACE_GATE="$gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/slots/integration/ \
  "$RUN" slot-acquire stale-slot-owner integration @target --session target-owner --harness codex --expected-owner "$target_owner_oid" > "$gate.1.out" 2>&1 & soa=$!
wait_for_file "$gate.ready.1" || bad "stale slot owner test did not reach slot mutation"
(git update-ref "$target_owner_ref" "$replacement_target_owner_oid" "$target_owner_oid" 2>/dev/null && : > "$gate.owner-done") & soo=$!
expect_not_before_release "$gate.owner-done" "target owner changed before integration slot creation"
: > "$gate.release"
wait "$soa"; soar=$?
wait "$soo" || true
expect_eq "$soar" 0

# SIGKILL cannot split a generation update from its registry mutation.
write_root_brief generation-crash crash-node crash/shared/ 0 0 0 0 crash-node.json
write_root_brief generation-crash crash-peer crash/shared/peer/ 0 0 0 0 crash-peer.json
expect_ok "$RUN" brief-put generation-crash crash-node crash-node.json --session crash-owner
expect_ok "$RUN" brief-put generation-crash crash-peer crash-peer.json --session crash-owner
crash_generation_before=$(git rev-parse refs/megapowers/runs/generation-crash/generation 2>/dev/null || true)
crash_gate="$TMP/generation-crash"
MP_RACE_GATE="$crash_gate" MP_RACE_ID=1 MP_RACE_REF_MATCH=/claim \
  "$RUN" claim generation-crash crash-node --session crash-session --harness codex > "$crash_gate.out" 2>&1 & crash_pid=$!
wait_for_file "$crash_gate.ready.1" || bad "generation crash test did not reach claim mutation"
kill -KILL "$crash_pid"
wait "$crash_pid" 2>/dev/null
(
  "$RUN" claim generation-crash crash-peer --session crash-peer-session --harness claude \
    > "$crash_gate.peer.out" 2>&1
  printf '%s\n' "$?" > "$crash_gate.peer.result"
) & crash_peer_pid=$!
expect_not_before_release "$crash_gate.peer.result" "contender bypassed a prepared generation transaction"
: > "$crash_gate.release"
crash_wait=0
while ! crash_claim_oid=$(git rev-parse --verify refs/megapowers/runs/generation-crash/nodes/crash-node/claim 2>/dev/null) &&
      [ "$crash_wait" -lt 200 ]; do
  sleep 0.01
  crash_wait=$((crash_wait + 1))
done
[ -n "${crash_claim_oid:-}" ] && ok || bad "orphaned claim transaction did not finish"
wait "$crash_peer_pid" 2>/dev/null
expect_eq "$(cat "$crash_gate.peer.result")" 4
crash_generation_after=$(git rev-parse refs/megapowers/runs/generation-crash/generation 2>/dev/null || true)
expect_ne "$crash_generation_before" "$crash_generation_after"
expect_eq "$(git cat-file blob "$crash_claim_oid" | jq -r '.generation')" "$crash_generation_after"
expect_ok "$RUN" release-claim generation-crash crash-node --session crash-session --expected "$crash_claim_oid"

printf '== sdd-run tests: %d passed, %d failed ==\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
