#!/usr/bin/env bash
# Linked-worktree test: in a linked worktree, `.git` is a FILE (a gitdir pointer),
# not a directory. The once-per-diff-state sentinel must be resolved through
# `git rev-parse --git-path` so it lands in the worktree's own git dir
# (.git/worktrees/<name>/...), never at a literal ".git/<file>" path that would
# either miss the real gitdir or corrupt the gitdir-pointer file itself.
# Run: plugins/mega-orchestration/hooks/tests/delegate-nudge-worktree.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
TMP="$(mktemp -d)"
cd "$TMP" || exit 1

git init -q main
cd main || exit 1
git config user.email t@t; git config user.name t; git config commit.gpgsign false
printf 'func handler() {}\n' > svc.go
git add svc.go
git commit -q -m init
git branch wt-branch >/dev/null 2>&1
git worktree add -q ../wt wt-branch

pass=0; fail=0

cd "$TMP/wt" || exit 1
if [ -f .git ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL sanity: worktree .git should be a file, not a directory"; fi

printf 'func handler() { billing() }\n' > svc.go   # risky, uncommitted
TR="$TMP/wt/transcript.jsonl"
: > "$TR"
input="$(printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TR")"

echo "== delegate-nudge linked-worktree tests =="

out1="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out1" | grep -q '"decision":"block"'; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL first stop on risky diff in worktree should nudge"; fi

sentinel="$(git rev-parse --git-path megapowers-delegate-nudge-seen)"
case "$sentinel" in
  */worktrees/*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL sentinel path not under a per-worktree gitdir: %s\n' "$sentinel" ;;
esac
if [ -f "$sentinel" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL sentinel file was not written"; fi

# The main worktree's .git must be untouched (still a real directory, not a file).
if [ -d "$TMP/main/.git" ] && [ ! -f "$TMP/main/.git/megapowers-delegate-nudge-seen" ]; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL main repo's .git dir was affected by the worktree's sentinel"
fi

# The linked worktree's own .git file must still be the plain gitdir pointer,
# not have been overwritten by the sentinel write.
if grep -q '^gitdir:' "$TMP/wt/.git" 2>/dev/null; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL worktree's .git pointer file was corrupted"
fi

out2="$(printf '%s' "$input" | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out2" | grep -q '"decision":"block"'; then fail=$((fail + 1)); echo "  FAIL second stop, SAME risky diff in worktree, should not re-nudge"; else pass=$((pass + 1)); fi

echo "== $pass passed, $fail failed =="
cd "$TMP" || true
rm -rf "$TMP"
[ "$fail" -eq 0 ]
