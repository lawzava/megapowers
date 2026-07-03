#!/usr/bin/env bash
set -u
grep -q PY_ABSENT "$TRACE" && { echo "python3 unavailable"; exit 77; }
grep -q RUN_TIMEOUT_OR_ERROR "$WORKDIR/run.out" 2>/dev/null && { echo "worker pool deadlocked/errored"; exit 1; }
grep -q "SUM=30" "$WORKDIR/run.out" 2>/dev/null || { echo "wrong/no result from worker pool"; exit 1; }
echo "ok: python async worker pool terminated correctly"
