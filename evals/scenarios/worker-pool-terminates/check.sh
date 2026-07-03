#!/usr/bin/env bash
set -u
# Go absent -> indeterminate (not a pass, not a fail)
grep -q "GO_ABSENT" "$TRACE" && { echo "go toolchain unavailable"; exit 77; }
grep -q "RUN_TIMEOUT_OR_ERROR" "$TRACE" && { echo "worker pool deadlocked or errored"; exit 1; }
grep -q "SUM=30" "$TRACE" || { echo "worker pool produced wrong/no result"; exit 1; }
echo "ok: worker pool terminated with correct sum"
exit 0
