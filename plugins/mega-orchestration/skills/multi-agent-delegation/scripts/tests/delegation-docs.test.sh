#!/usr/bin/env bash
# Documentation contract for the multi-agent-delegation skill.
#
# The skill's prose makes claims about executable things: the launcher's exit
# codes, the shipped effort table, the licences its prompting guidance came
# under. Prose drifts silently because nothing runs it, and a caller that
# branches on a stale exit map or an author who trusts a stale licence line is
# wrong in a way no other test catches. These assertions run the prose against
# the artifacts it describes.
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../.." && pwd)
SKILL_DIR="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation"
SKILL="$SKILL_DIR/SKILL.md"
RECEIPTS="$SKILL_DIR/references/receipts-and-rounds.md"
CONTRACT="$SKILL_DIR/references/dispatch-contract.md"
CODEX="$SKILL_DIR/references/providers/codex.md"
DELEGATES="$SKILL_DIR/delegates.toml"
RUN="$SKILL_DIR/scripts/delegate-run"

pass=0
fail=0
ok()  { printf '  PASS %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  FAIL %s\n' "$1"; fail=$((fail + 1)); }

# Markdown hard-wraps at 80 columns, so a sentence to assert on is split across
# lines and a literal grep for it misses. Fold whitespace before matching, or
# every phrase check here silently depends on where the paragraph happened to
# wrap.
folded() { tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

echo "== delegation docs contract =="

# --- exit map ---------------------------------------------------------------
# Every status delegate-run can actually exit with must be named in the exit
# map, or a caller branching on the documented set silently mishandles the ones
# it never heard of. Comment lines are stripped from the launcher first: they
# discuss codes (124, 128) it explains but never returns.
exit_section="$(sed -n '/^## Exit map/,/^## The round ledger/p' "$RECEIPTS" |
  tr '\n' ' ' | tr -s '[:space:]' ' ')"
# The enumeration is the first paragraph alone. Checking the whole section would
# pass on a code that only appears in the prose under it, which is exactly the
# shape of the omission this guards: 11 discussed but never enumerated.
enumeration="$(sed -n '/^## Exit map/,/^## The round ledger/p' "$RECEIPTS" |
  sed -n '/contract to branch on/,/^$/p' | tr '\n' ' ' | tr -s '[:space:]' ' ')"
if [ -z "$enumeration" ]; then
  bad "receipts-and-rounds.md: no exit-map enumeration paragraph to check"
else
  codes="$(grep -v '^[[:space:]]*#' "$RUN" | grep -oE '\bexit [0-9]+' |
    awk '{print $2}' | sort -n | uniq)"
  # 3 and 4 are the resolver's, propagated verbatim by delegate-run's
  # `exit "$route_rc"`, so they reach a caller without appearing as a literal.
  codes="$(printf '%s\n3\n4\n' "$codes" | sort -n | uniq | grep -v '^$')"
  missing=""
  signals_missing=""
  while IFS= read -r code; do
    [ -n "$code" ] || continue
    if [ "$code" -ge 100 ]; then
      # 130 and 143 are signal deaths, not statuses to branch on, so they belong
      # in the round-accounting prose rather than the enumeration.
      grep -qE "(^|[^0-9])$code([^0-9]|\$)" <<< "$exit_section" ||
        signals_missing="$signals_missing $code"
      continue
    fi
    grep -qE "(^|[^0-9])$code([^0-9]|\$)" <<< "$enumeration" ||
      missing="$missing $code"
  done <<< "$codes"
  if [ -n "$missing" ] || [ -n "$signals_missing" ]; then
    bad "exit map does not document status:$missing$signals_missing"
  else
    ok "exit map documents every status delegate-run can return"
  fi
fi

# 10 and 11 mean opposite things to the round ledger. Enumerating 11 without
# that is worse than omitting it: a caller retries the one that already spent a
# round as if it were the one that spent nothing.
if grep -qF 'paid for' <<< "$exit_section" &&
   grep -qF 'before a round is reserved' <<< "$exit_section" &&
   grep -qF 'the round counts and is closed as failed' <<< "$exit_section"; then
  ok "exit map states the round accounting that separates 10 from 11"
else
  bad "exit map does not say 11 consumes a round where 10 does not"
fi

