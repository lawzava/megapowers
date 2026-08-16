#!/usr/bin/env bash
# Tests for the UserPromptSubmit skill router: when a prompt carries a trigger
# phrase a skill already declares in its frontmatter, the router names that one
# skill in hookSpecificOutput.additionalContext. Everything else is silence,
# because a hook that fires on every prompt trains the model to ignore it.
# Run: plugins/megapowers/hooks/tests/skill-router.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../skill-router.sh"
HOOKS_JSON="$HERE/../hooks.json"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

pass=0; fail=0
ok()  { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }

# Build a UserPromptSubmit payload and run the router on it.
run_router() {
  jq -nc --arg p "$1" '{session_id:"t",hook_event_name:"UserPromptSubmit",prompt:$p}' \
    | bash "$HOOK" 2>/dev/null
}
context_of() { printf '%s' "$1" | jq -r '.hookSpecificOutput.additionalContext // ""' 2>/dev/null; }
# Every skill the router can name, so a test can assert no OTHER skill leaked in.
skills_named() { printf '%s' "$1" | grep -oE 'megapowers:[a-z-]+' | sort -u; }

echo "== skill-router tests =="

# 1. names_the_matching_skill: a verbatim frontmatter trigger names exactly the
#    skill that declares it, in a valid UserPromptSubmit envelope.
out="$(run_router "why is this failing")"; rc=$?
[ "$rc" -eq 0 ] && ok || bad "exit 0 on a matching prompt (got $rc)"
printf '%s' "$out" | jq -e . >/dev/null 2>&1 && ok || bad "output is valid JSON"
ev="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.hookEventName' 2>/dev/null)"
[ "$ev" = "UserPromptSubmit" ] && ok || bad "hookEventName is UserPromptSubmit (got '$ev')"
ctx="$(context_of "$out")"
printf '%s' "$ctx" | grep -qF 'megapowers:systematic-debugging' && ok \
  || bad "additionalContext names megapowers:systematic-debugging (got '$ctx')"
named="$(skills_named "$out")"
[ "$named" = "megapowers:systematic-debugging" ] && ok \
  || bad "no other skill is named (got '$(printf '%s' "$named" | tr '\n' ' ')')"
[ "${#ctx}" -lt 200 ] && ok || bad "emitted text under 200 characters (got ${#ctx})"
[ "$(printf '%s' "$ctx" | grep -c '')" -le 1 ] && ok || bad "emitted text is one line"

# An explicit /skill-name is an invocation, not a phrase to weigh: the user
# typed the skill's own address. The 2026-08 audit found one silently no-op
# unless it was the prompt's first token. It outranks every derived row and
# the paste-shape gate, and it must stand alone after whitespace so a pasted
# path fragment stays silent.
for pair in \
  "/using-megapowers|megapowers:using-megapowers" \
  "Use teams,subagents, workflows, whatever you need. /using-megapowers|megapowers:using-megapowers" \
  "/brainstorming a notification system|megapowers:brainstorming" \
  "refactor the module then /test-driven-development|megapowers:test-driven-development" \
  "$(printf 'line one of a long brief\nline two with detail\nfollow /using-megapowers')|megapowers:using-megapowers"
do
  prompt="${pair%|*}"; want="${pair##*|}"
  got="$(skills_named "$(run_router "$prompt")")"
  [ "$got" = "$want" ] && ok || bad "explicit slash '$prompt' names $want (got '$got')"
done
for prompt in \
  "see /home/z/.claude/plugins/cache/megapowers/megapowers/0.12.0/skills/using-megapowers/SKILL.md" \
  "the file plugins/megapowers/skills/brainstorming/SKILL.md is stale"
do
  got="$(skills_named "$(run_router "$prompt")")"
  [ -z "$got" ] && ok || bad "path fragment '$prompt' stays silent (got '$got')"
done

# The other three confirmed misses from the transcript audit.
for pair in \
  "i don't know which queries cause that|megapowers:systematic-debugging" \
  "implement all 3 fixes|megapowers:test-driven-development" \
  "ok, all good? ready for pr?|megapowers:requesting-code-review" \
  "check if ci green|megapowers:verification-before-completion"
