#!/usr/bin/env bash
# security-lint.sh - implementation is security-lint.go.
set -euo pipefail
dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MEGAPOWERS_ROOT="$(cd "$dir/.." && pwd)"
export MEGAPOWERS_ROOT
cache="${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}"
if ! mkdir -p "$cache" 2>/dev/null || [[ ! -w $cache ]]; then
  cache="${TMPDIR:-/tmp}/megapowers-gocache"
  mkdir -p "$cache" || { echo "security-lint: no writable Go cache" >&2; exit 2; }
fi
export GOCACHE="$cache"
command -v go >/dev/null || { echo "security-lint: go is required" >&2; exit 2; }
exec go run "$dir/security-lint.go" "$@"