# --- content the split must not have dropped --------------------------------
# A [roles] default names a provider, and reading it as a destination is how a
# caller dispatches to itself and calls the result independent.
grep -qF 'route to that provider unless you are already it' <<< "$(folded "$CONTRACT")" &&
  ok "dispatch-contract.md keeps the per-caller reading rule for role defaults" ||
  bad "dispatch-contract.md lost the 'unless you are already it' reading rule"

# The only statement of what the enforcement hook does when a role cannot reach
# two vendors. Without it the fewer-than-two case reads as a dead end.
grep -qF 'Stop-hook nudge' "$SKILL" && grep -qF 'human sign-off' "$SKILL" &&
  ok "SKILL.md keeps what the Stop-hook nudge does on a single-vendor role" ||
  bad "SKILL.md lost the Stop-hook nudge behaviour on a single-vendor role"

# --- one-hop pointers -------------------------------------------------------
# visual_verify needs a model route AND a driver, and resolution fails without
# both. Two hops to the driver contract is one hop past a partial read.
link="$(grep -oE '\(\.\./\.\./agents/browser-delegate\.md\)' "$SKILL" | head -1)"
if [ -n "$link" ] && [ -f "$ROOT/plugins/mega-orchestration/agents/browser-delegate.md" ]; then
  ok "SKILL.md reaches the browser-delegate driver contract in one hop"
else
  bad "SKILL.md has no resolving one-hop link to agents/browser-delegate.md"
fi

# --- provenance -------------------------------------------------------------
# The Apache-2.0 grant is codex-plugin-cc's. The GPT-5.6 docs page is not that
# repository and carries no such grant, so an unscoped licence line tells a
# reader they may redistribute text they may not.
codex_folded="$(folded "$CODEX")"
n_apache="$(grep -oF 'Apache-2.0' <<< "$codex_folded" | wc -l)"
n_scoped="$(grep -oE 'codex-plugin-cc[^.]{0,40}Apache-2\.0' <<< "$codex_folded" | wc -l)"
if [ "$n_apache" -eq 0 ]; then
  bad "codex.md names no license for its adapted prompting material"
elif [ "$n_apache" -eq "$n_scoped" ]; then
  ok "codex.md scopes every Apache-2.0 mention to codex-plugin-cc"
else
  bad "codex.md mentions Apache-2.0 away from codex-plugin-cc ($n_apache mentions, $n_scoped scoped), so it reads as covering the docs page too"
fi

grep -qF 'prompt-guidance-gpt-5p6' "$CODEX" && grep -qiF 'all rights reserved' "$CODEX" &&
  ok "codex.md states the docs page's own terms" ||
  bad "codex.md cites the GPT-5.6 docs page without saying what terms it is used under"

# --- freshness --------------------------------------------------------------
# codex.md pins a model generation and carries dated audit numbers, so it rots
# on a clock. scripts/check-freshness.sh needs a parseable date to guard.
grep -qE '^Last reviewed: [0-9]{4}-[0-9]{2}-[0-9]{2}\.?$' "$CODEX" &&
  ok "codex.md carries a parseable 'Last reviewed:' date" ||
  bad "codex.md has no 'Last reviewed: YYYY-MM-DD' line for check-freshness.sh"

# --- the effort table -------------------------------------------------------
# codex.md summarises [role_efforts] rather than restating it, so the summary
# has to be recomputed against the table or it drifts the moment a row moves.
num_word() {
  case "$1" in
    0) echo zero ;; 1) echo one ;; 2) echo two ;; 3) echo three ;;
    4) echo four ;; 5) echo five ;; 6) echo six ;; 7) echo seven ;;
    8) echo eight ;; 9) echo nine ;; *) echo "$1" ;;
  esac
}
efforts="$(sed -n '/^\[role_efforts\]/,/^\[/p' "$DELEGATES" | grep -oE '"[a-z]+"' | tr -d '"')"
n_high="$(grep -cx 'high' <<< "$efforts")"
n_medium="$(grep -cx 'medium' <<< "$efforts")"
want="splits $(num_word "$n_high") high against $(num_word "$n_medium") medium"
if grep -qF "$want" <<< "$(folded "$CODEX")"; then
  ok "codex.md's effort split matches [role_efforts] ($want)"
else
  bad "codex.md does not say '$want'; [role_efforts] moved and the prose did not"
fi

echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
