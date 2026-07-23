#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$HERE/ownership-preflight"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
passed=0
failed=0

run_case() {
  local name="$1" want="$2" file="$3" rc=0
  "$CHECK" "$file" >"$scratch/out" 2>"$scratch/err" || rc=$?
  if [ "$want" = pass ] && [ "$rc" -eq 0 ]; then
    echo "ok   $name"; passed=$((passed + 1))
  elif [ "$want" = fail ] && [ "$rc" -ne 0 ]; then
    echo "ok   $name"; passed=$((passed + 1))
  else
    echo "FAIL $name (rc=$rc)"; cat "$scratch/err"
    failed=$((failed + 1))
  fi
}

write_plan() {
  local file="$1" body="$2"
  printf '%s\n' "$body" > "$file"
}

write_plan "$scratch/good.md" '### Task 1: Router
**Parallel safety:** Parallel with Task 2
**Ownership:** `plugins/router/`
### Task 2: Docs
**Parallel safety:** Parallel with Task 1
**Ownership:** `docs/router.md`, `README.md`'
run_case "disjoint parallel roots pass" pass "$scratch/good.md"

write_plan "$scratch/duplicate.md" '### Task 1: A
**Parallel safety:** Parallel with Task 2
**Ownership:** `plugins/shared/file.sh`
### Task 2: B
**Parallel safety:** Parallel with Task 1
**Ownership:** `plugins/shared/file.sh`'
run_case "duplicate parallel file fails" fail "$scratch/duplicate.md"

write_plan "$scratch/ancestor.md" '### Task 1: A
**Parallel safety:** Parallel with Task 2
**Ownership:** `plugins/shared/`
### Task 2: B
**Parallel safety:** Parallel with Task 1
**Ownership:** `plugins/shared/file.sh`'
run_case "parallel parent-child overlap fails" fail "$scratch/ancestor.md"

write_plan "$scratch/glob.md" '### Task 1: A
**Parallel safety:** Sequential
**Ownership:** `plugins/*.sh`'
run_case "glob ownership fails" fail "$scratch/glob.md"

write_plan "$scratch/missing.md" '### Task 1: A
**Parallel safety:** Parallel with Task 9
**Ownership:** `plugins/a/`'
run_case "missing parallel task fails" fail "$scratch/missing.md"

write_plan "$scratch/no-owner.md" '### Task 1: A
**Parallel safety:** Sequential'
run_case "missing ownership fails" fail "$scratch/no-owner.md"

write_plan "$scratch/duplicate-task.md" '### Task 1: A
**Parallel safety:** Sequential
**Ownership:** `plugins/a/`
### Task 1: B
**Parallel safety:** Sequential
**Ownership:** `plugins/b/`'
run_case "duplicate task number fails" fail "$scratch/duplicate-task.md"

write_plan "$scratch/missing-prereq.md" '### Task 1: A
**Parallel safety:** Parallel after Task 7
**Ownership:** `plugins/a/`'
run_case "missing prerequisite task fails" fail "$scratch/missing-prereq.md"

write_plan "$scratch/sibling-overlap.md" '### Task 1: Root
**Parallel safety:** Sequential
**Ownership:** `plugins/root/`
### Task 2: A
**Parallel safety:** Parallel after Task 1
**Ownership:** `plugins/shared/`
### Task 3: B
**Parallel safety:** Parallel after Task 1
**Ownership:** `plugins/shared/file.sh`'
run_case "parallel-after sibling overlap fails" fail "$scratch/sibling-overlap.md"

write_plan "$scratch/ordered-reuse.md" '### Task 1: Root
**Parallel safety:** Sequential
**Ownership:** `plugins/root/`
### Task 2: Expand
**Parallel safety:** Parallel after Task 1
**Ownership:** `plugins/shared/file.sh`
### Task 3: Contract
**Parallel safety:** Parallel after Task 2
**Ownership:** `plugins/shared/file.sh`'
run_case "ordered tasks may reuse ownership" pass "$scratch/ordered-reuse.md"

echo "== $passed passed, $failed failed =="
[ "$failed" -eq 0 ]