do
  prompt="${pair%|*}"; want="${pair##*|}"
  got="$(skills_named "$(run_router "$prompt")")"
  [ "$got" = "$want" ] && ok || bad "'$prompt' names $want (got '$got')"
done

# Every trigger phrase the frontmatter of a routed skill declares reaches that
# skill. This is the table's contract with the descriptions it was derived from:
# if a description changes, the row that quotes it must change with it.
while IFS='|' read -r prompt want; do
  [ -n "$prompt" ] || continue
  got="$(skills_named "$(run_router "$prompt")")"
  [ "$got" = "megapowers:$want" ] && ok || bad "'$prompt' names $want (got '${got:-nothing}')"
done <<'TRIGGERS'
add a feature for csv export|brainstorming
figure out the approach first|brainstorming
work is done|finishing-a-development-branch
ship this|finishing-a-development-branch
merge or PR?|finishing-a-development-branch
wrap up the branch|finishing-a-development-branch
humanize the release notes|humanizing-prose
this sounds like AI|humanizing-prose
cut the slop|humanizing-prose
make it read naturally|humanizing-prose
remember this for next time|project-memory
note this decision|project-memory
what did we decide about retries|project-memory
review this|requesting-code-review
ready to merge?|requesting-code-review
check my work|requesting-code-review
subagent per task please|subagent-driven-development
fan out the plan tasks|subagent-driven-development
multi-writer run|subagent-driven-development
find the cause|systematic-debugging
the test suite is failing|systematic-debugging
this test is flaky|systematic-debugging
intermittent failures in the pool|systematic-debugging
intermittent test failures|systematic-debugging
it fails intermittently|systematic-debugging
debug the timeout|systematic-debugging
TDD this|test-driven-development
test-first please|test-driven-development
write the test first|test-driven-development
red-green-refactor|test-driven-development
implement and test the parser|test-driven-development
upgrade megapowers|upgrading-megapowers
is there a newer megapowers release|upgrading-megapowers
write a plan|writing-plans
break this into steps|writing-plans
save the plan|writing-plans
do not implement yet|writing-plans
create a new skill for this|writing-skills
edit the brainstorming skill|writing-skills
execute the plan|executing-plans
are the tests green?|verification-before-completion
is it done?|verification-before-completion
did it pass|verification-before-completion
ci is green now|verification-before-completion
lint passes|verification-before-completion
confirm it works|verification-before-completion
debug this|systematic-debugging
diagnose the deadlock|systematic-debugging
why is the build failing|systematic-debugging
implement the plan|executing-plans
add an endpoint for health checks|test-driven-development
ready for pr|requesting-code-review
review the diff|requesting-code-review
TRIGGERS

# 2. stays_silent_without_a_trigger: silence is the default. A hook that fires on
#    ordinary work trains the model to ignore it, which is worse than no hook, so
#    the corpus below is ordinary prompts and every one of them must be silent.
#
#    This corpus is half the deliverable. An earlier table passed a suite that
#    never asked this question and measured 31 percent false positives on an
#    independent probe of ordinary engineering prompts. The first block is that
#    probe. The second block is near misses written to break the patterns: words
#    one letter away from a trigger, triggers used as nouns, and clauses that a
#    proximity gap would weld together.
while IFS= read -r quiet; do
  [ -n "$quiet" ] || continue
  out="$(run_router "$quiet")"; rc=$?
  [ "$rc" -eq 0 ] && ok || bad "exit 0 on '$quiet' (got $rc)"
  [ -z "$out" ] && ok || bad "silent on '$quiet' (got '$(context_of "$out")')"
