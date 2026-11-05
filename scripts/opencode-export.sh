#!/bin/sh
# Repository-only entrypoint. OpenCode export logic lives in Go.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
cd "$repo_root"
exec go run ./scripts/cmd/opencode-export "$@"
