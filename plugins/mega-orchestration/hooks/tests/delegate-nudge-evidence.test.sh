#!/usr/bin/env bash
# A block must name WHAT MATCHED. The gate reports a category, and the pending
# change may have nothing to do with that category, so a reason that stops at
# "risky logic changed" leaves the receiving agent to guess which path is
# implicated. It guesses wrong: the 2026-08-11 transcript audit found three of
# twelve sampled fires misdiagnosed in session, one reported to the human as a
# credential exposure that did not exist.
#
# The evidence is one keyword and one location, because that is the entire
# trigger. These tests hold each scan leg to naming both.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP" || exit 1
git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'func handler() {}\n' > svc.go
git add svc.go
git commit -qm init
TR="$TMP/transcript.jsonl"
: > "$TR"

pass=0
fail=0
reason() {
  printf '{"stop_hook_active":false,"transcript_path":"%s","permission_mode":"default"}' "$TR" \
    | bash "$HOOK" 2>/dev/null | jq -r 'select(.decision == "block") | .reason' 2>/dev/null
}
# Both needles must appear in the same block reason. Substring, not shape: the
# wording around them is free to change, the two facts are not.
names() {
  local got want_where="$1" want_word="$2" label="$3"
  got="$(reason)"
  if [ -z "$got" ]; then
    fail=$((fail + 1)); printf '  FAIL no block at all :: %s\n' "$label"; return
  fi
  case "$got" in
    *"$want_where"*) : ;;
    *) fail=$((fail + 1)); printf '  FAIL reason never names location [%s] :: %s\n' "$want_where" "$label"; return ;;
  esac
  case "$got" in
    *"$want_word"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL reason never names keyword [%s] :: %s\n' "$want_word" "$label" ;;
  esac
}

echo "== delegate-nudge evidence tests =="

# Leg 1: the tracked diff scanner. The path comes from the diff header.
printf 'func handler() { authorize(u) }\n' > svc.go
names "svc.go" "authoriz" "tracked diff names path and keyword"

# The keyword reported is the one that MATCHED, not the first in the list. A
# reason naming the wrong keyword is worse than naming none: it sends the reader
# to a line that does not exist.
git checkout -q -- svc.go
printf 'func handler() { mu.Lock() /* mutex held */ }\n' > svc.go
names "svc.go" "mutex" "tracked diff names the keyword that actually matched"
git checkout -q -- svc.go

# Leg 2: an untracked file's CONTENT.
printf 'def charge():\n    return stripe.pay()\n' > pay.py
names "pay.py" "stripe" "untracked file content names path and keyword"
rm -f pay.py

# Leg 3: an untracked file's NAME. There is no file content to cite, so the
# location is the name itself and the keyword is what matched inside it.
#
# THE LOCATION AND THE KEYWORD MUST BE ASSERTED APART. An earlier version of
# this test passed "oauth" for both, so the keyword occurrence satisfied the
# location check too and the assertion held while the leg reported "-:1"
# instead of any path. gawk sets FILENAME to "-" for stdin, not to "", which is
# what the code was testing for. The filename here carries a risky keyword that
# the location string must contain IN FULL, and a distinct suffix that a bare
# keyword report cannot produce.
printf 'x = 1\n' > oauth_helper_svc.py
names "oauth_helper_svc.py" "oauth" "untracked path name reports the whole path"
got="$(reason)"
case "$got" in
  *"-:1"*) fail=$((fail + 1)); printf '  FAIL the name scan reported the stdin placeholder instead of the path\n' ;;
  *) pass=$((pass + 1)) ;;
esac
rm -f oauth_helper_svc.py

# A false positive has to be diagnosable, which is the whole point: the reason
# tells the reader to say so plainly rather than invent a matching risk.
printf 'func handler() { subscription() }\n' > svc.go
got="$(reason)"
case "$got" in
  *"false positive"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL reason never offers the false-positive reading\n' ;;
esac
git checkout -q -- svc.go

# NOT COVERED HERE, AND DELIBERATELY SO: comment markers are chosen by file
# extension, so an extensionless script (the shape of nearly every CLI) has its
# comments scanned as if they were code. That is the loudest false-positive
# class this gate has, and the evidence line above is what makes each instance
# diagnosable in one line rather than a guess.
#
# A shebang fallback for it was built and withdrawn on 2026-08-11 after four
# independent review rounds: it produced two Stop-hook hangs (an awk getline on
# a fifo, an unterminated `env` flag loop) and repeatedly classified content
# from a source the scanned diff did not describe, each time making the gate
# QUIETER than no fallback at all. Scanning a comment costs one explainable
# false fire; skipping a real line costs the thing the gate exists for. Anyone
# rebuilding it: classify from the blob named in the `index` header, key the
# table by that blob rather than by path, allowlist interpreters as tokens, and
# never open a path from a diff header in awk.

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
