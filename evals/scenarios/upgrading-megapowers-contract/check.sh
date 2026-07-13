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

if grep -Eq '(^|[^[:alnum:]_])v?[0-9]+\.[0-9]+\.[0-9]+([^[:alnum:]_]|$)' "$s" "$r"; then
  echo "hardcoded release version in reusable workflow"; exit 1
fi

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

grep -qiE 'read.only|before (any|the first) write' "$s" || { echo "missing inspect-before-write rule"; exit 1; }
grep -qiE 'one .*approval|summari[sz]ed approval' "$s" || { echo "missing single approval plan"; exit 1; }
grep -qiE 'latest stable' "$s" || { echo "missing stable target default"; exit 1; }
grep -qiE 'preserve.*pin|pin.*preserve' "$s" || { echo "missing pin preservation"; exit 1; }
grep -qiE 'preserv(e|ed).*(scope|source).*(scope|source)' "$s" || { echo "missing source and scope preservation"; exit 1; }
grep -qiE 'relevant.*first' "$s" || { echo "missing relevant-first additions"; exit 1; }
grep -qi 'show all' "$s" || { echo "missing full-catalog path"; exit 1; }
grep -qiE 'do not .*install|never .*install|explicit.*select' "$s" || { echo "optional additions can be implicit"; exit 1; }
grep -qi 'overlapping any visible' "$s" || { echo "visible component overlap not excluded"; exit 1; }
grep -qiE 'do not install.*(both|double|simultaneous)|prevent.*double registration' "$s" || { echo "overlap migration can leave double registration"; exit 1; }
grep -qiE 'stop before .*addition' "$s" || { echo "partial failure does not stop additions"; exit 1; }
grep -qiE 'applied.*failed.*not attempted|applied, failed, and not attempted' "$s" || { echo "partial-state report incomplete"; exit 1; }
grep -qiE 're-read actual|re-inspect|inspect again' "$s" || { echo "observed-state reinspection absent"; exit 1; }
grep -qiE 'settings|hook' "$s" || { echo "settings and hook boundary absent"; exit 1; }

grep -Fq 'claude plugin list --available --json' "$r" || { echo "Claude discovery command absent"; exit 1; }
grep -Fq 'codex plugin list --available --json' "$r" || { echo "Codex discovery command absent"; exit 1; }
grep -Fq 'codex --version' "$r" || { echo "Codex CLI version probe absent"; exit 1; }
grep -Fq 'codex app-server daemon version' "$r" || { echo "Codex app-server version probe absent"; exit 1; }
grep -Fq 'npx skills update <approved-skill>... -p -y' "$r" || { echo "project skills update is not explicit"; exit 1; }
grep -Fq 'npx skills update <approved-skill>... -g -y' "$r" || { echo "global skills update is not explicit"; exit 1; }
grep -qi 'symlink' "$r" || { echo "symlink channel absent"; exit 1; }
grep -qi 'fork' "$r" || { echo "fork channel absent"; exit 1; }
grep -qiE 'stop.*optional additions' "$r" || { echo "channel partial-failure stop absent"; exit 1; }

grep -q '`upgrading-megapowers`' "$pr" || { echo "plugin README does not list skill"; exit 1; }
grep -q '`upgrading-megapowers`' "$setup" || { echo "setup updating route absent"; exit 1; }
grep -qi 'clean floating branch' "$setup" || { echo "setup symlink fallback ignores pins"; exit 1; }

echo "ok: upgrading-megapowers contract is complete"
