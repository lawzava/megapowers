#!/bin/sh
# Compatibility entrypoint. Source review checks live in Go.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
export MEGAPOWERS_ROOT="$repo_root"
cd "$repo_root"
exec go run ./scripts/cmd/check-freshness "$@"