done <<'QUIET'
set the log level to debug
add a debug flag to the CLI
remove the debug print from the handler
the debug output is too verbose
turn off debug logging in production
check that the caller passes a context
the lint config is clean
the build directory is clean
this is a clean checkout
why is the error message truncated
why does the help text say "flag" here
update the skill router hook comment
list the skills in the marketplace
which skill owns this procedure
document the skill frontmatter format
Tests: 3 failed, 27 passed, 30 total
does it work on windows
did it work before the refactor
confirm the port number in the config
run the plan through the linter
start the plan file with a header
add a feature flag to the config
clean up the branch names
rebase onto main
what does this regex match
inline this helper
move the parser into its own package
sort the imports
the error type should wrap the cause
add a field to the response struct
add an option to the config struct
delete the unused import
what is the p95 latency
show me the goroutine dump
why is the binary so large
grep for callers of this function
what does the -race flag do
who wrote this line
copy the fixture into testdata
change the timeout to 30s
the config is clean now
open a shell in the container
what ports does it bind
print the environment
compare these two error messages
where does the debug symbol table live
the wrong file was staged
switch to the other worktree
what does this diagnostic message mean
the debugger stops at the wrong line
diagnose.go needs a rename
add a diagnostic counter
the diagnose subcommand is undocumented
why is the test named that way
why does the build directory exist
the flaky-test dashboard is down
tests are documented in the wiki
check the config passes validation
verify the checksum matches
all lint rules are documented
the checks tab is empty
did the pipeline pass the artifact along
does this pass through a proxy
i reviewed this yesterday
the review comment is stale
merge or rebase, which do you prefer
ship date is friday
wrap up the branch names into a list
the plan file lives in docs
add a feature branch protection rule
add an endpoint list to the docs
tests with tests inside them
skillet is a bad variable name
the sloppy indentation bothers me
humanized output is not the goal
the planner assigns tasks
plan on it taking a while
remember to close the file
note this is a breaking change
what did we do about retries
fan out the requests to 4 workers
break this into two functions
save the plan file to disk
execute the planner binary
next task in the queue
green is the new color for the badge
is everything ready for the demo
is it working on safari
confirm it works on arm64
what is in this directory
read the config file and tell me what the timeout is
thanks, that looks right
show me the last 5 commits
what does this function do
rename the variable to userID
bump the version to 0.8.0
add a comment explaining the retry loop
list the open PRs
push it
run the tests
run the linter and paste the output
the deploy is stuck
format this file
what is the difference between the two branches
git status
explain how the dispatcher picks a target
translate this error message
who owns this file
make the button blue
show me the implementation of the retry loop
where is the test helper defined
how many tests are there
the CI config needs a new job
explain the difference between the two plans
what does the plan file contain
read the skill listing
which skills exist
open the PR description in the editor
what did the reviewer say
paste the diff
summarize the last review comment
the build takes 4 minutes
how long does the test suite take
this is a clean checkout
is the port already in use
add a new column to the table
add logging around the retry
document the flag in the README
update the changelog
bump the dependency
what version of go is this
list the failing files
tell me about intermittent connection pooling
QUIET

# A status claim is a trigger, not noise: verification-before-completion's
# frontmatter says it triggers on "any success or status claim", so an unverified
# pass claim naming it is the intended behavior, not a false positive.
named="$(skills_named "$(run_router "the tests pass locally")")"
[ "$named" = "megapowers:verification-before-completion" ] && ok \
  || bad "a status claim names verification-before-completion (got '${named:-nothing}')"

# 3. at_most_one_skill_per_prompt: two triggers, one named, and it is the one the
#    skills' own frontmatter puts first (test-driven-development says "Diagnose
#    unknown failures first"; requesting-code-review says "Verify behavior
#    first").
out="$(run_router "the test suite is failing, implement all 3 fixes")"
named="$(skills_named "$out")"
[ "$(printf '%s' "$named" | grep -c 'megapowers:')" -eq 1 ] && ok \
  || bad "exactly one skill named for two triggers (got '$(printf '%s' "$named" | tr '\n' ' ')')"
[ "$named" = "megapowers:systematic-debugging" ] && ok \
  || bad "diagnosis wins over TDD per TDD's own frontmatter (got '$named')"
named="$(skills_named "$(run_router "check if ci green, then ready for pr?")")"
[ "$named" = "megapowers:verification-before-completion" ] && ok \
  || bad "verification wins over review per review's own frontmatter (got '$named')"

# 3b. A pasted log is not an instruction. The corpus below is real tool output,
#     not text written to avoid the trigger words: an earlier version of this
#     test built its paste out of words the table never looked for, which proved
#     the cost and nothing about the silence. Real logs are dense in "passed",
#     "PASS", and "checks passed", which is exactly what the table reads.
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# A jest run: a failure summary, a diff, and a "27 passed" tally.
cat > "$TMP/jest.txt" <<'PASTE'
 PASS  src/parser/tokenize.test.ts
 PASS  src/parser/ast.test.ts
 FAIL  src/router/match.test.ts
  ● router › maps a trigger phrase to exactly one skill

    expect(received).toBe(expected)

    Expected: "systematic-debugging"
    Received: "test-driven-development"

      41 |     const out = route(sample);
    > 42 |     expect(out).toBe("systematic-debugging");
         |                 ^

