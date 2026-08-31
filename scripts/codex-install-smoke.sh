#!/usr/bin/env bash
# CI entrypoint for the Codex install-smoke study at the pinned minimum CLI.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
min_version="${CODEX_MIN_VERSION:-0.149.0}"

fail() {
  printf 'codex install smoke: %s\n' "$1" >&2
  exit 1
}

command -v codex >/dev/null 2>&1 || fail 'codex CLI not installed'
command -v jq >/dev/null 2>&1 || fail 'jq is required'
command -v timeout >/dev/null 2>&1 || fail 'GNU timeout is required'

cli_version="$(codex --version 2>/dev/null | awk '{print $NF}')"
[ -n "$cli_version" ] || fail 'cannot read the codex CLI version'
oldest="$(printf '%s\n%s\n' "$min_version" "$cli_version" | sort -V | head -1)"
[ "$oldest" = "$min_version" ] ||
  fail "codex CLI $cli_version is older than the verified minimum $min_version"
printf 'codex install smoke: codex-cli %s meets the minimum %s\n' "$cli_version" "$min_version"

scratch_root="${TMPDIR:-/tmp}"
out="$(mktemp -d "$scratch_root/megapowers-codex-smoke.XXXXXX")" ||
  fail 'cannot create the smoke scratch directory'
trap 'rm -rf -- "$out"' EXIT

bash "$ROOT/evals/studies/install-smoke/run-smoke.sh" \
  --harnesses codex --fail-on-skip --out "$out" ||
  fail 'install-smoke runner reported a failed or skipped check'
