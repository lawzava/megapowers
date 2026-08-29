#!/usr/bin/env bash
# CI install smoke for the Codex harness: verifies a pinned-minimum CLI can
# register the local marketplace, install the plugin into a fresh CODEX_HOME,
# and discover the shipped hooks without executing them. Reuses the
# install-smoke study runner for the registration checks, then installs into a
# home this script owns to compare the full installed tree (files and modes)
# with the repository.
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
home="$(mktemp -d "$scratch_root/megapowers-codex-home.XXXXXX")" ||
  fail 'cannot create the smoke config home'
trap 'rm -rf -- "$home" "$out"' EXIT

bash "$ROOT/evals/studies/install-smoke/run-smoke.sh" \
  --harnesses codex --fail-on-skip --out "$out" ||
  fail 'install-smoke runner reported a failed or skipped check'

expected_version="$(jq -er .version "$ROOT/plugins/megapowers/.codex-plugin/plugin.json")"

if ! CODEX_HOME="$home" timeout 300 codex plugin marketplace add "$ROOT" --json \
    >"$out/marketplace.json" 2>"$out/marketplace.err"; then
  cat "$out/marketplace.err" >&2
  fail 'marketplace add failed'
fi
if ! CODEX_HOME="$home" timeout 300 codex plugin add megapowers@megapowers --json \
    >"$out/install.json" 2>"$out/install.err"; then
  cat "$out/install.err" >&2
  fail 'plugin add failed'
fi

install_path="$(jq -er '.installedPath' "$out/install.json")" ||
  fail 'plugin add JSON is missing installedPath'
installed_version="$(jq -er '.version' "$out/install.json")"
[ "$installed_version" = "$expected_version" ] ||
  fail "installed manifest version $installed_version is not $expected_version"
printf 'codex install smoke: megapowers %s installed at %s\n' "$expected_version" "$install_path"

if ! CODEX_HOME="$home" timeout 120 codex plugin list --json \
    >"$out/list.json" 2>"$out/list.err"; then
  cat "$out/list.err" >&2
  fail 'plugin list failed'
fi
if ! jq -e --arg v "$expected_version" \
    '.installed[] | select(.pluginId == "megapowers@megapowers" and .version == $v and .installed == true and .enabled == true)' \
    "$out/list.json" >/dev/null; then
  fail 'installed plugin missing from registration JSON'
fi

repo_tree="$(cd "$ROOT/plugins/megapowers" && find . -type f -printf '%m %p\n' | LC_ALL=C sort)"
installed_tree="$(cd "$install_path" && find . -type f -printf '%m %p\n' | LC_ALL=C sort)"
if [ "$repo_tree" != "$installed_tree" ]; then
  diff <(printf '%s\n' "$repo_tree") <(printf '%s\n' "$installed_tree") >&2 || true
  fail 'installed plugin tree differs from the repository tree (files or modes)'
fi
printf 'codex install smoke: installed tree matches the repository tree (%s files, modes included)\n' \
  "$(printf '%s\n' "$repo_tree" | wc -l)"

hook_manifest="$install_path/hooks/hooks.json"
cmp -s "$ROOT/plugins/megapowers/hooks/hooks.json" "$hook_manifest" ||
  fail 'installed hooks.json differs from the repository manifest'
hook_count="$(jq -er '[.hooks[][] | .hooks[] | .command] | length' "$hook_manifest")"
[ "$hook_count" -eq 2 ] ||
  fail "expected 2 discovered hook commands, found: $hook_count"
hook_commands="$(jq -r '.hooks[][] | .hooks[] | .command' "$hook_manifest")"
if printf '%s\n' "$hook_commands" | grep -qvF -- '${CLAUDE_PLUGIN_ROOT}/hooks/run-hook.cmd '; then
  fail "unexpected hook command discovered: $hook_commands"
fi
printf 'codex install smoke: discovered %s hook commands, all through run-hook.cmd (no hooks executed)\n' "$hook_count"