Test Suites: 1 failed, 2 passed, 3 total
Tests:       3 failed, 27 passed, 30 total
Snapshots:   0 total
Time:        4.117 s
Ran all test suites.
PASTE

# A go test run: bare PASS lines and per package ok lines.
cat > "$TMP/gotest.txt" <<'PASTE'
=== RUN   TestRouteTable
=== RUN   TestRouteTable/silence_is_the_default
--- PASS: TestRouteTable/silence_is_the_default (0.00s)
=== RUN   TestRouteTable/one_skill_per_prompt
--- PASS: TestRouteTable/one_skill_per_prompt (0.00s)
--- PASS: TestRouteTable (0.00s)
PASS
ok      github.com/lawzava/megapowers/internal/router   0.012s
ok      github.com/lawzava/megapowers/internal/hooks    0.004s
ok      github.com/lawzava/megapowers/internal/catalog  (cached)
PASTE

# A GitHub Actions tail. The last line is word for word a status claim, so only
# its position past the typed instruction budget keeps the router quiet.
cat > "$TMP/gha.txt" <<'PASTE'
2026-07-30T09:14:02.1183921Z ##[group]Run actions/checkout@v4
2026-07-30T09:14:02.1184550Z with:
2026-07-30T09:14:02.1184901Z   fetch-depth: 0
2026-07-30T09:14:02.9931204Z ##[endgroup]
2026-07-30T09:14:03.4410233Z Syncing repository: lawzava/megapowers
2026-07-30T09:14:04.8820117Z ##[group]Run scripts/validate.sh
2026-07-30T09:14:05.0021884Z shellcheck: 41 files, no findings
2026-07-30T09:14:07.7712008Z plugin-validate: 9 plugins, 34 skills, 12 hooks
2026-07-30T09:14:09.1120744Z hook tests: 12 suites, 0 failures
2026-07-30T09:14:09.4410920Z ##[endgroup]
2026-07-30T09:14:10.0011203Z Post job cleanup.
2026-07-30T09:14:10.7781122Z Cleaning up orphan processes
2026-07-30T09:14:11.0010044Z all checks passed
PASTE

# The shortest paste that matters: three lines, 54 bytes, ending in a phrase
# that is word for word a completion claim. Length cannot separate this from a
# typed prompt (the longest single line in the quiet corpus is 52 bytes) and no
# pattern can either, because a user typing that last line alone SHOULD fire.
# Only the shape tells them apart.
cat > "$TMP/short.txt" <<'PASTE'
Run tests
  ok  github.com/x/y 0.4s
all checks passed
PASTE

# A question the user typed with a log pasted under it. This stays silent: they
# are asking about the log, not claiming their own work is done. A miss here
# costs one unfired suggestion, and firing costs context on every later turn.
printf 'why is the build failing on this branch\n' | cat - "$TMP/gha.txt" > "$TMP/q-log.txt"

for paste in jest gotest gha short q-log; do
  jq -n --rawfile p "$TMP/$paste.txt" '{prompt:$p}' > "$TMP/$paste.json"
  out="$(bash "$HOOK" < "$TMP/$paste.json" 2>/dev/null)"
  [ -z "$out" ] && ok || bad "silent on a pasted $paste log (got '$(context_of "$out")')"
done

# 3c. The shape gate, at its edges. The same claim that is silent inside the
#     three line paste above fires when the user types it alone, which is the
#     whole point: the phrase is not what changed, the shape is.
named="$(skills_named "$(run_router "all checks passed")")"
[ "$named" = "megapowers:verification-before-completion" ] && ok \
  || bad "a typed completion claim still fires on its own (got '${named:-nothing}')"
named="$(skills_named "$(run_router "ok, all good?
ready for pr?")")"
[ "$named" = "megapowers:requesting-code-review" ] && ok \
  || bad "two typed lines still route (got '${named:-nothing}')"
