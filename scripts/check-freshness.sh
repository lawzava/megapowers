#!/usr/bin/env bash
# check-freshness.sh — fail when a dated opinion has gone unreviewed too long.
#
# The repo's opinions rot on a clock, not on a commit: model IDs, delegate
# routes, stack picks, and published eval numbers age even when no one touches
# the files. Each opinion-bearing file carries a "Last reviewed:"/"Last run:"
# date; this script fails when any of them is older than its review window
# (30 days by default, overridable per file; an explicit --max-age-days applies
# to every entry). A weekly scheduled CI job runs it, so staleness surfaces as
# a failed run instead of rotting silently. To clear a failure: re-review the
# file's opinions (update or confirm them), then bump its date.
#
#   scripts/check-freshness.sh [--max-age-days N]
#
# Implementation is check-freshness.go.
#
# Codex config reviewed: 2026-08-12
#   Sentinel for the Codex-facing surface (templates/codex-config.toml and the
#   docs/setup.md Codex section), neither of which carries a date line of its
#   own. Codex ships weekly, so this entry rides a 30-day window: re-review those
#   two surfaces and bump the date on this comment line to clear a failure.
#   2026-08-12: checked against codex-cli 0.147.0. The config keys the template
#   sets all still parse; the marketplace verbs are unchanged. New that release:
#   a skills context budget that shortens skill descriptions once too many
#   plugins are enabled, now documented in the docs/setup.md Codex section.
set -euo pipefail
dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MEGAPOWERS_ROOT="$(cd "$dir/.." && pwd)"
export MEGAPOWERS_ROOT
src="$dir/check-freshness.go"
cache="${GOCACHE:-${TMPDIR:-/tmp}/megapowers-gocache}"
mkdir -p "$cache"
bin="$cache/check-freshness-$(cksum "$src" | awk '{print $1}')"
if [[ ! -x $bin || $src -nt $bin ]]; then
  command -v go >/dev/null || { echo "check-freshness: go is required" >&2; exit 2; }
  go build -o "$bin" "$src" || exit 2
fi
cd "$MEGAPOWERS_ROOT"
exec "$bin" "$@"
