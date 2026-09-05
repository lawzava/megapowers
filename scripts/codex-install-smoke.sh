#!/bin/sh
# Compatibility entrypoint. Codex smoke policy lives in Go.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
export MEGAPOWERS_ROOT="$repo_root"
MEGAPOWERS_CALLER_CWD=$(pwd -P)
export MEGAPOWERS_CALLER_CWD
export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}
mkdir -p "$GOCACHE"
cd "$repo_root"
exec go run "$repo_root/scripts/cmd/maintainer" codex-install-smoke "$@"