# Three lines is where every paste fixture starts and where no prompt in either
# corpus lives, so it is the gate.
[ -z "$(run_router "ok
all good?
ready for pr?")" ] && ok || bad "three lines is a paste, not a statement"
# Length is the backstop for the one paste a line count misses: a single
# enormous line. 600 is more than ten times the longest prompt in either corpus.
[ -n "$(run_router "check if ci green$(printf ' x%.0s' $(seq 280))")" ] && ok \
  || bad "a long but in budget single line still routes"
[ -z "$(run_router "check if ci green$(printf ' x%.0s' $(seq 300))")" ] && ok \
  || bad "a single line over the budget is a paste"

# Cost on a large paste: the router is synchronous on every prompt, so a wall of
# text cannot stall the turn. The shape gate now rejects this before the table
# sees it, which is why the number is flat. Keep the assertion anyway: it is
# what catches a future change that moves matching back ahead of the gate.
for i in $(seq 400); do
  printf 'ERROR why the suite runner spec %s assertion in check implementation of the ci build lint pipeline\n' "$i"
done > "$TMP/log.txt"
jq -n --rawfile p "$TMP/log.txt" '{prompt:$p}' > "$TMP/log.json"
start="$(date +%s%N)"
out="$(bash "$HOOK" < "$TMP/log.json" 2>/dev/null)"
big_ms=$(( ( $(date +%s%N) - start ) / 1000000 ))
printf '  cost: %sms on a %s byte pasted log\n' "$big_ms" "$(wc -c < "$TMP/log.txt" | tr -d ' ')"
[ -z "$out" ] && ok || bad "silent on a pasted log (got '$(context_of "$out")')"
[ "$big_ms" -lt 200 ] && ok || bad "a pasted log does not stall the turn (got ${big_ms}ms)"

# 4. No-op without jq, matching delegate-nudge.sh:10.
out="$(jq -nc '{prompt:"why is this failing"}' | PATH=/nonexistent "$BASH" "$HOOK" 2>/dev/null)"; rc=$?
[ "$rc" -eq 0 ] && ok || bad "exit 0 without jq on PATH (got $rc)"
[ -z "$out" ] && ok || bad "no output without jq on PATH (got '$out')"

# Malformed and empty payloads stay silent rather than emitting a broken envelope.
for junk in '' 'not json' '{}' '{"prompt":null}' '{"prompt":""}'; do
  out="$(printf '%s' "$junk" | bash "$HOOK" 2>/dev/null)"; rc=$?
  [ "$rc" -eq 0 ] && ok || bad "exit 0 on payload '$junk' (got $rc)"
  [ -z "$out" ] && ok || bad "no output on payload '$junk' (got '$out')"
done

# 5. Registration: UserPromptSubmit only, synchronous, through the dispatcher.
cmd="$(jq -r '.hooks.UserPromptSubmit[]?.hooks[]?.command // empty' "$HOOKS_JSON" 2>/dev/null)"
printf '%s' "$cmd" | grep -qF 'run-hook.cmd dispatch.sh skill-router.sh' && ok \
  || bad "hooks.json registers the router under UserPromptSubmit (got '$cmd')"
[ "$(jq -r '[.hooks.UserPromptSubmit[]?.hooks[]?] | length' "$HOOKS_JSON" 2>/dev/null)" = "1" ] && ok \
  || bad "exactly one UserPromptSubmit hook"
[ "$(jq -r '.hooks.UserPromptSubmit[0].hooks[0].async' "$HOOKS_JSON" 2>/dev/null)" = "false" ] && ok \
  || bad "UserPromptSubmit hook is async: false"
others="$(jq -r '[.hooks | to_entries[] | select(.key != "UserPromptSubmit") | .value[]?.hooks[]?.command] | map(select(test("skill-router"))) | length' "$HOOKS_JSON" 2>/dev/null)"
[ "$others" = "0" ] && ok || bad "router is registered for no other event (got $others)"

