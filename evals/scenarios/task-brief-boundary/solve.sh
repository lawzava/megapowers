#!/usr/bin/env bash
# Run the shipped task-brief helper for the LAST task and for Task 1.
set -e
sb="$ROOT/plugins/megapowers/skills/subagent-driven-development/scripts/task-brief"
"$sb" plan.md 2 brief-2.md
"$sb" plan.md 1 brief-1.md
