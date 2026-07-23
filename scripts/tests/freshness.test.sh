#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
script="$ROOT/scripts/check-freshness.sh"
workflow="$ROOT/.github/workflows/freshness.yml"

grep -q '^DEFAULT_MAX_AGE=30$' "$script"
grep -q "cron: '17 6 \\* \\* 1'" "$workflow"
grep -q 'reviewed within 30 days' "$workflow"

echo "freshness contract: ok"
