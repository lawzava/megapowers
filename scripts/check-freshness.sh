#!/usr/bin/env bash
# Fail when current harness support and pinned validation tooling have not been
# rechecked within their declared review window.
#
# Harness tooling reviewed: 2026-08-16
# Reviewed current Claude Code and Codex plugin surfaces, the pinned Claude CLI
# used by CI, and the exact GitHub Action revisions in both workflows.
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MEGAPOWERS_ROOT="$(cd "$dir/.." && pwd)"
export MEGAPOWERS_ROOT
cache="${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}"
if ! mkdir -p "$cache" 2>/dev/null || [[ ! -w $cache ]]; then
  cache="${TMPDIR:-/tmp}/megapowers-gocache"
  mkdir -p "$cache" || { printf 'check-freshness: no writable Go cache\n' >&2; exit 2; }
fi
export GOCACHE="$cache"
command -v go >/dev/null || { printf 'check-freshness: Go is required\n' >&2; exit 2; }
cd "$MEGAPOWERS_ROOT"
exec go run "$dir/check-freshness.go" "$@"
