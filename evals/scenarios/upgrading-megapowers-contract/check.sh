#!/usr/bin/env bash
set -u

s="$WORKDIR/SKILL.md"
r="$WORKDIR/channels.md"
pr="$WORKDIR/plugin-readme.md"
setup="$WORKDIR/setup.md"

[ -f "$s" ] || { echo "missing upgrading-megapowers skill"; exit 1; }
[ -f "$r" ] || { echo "missing channel reference"; exit 1; }

grep -q '^name: upgrading-megapowers$' "$s" || { echo "wrong skill name"; exit 1; }
grep -qiE '^description: Use when .*updat|^description: Use when .*upgrad|^description: Use when .*discover' "$s" || { echo "description is not trigger-only upgrade discovery"; exit 1; }

# The ban is on pinning THIS release, which would make the skill stale the next
# time one ships. A version named as history ("the pre-0.8.2 bare-string form")
# is the opposite: it dates a format change the reader has to recognize, and it
# stays true forever. Strip those SPANS rather than their whole lines: dropping the
# line would let a real hardcoded version hide behind a historical one beside it.
#
# ONLY `pre-X.Y.Z` is stripped, and that narrowness is the point. A first cut also
# accepted since/before/after/as of/from, which reads reasonable and is not: "install
# from v9.9.9 and keep that release pinned" is a real pin wearing an accepted prefix,
# and it sailed through. `pre-` is the one form that cannot introduce a pin, because
# it names the era BEFORE a version rather than the version to use. Widening this
# list means finding another prefix with that property, which is a stronger claim
# than it looks.
version_ban_hits() {  # stdin -> 0 when a pinned version survives the historical strip
  sed -E 's/pre-v?[0-9]+\.[0-9]+\.[0-9]+//g' |
    grep -Eq '(^|[^[:alnum:]_])v?[0-9]+\.[0-9]+\.[0-9]+([^[:alnum:]_]|$)'
}

# Self-test the filter before trusting it, because a too-permissive strip turns this
# whole check into a no-op that still reports PASS. An earlier cut accepted
# since/before/after/as of/from and let "install from v9.9.9 and keep that release
# pinned" through; these cases are that defect, pinned.
# One case per rejected prefix, so a mutant that re-accepts ANY single one of them
# fails here. Dropping a prefix from this list is how the matrix stops covering the
# defect it was written for: with `before` missing, a filter stripping `pre-|before`
# passed every remaining case while letting "pinned before v2.0.0" through.
for pin in \
  'Install Megapowers from v9.9.9 and keep that release pinned.' \
  'Requires version since 9.9.9.' \
  'pinned before v2.0.0' \
  'pinned after v2.0.0' \
  'as of v3.1.4 the flag moved' \
  'Use v9.9.9.' \
  'the pre-0.8.2 form changed. Also install v1.2.3 exactly.'; do
  printf '%s\n' "$pin" | version_ban_hits || { echo "version ban self-test: missed a pin ($pin)"; exit 1; }
done
printf 'still using the pre-0.8.2 bare-string form\n' | version_ban_hits &&
  { echo "version ban self-test: flagged a historical reference"; exit 1; }

if version_ban_hits < <(cat "$s" "$r"); then
  echo "hardcoded release version in reusable workflow"; exit 1
fi

# Prose assertions run against the file with newlines collapsed. These check
# that a rule is stated, not where the paragraph happens to break, and a
# line-anchored regex over wrapped prose fails on a reflow that changed no
# guidance at all. Structural checks (frontmatter, heading order, the fenced
# command strings) stay line-oriented below, because there the line IS the
# contract.
flat() { tr '\n' ' ' < "$1" | tr -s ' '; }
s_flat="$(flat "$s")"; r_flat="$(flat "$r")"
has() { printf '%s' "$1" | grep -qiE "$2"; }

line() { grep -niE "^## .*${1}" "$s" | head -1 | cut -d: -f1; }
inspect="$(line Inspect)"; classify="$(line Classify)"; propose="$(line Propose)"
apply="$(line Apply)"; verify="$(line Verify)"
case "$inspect:$classify:$propose:$apply:$verify" in
  *::*|:*:) echo "missing ordered phase"; exit 1 ;;
esac
[ "$inspect" -lt "$classify" ] && [ "$classify" -lt "$propose" ] && \
  [ "$propose" -lt "$apply" ] && [ "$apply" -lt "$verify" ] || {
    echo "workflow phases out of order"; exit 1;
  }

