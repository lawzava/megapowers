#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
validator="$ROOT/scripts/validate.sh"

fail() {
  printf 'validation contract: %s\n' "$*" >&2
  exit 1
}

for removed in OpenCode opencode 'seven plugin' 'model catalog' models.toml delegates.toml \
  skill-router session-metrics copied-agent; do
  if grep -qiF "$removed" "$validator"; then
    fail "validator retains removed surface: $removed"
  fi
done

grep -q 'timeout' "$validator" || fail 'validator has no bounded command runner'
grep -qF 'plugins/megapowers' "$validator" || fail 'validator does not name the sole plugin'
grep -qF '.claude-plugin/marketplace.json' "$validator" || fail 'Claude marketplace is not validated'
grep -qF '.agents/plugins/marketplace.json' "$validator" || fail 'Codex marketplace is not validated'
grep -qF 'grep -qF "## $claude_version - " CHANGELOG.md' "$validator" ||
  fail 'pre-stamp release candidate cannot validate its current manifest version'
if grep -q 'release_version=.*head -1' "$validator"; then
  fail 'validator deadlocks changelog-first release candidates on the newest entry'
fi
grep -qF 'git ls-files "plugins/*"' "$validator" ||
  fail 'plugin inventory is not derived from the tracked release artifact'
if grep -qF 'find plugins -mindepth 1 -maxdepth 1 -type d' "$validator"; then
  fail 'ignored local plugin residue can make validation environment-dependent'
fi

printf 'validation contract: ok\n'
