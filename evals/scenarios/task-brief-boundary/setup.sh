#!/usr/bin/env bash
# Seed a plan whose LAST task (Task 2) is followed by non-task sections, and whose
# Task 1 contains a DEEPER heading that itself mentions "Task 1" (a subsection),
# to guard the taskdepth-reset regression an independent cross-model pass found.
set -e
cat > plan.md <<'EOF'
# Some Plan

### Task 1: First thing
Do the first thing.
#### Task 1 notes
context that mentions Task 1
#### Step 2
second step body

### Task 2: Second thing
Do the second thing.

## Self-Review
This is NOT part of task 2.

## Final Verification
Also not a task.
EOF
