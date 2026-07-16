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

printf '== sdd-run tests: %d passed, %d failed ==\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
