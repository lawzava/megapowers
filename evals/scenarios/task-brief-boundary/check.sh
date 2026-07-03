#!/usr/bin/env bash
# Oracle: Task 2's brief must contain Task 2 but NOT the trailing sections;
# Task 1's brief must contain its own subsection but NOT Task 2.
set -u
b2="$WORKDIR/brief-2.md"; b1="$WORKDIR/brief-1.md"
[ -f "$b2" ] && [ -f "$b1" ] || { echo "briefs not produced"; exit 1; }

grep -q "Task 2: Second thing" "$b2" || { echo "task 2 brief missing its own heading"; exit 1; }
grep -q "Self-Review"       "$b2" && { echo "LEAK: Self-Review bled into task 2 brief"; exit 1; }
grep -q "Final Verification" "$b2" && { echo "LEAK: Final Verification bled into task 2 brief"; exit 1; }

grep -q "context that mentions Task 1" "$b1" || { echo "task 1 brief dropped its 'Task 1 notes' subsection"; exit 1; }
grep -q "second step body"  "$b1" || { echo "task 1 brief dropped Step 2 (taskdepth reset by a deeper Task-named heading)"; exit 1; }
grep -q "Task 2: Second thing" "$b1" && { echo "LEAK: task 2 bled into task 1 brief"; exit 1; }

echo "ok: task briefs bounded correctly"
exit 0
