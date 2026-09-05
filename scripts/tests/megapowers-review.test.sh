#!/bin/sh
# Compatibility entrypoint for the Go review regression suite.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
cd "$repo_root"
exec go test ./scripts/contracts -run "^TestReview" -count=1 "$@"
