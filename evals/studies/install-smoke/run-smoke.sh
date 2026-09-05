#!/bin/sh
# Compatibility entrypoint. Install-smoke policy lives in Go.
set -eu
here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$here/../../.." && pwd)
export MEGAPOWERS_ROOT="$repo_root"
MEGAPOWERS_CALLER_CWD=$(pwd -P)
export MEGAPOWERS_CALLER_CWD
export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}
mkdir -p "$GOCACHE"
cd "$repo_root"
exec go run "$repo_root/scripts/cmd/maintainer" install-smoke "$@"
