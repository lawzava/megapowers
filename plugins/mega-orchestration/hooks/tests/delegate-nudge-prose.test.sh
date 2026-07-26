#!/usr/bin/env bash
# The gate reads changed logic, not documentation. Prose that names the risk
# categories (a review checklist, an instruction template) must not trip it.
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
printf '# Notes\n' > GUIDE.md
git add svc.go GUIDE.md
git commit -qm init
TR="$TMP/transcript.jsonl"
: > "$TR"

pass=0
fail=0
verdict() {
  local out
  out="$(printf '{"stop_hook_active":false,"transcript_path":"%s","permission_mode":"default"}' "$TR" \
    | bash "$HOOK" 2>/dev/null)"
  if printf '%s' "$out" | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
check() {
  local got
  got="$(verdict)"
  if [ "$1" = "$got" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$got" "$2"; fi
}

echo "== delegate-nudge prose tests =="

# A tracked doc that lists the risk categories as review triggers.
printf '# Notes\n\nGet an independent review for billing, oauth, and mutex changes.\n' > GUIDE.md
check ALLOW "tracked doc naming the categories allows"

# Same words in a new, untracked doc.
printf 'Route payment and authorization work to a second vendor.\n' > REVIEW.md
check ALLOW "untracked doc naming the categories allows"

# A real code change alongside the docs still blocks.
printf 'func handler() { billing() }\n' > svc.go
check BLOCK "code change beside the docs still blocks"

# Reverting the code leaves only the docs pending.
git checkout -q -- svc.go
check ALLOW "docs-only pending tree allows"

# An untracked source file is still scanned.
rm -f REVIEW.md
printf 'func pay() { /* oauth */ }\n' > pay.go
check BLOCK "untracked source file still blocks"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
