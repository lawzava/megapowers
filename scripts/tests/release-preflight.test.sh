#!/bin/sh
# Compatibility entrypoint. Contract logic lives in Go.
set -eu
here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$here/../.." && pwd)
export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}
mkdir -p "$GOCACHE"
cd "$repo_root"
exec go test ./scripts/internal/maintain -run 'TestReleasePreflight' -count=1