# 6. Cost: this runs synchronously on every prompt. The guard hook's accepted p95
#    is 121ms, so a single run must sit well inside that.
runs=20; start="$(date +%s%N)"
for _ in $(seq "$runs"); do run_router "why is this failing" >/dev/null; done
avg_ms=$(( ( $(date +%s%N) - start ) / runs / 1000000 ))
printf '  cost: %sms per run averaged over %s runs (payload build included)\n' "$avg_ms" "$runs"
[ "$avg_ms" -lt 121 ] && ok || bad "under the 121ms the guard establishes (got ${avg_ms}ms)"

# 7. Enforcement lifecycle. This rule is declared advisory in ../enforcement.toml
#    and reads two fields from there. `state` is the per-repository off switch,
#    so a repo can silence the router without patching a hook. `fire_prefix` is
#    the string scripts/session-metrics counts fires by, and keeping one copy is
#    the point: a router that changed its wording while the metric kept matching
#    the old one would report a rule nobody honors while it was in fact being
#    obeyed.
#
#    The project layer is read relative to the working directory, so these cases
#    run from a temp directory. Everything above ran from wherever the suite was
#    invoked, and that is restored afterwards so a later addition is unaffected.
_router_cwd="$PWD"
_router_tmp="$(mktemp -d "${TMPDIR:-/tmp}/skill-router-layer.XXXXXX")"
cd "$_router_tmp" || exit 1
mkdir -p .megapowers

# Baseline from this directory: no layer, shipped state advisory, router fires.
out="$(run_router "why is this failing")"
[ -n "$(context_of "$out")" ] && ok || bad "layer baseline: router fires with no project layer"

printf '[rules.skill-router]\nstate = "off"\n' > .megapowers/enforcement.toml
[ -z "$(run_router "why is this failing")" ] && ok || bad "project layer state=off silences the router"

printf '[rules.skill-router]\nstate = "advisory"\n' > .megapowers/enforcement.toml
[ -n "$(context_of "$(run_router "why is this failing")")" ] && ok \
  || bad "project layer state=advisory keeps the router speaking"

# A typo disables a rule rather than half-enabling it, which is the lifecycle
# contract every consumer has to honor. This hook once tested only the literal
# "off", so a misspelling kept it speaking while check-enforcement.sh would have
# rejected the same file. An independent review found the mismatch.
printf '[rules.skill-router]\nstate = "advisroy"\n' > .megapowers/enforcement.toml
[ -z "$(run_router "why is this failing")" ] && ok \
  || bad "an unknown state reads as off and silences the router"

# `enforced` is not this consumer's state either. A UserPromptSubmit hook cannot
# block, so the honest reading of a rule promoted past advisory is that this
# consumer no longer speaks for it.
printf '[rules.skill-router]\nstate = "enforced"\n' > .megapowers/enforcement.toml
[ -z "$(run_router "why is this failing")" ] && ok \
  || bad "a state this consumer does not implement silences the router"

# The prefix comes from the rules file, not from a literal in the hook. A layer
# that changes it must change what the hook emits, or the metric and the hook
# have already drifted.
printf '[rules.skill-router]\nstate = "advisory"\nfire_prefix = "ROUTED>> "\n' > .megapowers/enforcement.toml
ctx="$(context_of "$(run_router "why is this failing")")"
case "$ctx" in
  "ROUTED>> systematic-debugging."*) ok ;;
  *) bad "fire_prefix is read from the rules file (got '$ctx')" ;;
esac

# Fails OPEN, unlike the blocking gates. An advisory line cannot block anything,
# so an unreadable rules file falls back to the shipped wording and keeps
# routing: silence costs more here than a stale string would.
printf 'this is not toml at all {{{\n' > .megapowers/enforcement.toml
ctx="$(context_of "$(run_router "why is this failing")")"
case "$ctx" in
  "Trigger matched: megapowers:systematic-debugging."*) ok ;;
  *) bad "unparseable rules file falls back to the shipped prefix (got '$ctx')" ;;
esac

# A layer naming a different rule says nothing about this one. Per-key merging,
# not per-file replacement.
printf '[rules.some-other-rule]\nstate = "off"\n' > .megapowers/enforcement.toml
[ -n "$(context_of "$(run_router "why is this failing")")" ] && ok \
  || bad "a layer silencing another rule leaves the router speaking"

cd "$_router_cwd" || exit 1
rm -rf "$_router_tmp"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
