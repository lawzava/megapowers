#!/usr/bin/env bash
# Pins the single delegation path: orchestrating routes to the two scripts, to
# no delegate subagent, and still names an executable route for the roles
# delegate-run cannot launch. The audit behind it counted 390 general-purpose
# subagent dispatches against 11 model-delegate ones while delegate-resolve and
# delegate-run were called 607 and 687 times, so the drift this guards against
# is guidance naming an agent nobody dispatches instead of the live commands.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
SKILL="$ROOT/plugins/mega-orchestration/skills/orchestrating/SKILL.md"
PRIMITIVES="$ROOT/plugins/mega-orchestration/skills/orchestrating/references/harness-primitives.md"
RETIRED="$ROOT/plugins/mega-orchestration/agents/model-delegate.md"
README="$ROOT/plugins/mega-orchestration/README.md"
DELEGATES="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml"
RUNNER="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/scripts/delegate-run"

fail() {
  printf 'delegation routing contract: %s\n' "$*" >&2
  exit 1
}

# Flattened so an assertion survives a rewrap of the paragraph it lives in.
flat() { awk '{$1=$1; printf "%s ", $0}' "$1"; }

must_have() {
  local body
  body=$(flat "$1")
  [[ $body == *"$2"* ]] || fail "missing contract text in ${1#"$ROOT/"}: $2"
}

must_not_have() {
  local body
  body=$(flat "$1")
  [[ $body != *"$2"* ]] || fail "forbidden text in ${1#"$ROOT/"}: $2"
}

# The routing skill is read on every non-trivial task, so its length is the
# budget that decides whether the delegation section is read at all. 140 buys
# the escape hatch one line for the non-review roles; anything more is a rewrite.
lines=$(wc -l < "$SKILL" | tr -d ' ')
(( lines <= 140 )) || fail "orchestrating SKILL.md exceeds 140 lines: $lines"

# Both halves of the interface, named as commands rather than as a role to look up.
must_have "$SKILL" 'scripts/delegate-resolve <role>'
must_have "$SKILL" 'scripts/delegate-run --role ROLE --author-vendor VENDOR --artifact worktree --claim TEXT'
must_have "$SKILL" 'delegate-run --role verify'
must_have "$SKILL" 'delegate-run --role code_review'

# One default and one escape hatch. Both sentences are load-bearing: without the
# first a session reaches for a subagent, without the second it has no route for
# the roles delegate-run does not launch.
must_have "$SKILL" 'reading the routing table is a script call, not a delegate subagent to dispatch'
must_have "$SKILL" 'Escape hatch: a route delegate-run cannot launch still resolves.'

# The escape hatch has to reach browser and visual work by name. A hatch worded
# only for `self` leaves the shapes the table assigns to delegation with no
# executable route at all, because visual and browser_test resolve to a CLI
# provider (asserted against delegates.toml below), not to `self`.
must_have "$SKILL" 'visual and browser_test are `cli` on a computer-use provider'
# ... and it has to reopen the skill that carries the channel and driver wording,
# which is the only place a browser dispatch is actually written down.
must_have "$SKILL" 'Load multi-agent-delegation for the provider and driver references'

# The decision root must not send a session looking for a delegate agent.
for agent in model-delegate browser-delegate codex-delegate; do
  must_not_have "$SKILL" "$agent"
done

# What makes the escape hatch necessary rather than decorative. delegate-run
# pins a read-only preset on every provider dispatch and demands the review
# verdict schema, so it runs review roles only; the config routes visual and
# browser_test to a named provider rather than to `self`. If either fact ever
# changes, the wording above is the thing to revisit.
presets=$(grep -c 'MEGAPOWERS_PRESET=read_only' "$RUNNER" || true)
(( presets >= 3 )) || fail "delegate-run no longer pins read_only on every dispatch: $presets sites"
grep -q 'review-verdict-v1' "$RUNNER" || fail 'delegate-run no longer demands the review verdict schema'
for role in visual browser_test; do
  grep -Eq "^${role}[[:space:]]+=" "$DELEGATES" || fail "delegates.toml has no [roles] entry for $role"
  if grep -Eq "^${role}[[:space:]]+=[[:space:]]+\"self\"" "$DELEGATES"; then
    fail "$role now resolves to self; rewrite the escape hatch"
  fi
done

# The retired wrapper may still ship until the lead removes it, but the harness
# loads it into the agent selector regardless, so its description is the only
# thing standing between a session and a dispatch into a no-op. It must read as
# a tombstone and must not advertise the live commands: naming delegate-run or
# delegate-resolve there is what makes the selector offer it for review work.
if [[ -f $RETIRED ]]; then
  desc=$(grep -m1 '^description:' "$RETIRED") || fail 'model-delegate.md has no description line'
  [[ $desc == 'description: "Retired.'* ]] || fail 'model-delegate description does not lead with Retired.'
  [[ $desc == *'Do not invoke this agent.'* ]] || fail 'model-delegate description does not forbid invocation'
  [[ $desc != *'delegate-run'* && $desc != *'delegate-resolve'* ]] ||
    fail 'model-delegate description advertises the live commands and attracts the selector'
fi

# The README is the plugin's shipped inventory, so it must not describe the
# retired agent as a working one.
must_not_have "$README" 'Two ship here'
must_not_have "$README" 'two delegate agents'
must_have "$README" '`agents/model-delegate.md` is retired and does nothing.'

# Anthropic's skill guidance: a reference over 100 lines opens with a table of
# contents, because the model otherwise reads it partially and at random.
plines=$(wc -l < "$PRIMITIVES" | tr -d ' ')
if (( plines > 100 )); then
  head -n 40 "$PRIMITIVES" | grep -q '^- \[Claude Code\](#claude-code)$' ||
    fail "harness-primitives.md is $plines lines and has no table of contents"
fi

# The reference must not reinstate model-delegate one level down. A wrapper agent
# that picks the external model is the retired agent under another name, and it
# contradicts the SKILL's one-path rule pinned above.
must_not_have "$PRIMITIVES" 'spawn a thin wrapper agent'
must_have "$PRIMITIVES" 'Putting that Bash call in a subagent is a context decision, never a routing one'

# codex-delegate was renamed in v0.4.x. Scanned over tracked files only: eval
# run artifacts and the docs/megapowers design notes are untracked history that
# records the old name on purpose, and so is the CHANGELOG entry announcing it.
# git grep, not `git ls-files | xargs grep ... || true`: that form swallowed a
# git failure, so a non-repo checkout printed ok without scanning anything.
scan_rc=0
stale=$(cd "$ROOT" && git grep -lF 'codex-delegate' -- ':!CHANGELOG.md' ':!scripts/tests') || scan_rc=$?
(( scan_rc <= 1 )) || fail "the codex-delegate scan did not run (git grep exit $scan_rc)"
[[ -z $stale ]] || fail "stale codex-delegate reference: $stale"

# Positive control for the scan above: an empty result only means clean if the
# same command can still find a string the tree certainly tracks.
(cd "$ROOT" && git grep -qlF 'delegate-resolve' -- ':!CHANGELOG.md' ':!scripts/tests') ||
  fail 'the codex-delegate scan matched nothing at all, so it proves nothing'

printf 'delegation routing contract: ok\n'
