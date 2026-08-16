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

# A no-commit task can have changes in all three working-tree states. The
# reviewer must see them alongside the committed range, or SDD can approve an
# empty/incomplete package before an authorized commit exists.
printf 'staged\n' >> "$repo/a.txt"
git -C "$repo" add a.txt
printf 'unstaged\n' >> "$repo/a.txt"
printf 'untracked\n' > "$repo/c.txt"
worktree_out="$scratch/worktree.diff"
( cd "$repo" && "$PACKAGE" HEAD~2 HEAD "$worktree_out" ) >/dev/null
contains "package has a staged section" "$worktree_out" "## Staged changes"
contains "package has an unstaged section" "$worktree_out" "## Unstaged changes"
contains "package has an untracked section" "$worktree_out" "## Untracked changes"
contains "package carries staged content" "$worktree_out" "+staged"
contains "package carries unstaged content" "$worktree_out" "+unstaged"
contains "package carries untracked content" "$worktree_out" "+untracked"

# An explicit package path inside the repository is itself untracked while the
# package is written. It must not recursively package its own partial output.
in_repo_out="$repo/review-package.diff"
( cd "$repo" && "$PACKAGE" HEAD~2 HEAD "$in_repo_out" ) >/dev/null
if grep -qF "diff --git a/review-package.diff b/review-package.diff" "$in_repo_out"; then
  nope "explicit in-repo package excludes itself" "package diff included its own OUTFILE"
else
  ok "explicit in-repo package excludes itself"
fi

# A distinct untracked filename can end in a newline. String normalization that
# strips that newline must not confuse it with the package output.
newline_file="$repo/review-package.diff"$'\n'
printf 'newline-path\n' > "$newline_file"
( cd "$repo" && "$PACKAGE" HEAD~2 HEAD "$in_repo_out" ) >/dev/null
contains "newline-named untracked file is included" "$in_repo_out" "+newline-path"

# Exit 1 means an untracked file differs from /dev/null. Exit 2 or higher means
# capture failed and must not be hidden behind a successful, incomplete package.
fatal_repo="$(make_repo rp-fatal)"
fatal_name=$'fatal-path\n'
printf 'must be reviewed\n' > "$fatal_repo/$fatal_name"
git_shim="$scratch/git-error-shim"
mkdir -p "$git_shim"
real_git="$(command -v git)"
cat > "$git_shim/git" <<'EOF'
#!/usr/bin/env bash
if [ "${1-}" = diff ] &&
   [ "${2-}" = --no-index ] &&
   [ "${3-}" = -- ] &&
   [ "${4-}" = /dev/null ] &&
   [ "${5-}" = $'fatal-path\n' ]; then
  printf 'injected fatal untracked diff\n' >&2
  exit 2
fi
exec "$REAL_GIT" "$@"
EOF
chmod +x "$git_shim/git"
fatal_out="$scratch/fatal.diff"
fatal_err="$scratch/fatal.err"
rc=0
(
  cd "$fatal_repo"
  REAL_GIT="$real_git" PATH="$git_shim:$PATH" \
    "$PACKAGE" HEAD~2 HEAD "$fatal_out"
) >/dev/null 2>"$fatal_err" || rc=$?
check "fatal untracked diff exits 2" 2 "$rc"
contains "fatal untracked diff is diagnosed" "$fatal_err" \
  "review-package: git diff failed for untracked path"
if [ ! -e "$fatal_out" ]; then
  ok "fatal capture does not publish a partial review package"
else
  nope "fatal capture does not publish a partial review package" "partial output exists: $fatal_out"
fi

# A non-regular untracked path (device node from a sandbox dotfile mask, socket)
# is listed by ls-files but git cannot hash it: "unsupported file type", exit
# 128. That is not a capture failure, it is an unreviewable path; it must be
# noted and skipped, not abort the package. Device nodes need root to create,
# so the shim injects git's real error for one path.
irregular_repo="$(make_repo rp-irregular)"
printf 'masked\n' > "$irregular_repo/.bash_profile"
printf 'real\n' > "$irregular_repo/real.txt"
cat > "$git_shim/git" <<'EOF'
#!/usr/bin/env bash
if [ "${1-}" = diff ] &&
   [ "${2-}" = --no-index ] &&
   [ "${3-}" = -- ] &&
   [ "${4-}" = /dev/null ] &&
   [ "${5-}" = .bash_profile ]; then
  printf 'error: .bash_profile: unsupported file type\nfatal: cannot hash .bash_profile\n' >&2
  exit 128
fi
exec "$REAL_GIT" "$@"
EOF
irregular_out="$scratch/irregular.diff"
rc=0
(
  cd "$irregular_repo"
  REAL_GIT="$real_git" PATH="$git_shim:$PATH" \
    "$PACKAGE" HEAD~2 HEAD "$irregular_out"
) >/dev/null 2>&1 || rc=$?
check "unsupported-file-type untracked path does not abort the package" 0 "$rc"
contains "package notes the skipped non-regular path" "$irregular_out" \
  "skipped non-regular untracked path"
contains "package still carries regular untracked content" "$irregular_out" "+real"

echo
echo "passed: $passed, failed: $failed"
[ "$failed" -eq 0 ]