has "$s_flat" 'read.only|before (any|the first) write' || { echo "missing inspect-before-write rule"; exit 1; }
has "$s_flat" 'one .*approval|summari[sz]ed approval' || { echo "missing single approval plan"; exit 1; }
has "$s_flat" 'latest stable' || { echo "missing stable target default"; exit 1; }
has "$s_flat" 'preserve.*pin|pin.*preserve' || { echo "missing pin preservation"; exit 1; }
has "$s_flat" 'preserv(e|ed).*(scope|source).*(scope|source)' || { echo "missing source and scope preservation"; exit 1; }
has "$s_flat" 'relevant.*first' || { echo "missing relevant-first additions"; exit 1; }
has "$s_flat" 'show all' || { echo "missing full-catalog path"; exit 1; }
has "$s_flat" 'do not .*install|never .*install|explicit.*select' || { echo "optional additions can be implicit"; exit 1; }
has "$s_flat" 'overlapping any visible' || { echo "visible component overlap not excluded"; exit 1; }
has "$s_flat" 'do not install.*(both|double|simultaneous)|prevent.*double registration' || { echo "overlap migration can leave double registration"; exit 1; }
has "$s_flat" 'stop before .*addition' || { echo "partial failure does not stop additions"; exit 1; }
has "$s_flat" 'applied.*failed.*not attempted|applied, failed, and not attempted' || { echo "partial-state report incomplete"; exit 1; }
has "$s_flat" 're-read actual|re-inspect|inspect again' || { echo "observed-state reinspection absent"; exit 1; }
has "$s_flat" 'settings|hook' || { echo "settings and hook boundary absent"; exit 1; }

grep -Fq 'claude plugin list --available --json' "$r" || { echo "Claude discovery command absent"; exit 1; }
grep -Fq 'codex plugin list --available --json' "$r" || { echo "Codex discovery command absent"; exit 1; }
grep -Fq 'codex --version' "$r" || { echo "Codex CLI version probe absent"; exit 1; }
grep -Fq 'codex app-server daemon version' "$r" || { echo "Codex app-server version probe absent"; exit 1; }
grep -Fq 'npx skills update <approved-skill>... -p -y' "$r" || { echo "project skills update is not explicit"; exit 1; }
grep -Fq 'npx skills update <approved-skill>... -g -y' "$r" || { echo "global skills update is not explicit"; exit 1; }
has "$r_flat" 'symlink' || { echo "symlink channel absent"; exit 1; }
has "$r_flat" 'fork' || { echo "fork channel absent"; exit 1; }

# OpenCode. It has no marketplace, so an inspector that only knows the two
# marketplace channels reports "not installed" for a harness that is, and the
# skill walks past a whole install. The channel has to name the paths, and it has
# to carry the two facts that make this install break silently when it is wrong:
# a copied plugin resolves its sibling scripts to a directory that is not there,
# and the skills CLI lands OpenCode's skills in the SHARED agents directory that
# Claude Code also scans, which double-registers every skill on a machine that
# runs both.
has "$r_flat" 'opencode' || { echo "OpenCode channel absent"; exit 1; }
# The needles drop the leading tilde on purpose: a literal '~' inside quotes is
# exactly what the reference should contain, and shellcheck reads a quoted '~/...'
# in a script as a path that will not expand (SC2088).
grep -Fq '/.config/opencode/plugins' "$r" || { echo "OpenCode plugin path absent"; exit 1; }
has "$r_flat" 'symlink.*(rather than|instead of|never).*(copy|copie)|(copy|copie).*(break|not.*resolve)' ||
  { echo "OpenCode copy trap absent"; exit 1; }
grep -Fq '/.agents/skills' "$r" || { echo "shared skills-dir double registration absent"; exit 1; }
has "$r_flat" 'OPENCODE\.md' || { echo "OpenCode baseline absent from drift check"; exit 1; }
has "$r_flat" 'stop.*optional additions' || { echo "channel partial-failure stop absent"; exit 1; }

grep -q '`upgrading-megapowers`' "$pr" || { echo "plugin README does not list skill"; exit 1; }
grep -q '`upgrading-megapowers`' "$setup" || { echo "setup updating route absent"; exit 1; }
grep -qi 'clean floating branch' "$setup" || { echo "setup symlink fallback ignores pins"; exit 1; }

echo "ok: upgrading-megapowers contract is complete"
