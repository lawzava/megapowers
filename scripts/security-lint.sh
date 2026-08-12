#!/usr/bin/env bash
# security-lint.sh — implementation is security-lint.go.
set -euo pipefail
dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MEGAPOWERS_ROOT="$(cd "$dir/.." && pwd)"
export MEGAPOWERS_ROOT
src="$dir/security-lint.go"
cache="${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}"
mkdir -p "$cache"
bin="$cache/security-lint-$(cksum "$src" | awk '{print $1}')"
if [[ ! -x $bin || $src -nt $bin ]]; then
  command -v go >/dev/null || { echo "security-lint: go is required" >&2; exit 2; }
  go build -o "$bin" "$src" || exit 2
fi
exec "$bin" "$@"
