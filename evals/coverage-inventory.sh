#!/bin/sh
# Compatibility entrypoint. Coverage inventory logic lives in Go.
set -eu
evals_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$evals_dir/.." && pwd)
export MEGAPOWERS_ROOT="$repo_root"
MEGAPOWERS_CALLER_CWD=$(pwd -P)
export MEGAPOWERS_CALLER_CWD
export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}
mkdir -p "$GOCACHE"
cd "$repo_root"
exec go run "$repo_root/evals/cmd/evaltool" coverage-inventory "$@"
