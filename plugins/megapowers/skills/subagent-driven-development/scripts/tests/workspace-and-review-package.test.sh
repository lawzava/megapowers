#!/usr/bin/env bash
# Execution coverage for the two SDD helpers that ship without any: sdd-workspace
# (the single source of truth for the artifact directory) and review-package (the
# reviewer's one-call diff bundle). Both need a real repository, so each case runs
# against a throwaway one. No network, no credentials.
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
WORKSPACE="$HERE/sdd-workspace"
PACKAGE="$HERE/review-package"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
passed=0
failed=0

ok()   { echo "ok   $1"; passed=$((passed + 1)); }
nope() { echo "FAIL $1"; shift; [ $# -gt 0 ] && printf '     %s\n' "$@"; failed=$((failed + 1)); }

check() {  # check NAME EXPECTED ACTUAL
  if [ "$2" = "$3" ]; then ok "$1"; else nope "$1" "want: $2" "got:  $3"; fi
}
contains() {  # contains NAME HAYSTACK-FILE NEEDLE
  if grep -qF -- "$3" "$2"; then ok "$1"; else nope "$1" "missing: $3"; fi
}

# A throwaway repo with a three-commit history. Signing is off and identity is
# local so the test never depends on the caller's git config or GPG agent.
make_repo() {
  local d="$scratch/$1"
  mkdir -p "$d"
  git -C "$d" init -q
  git -C "$d" config user.email test@example.invalid
  git -C "$d" config user.name "SDD Test"
  git -C "$d" config commit.gpgsign false
  printf 'one\n' > "$d/a.txt"
  git -C "$d" add a.txt && git -C "$d" commit -qm "first: seed"
  printf 'two\n' >> "$d/a.txt"
  git -C "$d" add a.txt && git -C "$d" commit -qm "second: extend a"
  printf 'three\n' > "$d/b.txt"
  git -C "$d" add b.txt && git -C "$d" commit -qm "third: add b"
  printf '%s' "$d"
}

# --- sdd-workspace ----------------------------------------------------------
repo="$(make_repo ws)"

dir="$(cd "$repo" && "$WORKSPACE")"
check "workspace path is the repo's .megapowers/sdd" "$repo/.megapowers/sdd" "$dir"
[ -d "$dir" ] && ok "workspace directory is created" || nope "workspace directory is created"
check "workspace .gitignore ignores everything" "*" "$(cat "$dir/.gitignore")"

# The whole point of the self-ignoring .gitignore: artifacts must not show up as
# untracked noise, and no tracked file gets modified to achieve that.
printf 'implementer report\n' > "$dir/report.md"
check "workspace stays out of git status" "" "$(cd "$repo" && git status --porcelain)"

dir2="$(cd "$repo" && "$WORKSPACE")"
check "workspace is idempotent" "$dir" "$dir2"
[ -f "$dir/report.md" ] && ok "re-running preserves existing artifacts" \
  || nope "re-running preserves existing artifacts"

rc=0; ( cd "$scratch" && "$WORKSPACE" ) >/dev/null 2>&1 || rc=$?
[ "$rc" -ne 0 ] && ok "workspace fails outside a git repository" \
  || nope "workspace fails outside a git repository" "rc=$rc"

# --- review-package: argument handling --------------------------------------
repo="$(make_repo rp)"

for args in "" "HEAD" "a b c d"; do
  rc=0
  # shellcheck disable=SC2086  # deliberate word splitting: each case is an arg list
  ( cd "$repo" && "$PACKAGE" $args ) >/dev/null 2>&1 || rc=$?
  check "wrong arg count ($([ -z "$args" ] && echo none || echo "$args")) exits 2" 2 "$rc"
done

rc=0; ( cd "$repo" && "$PACKAGE" no-such-ref HEAD ) >/dev/null 2>&1 || rc=$?
check "unknown BASE exits 2" 2 "$rc"
rc=0; ( cd "$repo" && "$PACKAGE" HEAD~2 no-such-ref ) >/dev/null 2>&1 || rc=$?
check "unknown HEAD exits 2" 2 "$rc"

# --- review-package: explicit outfile ---------------------------------------
out="$scratch/pkg.diff"
summary="$(cd "$repo" && "$PACKAGE" HEAD~2 HEAD "$out")"

[ -f "$out" ] && ok "explicit OUTFILE is written" || nope "explicit OUTFILE is written"
contains "package has a Commits section"       "$out" "## Commits"
contains "package has a Files changed section" "$out" "## Files changed"
contains "package has a Diff section"          "$out" "## Diff"
contains "package names the range"             "$out" "# Review package: HEAD~2..HEAD"

# The recorded-BASE contract: a two-commit task keeps BOTH commits. HEAD~1 would
# silently drop the older one, which is the bug the BASE argument exists to avoid.
contains "multi-commit range keeps the older commit" "$out" "second: extend a"
contains "multi-commit range keeps the newer commit" "$out" "third: add b"
if grep -qF "first: seed" "$out"; then
  nope "range excludes commits before BASE" "first: seed leaked into HEAD~2..HEAD"
else
  ok "range excludes commits before BASE"
fi
contains "diff body carries the added file" "$out" "+three"

case "$summary" in
  *"2 commit(s)"*) ok "summary reports the commit count" ;;
  *) nope "summary reports the commit count" "got: $summary" ;;
esac
case "$summary" in
  "wrote $out:"*) ok "summary names the output file" ;;
  *) nope "summary names the output file" "got: $summary" ;;
esac

# --- review-package: default outfile lands in the workspace -----------------
base7="$(git -C "$repo" rev-parse --short HEAD~1)"
head7="$(git -C "$repo" rev-parse --short HEAD)"
( cd "$repo" && "$PACKAGE" HEAD~1 HEAD ) >/dev/null
default="$repo/.megapowers/sdd/review-${base7}..${head7}.diff"
[ -f "$default" ] && ok "default OUTFILE is range-named inside the sdd workspace" \
  || nope "default OUTFILE is range-named inside the sdd workspace" "expected: $default"
check "default package leaves git status clean" "" "$(cd "$repo" && git status --porcelain)"

echo
echo "passed: $passed, failed: $failed"
[ "$failed" -eq 0 ]
