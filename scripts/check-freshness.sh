#!/usr/bin/env bash
# check-freshness.sh — fail when a dated opinion has gone unreviewed too long.
#
# The repo's opinions rot on a clock, not on a commit: model IDs, delegate
# routes, stack picks, and published eval numbers age even when no one touches
# the files. Each opinion-bearing file carries a "Last reviewed:"/"Last run:"
# date; this script fails when any of them is older than its review window
# (90 days by default, overridable per file; an explicit --max-age-days applies
# to every entry). A monthly scheduled CI job runs it, so staleness surfaces as
# a failed run instead of rotting silently. To clear a failure: re-review the
# file's opinions (update or confirm them), then bump its date.
#
#   scripts/check-freshness.sh [--max-age-days N]
#
# validate.sh calls this with a huge --max-age-days as a format guard: it
# proves every listed file still carries a parseable date line.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2

DEFAULT_MAX_AGE=90
MAX_AGE_DAYS=$DEFAULT_MAX_AGE
FLAG_SET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --max-age-days) MAX_AGE_DAYS="$2"; FLAG_SET=1; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# file | the dated line's marker (grep -i) | optional per-file max-age-days.
# Add a line here when a new file gains time-sensitive opinions. The third
# field overrides the default window for that entry when no --max-age-days is
# passed; an explicit --max-age-days applies to every entry (validate.sh passes
# a huge value as a pure date-line format guard).
#
# Codex config reviewed: 2026-07-12
#   Sentinel for the Codex-facing surface (templates/codex-config.toml and the
#   docs/setup.md Codex section), neither of which carries a date line of its
#   own. Codex ships weekly, so this entry rides a 30-day window: re-review those
#   two surfaces and bump the date on this comment line to clear a failure.
FILES='docs/harness-support.md|Last reviewed:
evals/RESULTS.md|Last run:
plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml|Last reviewed:
plugins/megapowers/models.toml|Last reviewed:
plugins/mega-orchestration/skills/orchestrating/references/harness-primitives.md|Last reviewed:|30
plugins/mega-frontend/skills/designing-frontends/SKILL.md|Calibration reviewed:
scripts/check-freshness.sh|Codex config reviewed:|30'

pass=0
fail=0
ok()  { printf '  \033[32mPASS\033[0m %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=$((fail + 1)); }

epoch_of() { # YYYY-MM-DD -> seconds, GNU date first, BSD fallback
  date -d "$1" +%s 2>/dev/null || date -j -f '%Y-%m-%d' "$1" +%s 2>/dev/null
}

now="$(date +%s)"
echo "== freshness (max age: ${MAX_AGE_DAYS} days) =="
while IFS='|' read -r file marker maxage; do
  [ -n "$file" ] || continue
  if [ ! -f "$file" ]; then bad "$file missing"; continue; fi
  d="$(grep -i "$marker" "$file" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)"
  if [ -z "$d" ]; then bad "$file: no '$marker YYYY-MM-DD' line found"; continue; fi
  then_s="$(epoch_of "$d")"
  if [ -z "$then_s" ]; then bad "$file: unparseable date '$d'"; continue; fi
  # Effective window: an explicit --max-age-days applies to every entry (the
  # validate.sh format guard); otherwise a per-file third field overrides the
  # default for that entry alone.
  if [ "$FLAG_SET" -eq 1 ]; then limit="$MAX_AGE_DAYS"; else limit="${maxage:-$DEFAULT_MAX_AGE}"; fi
  age=$(( (now - then_s) / 86400 ))
  if [ "$age" -le "$limit" ]; then
    ok "$file reviewed ${age}d ago ($d)"
  else
    bad "$file reviewed ${age}d ago ($d) — re-review its opinions, then bump the date"
  fi
done <<< "$FILES"

echo "== summary: $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
