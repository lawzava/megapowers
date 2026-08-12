#!/usr/bin/env bash
# check-enforcement.sh: keep the enforcement lifecycle honest.
# Implementation is check-enforcement.go.
set -euo pipefail
dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MEGAPOWERS_ROOT="$(cd "$dir/.." && pwd)"
export MEGAPOWERS_ROOT
src="$dir/check-enforcement.go"
cache="${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}"
mkdir -p "$cache"
bin="$cache/check-enforcement-$(cksum "$src" | awk '{print $1}')"
if [[ ! -x $bin || $src -nt $bin ]]; then
  command -v go >/dev/null || { echo "check-enforcement: go is required" >&2; exit 2; }
  go build -o "$bin" "$src" || exit 2
fi
exec "$bin" "$@"
