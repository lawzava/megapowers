#!/usr/bin/env bash
set -uo pipefail
out="$WORKDIR/out.txt"
[ -f "$out" ] || { echo "missing out.txt"; exit 1; }
markers='plan-fields private-refs bounded-worktrees bounded-cache branch-ownership sequential-when-recursive-commit-authorization-absent sequential-commit-authorization recursive-explicit-commit-authorization recursive-selection-not-authorization commit-authorization-boundary coordinator-result release-lifecycle no-inexact-writer-release owner-target no-stale-takeover fail-closed codex-fresh codex-depth-five claude-no-teams no-recursive-agent-teams formal-policy policy-fixtures codex-lead-rule claude-lead-rule registry-tests worktree-tests'
for marker in $markers; do
  grep -q "^OK $marker$" "$out" || { echo "missing marker: $marker"; exit 1; }
done
grep -q '^MISSING ' "$out" && { grep '^MISSING ' "$out"; exit 1; }
echo "ok: recursive multi-writer contract complete"
