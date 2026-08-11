#!/usr/bin/env bash
# UserPromptSubmit router: when a prompt carries a trigger phrase a megapowers
# skill already declares, name that one skill at the moment the phrase is typed.
# Claude Code only. dispatch.sh is invoked with this file as its first argument
# and no second one, so a Codex session (which sets PLUGIN_ROOT) no-ops: Codex
# loads its charter from AGENTS.md and has no Skill tool to name.
#
# Why a hook and not the SessionStart reminder: the reminder is already in
# context for every turn and skills still did not fire. A transcript audit found
# one 13.5 hour session, 2009 messages, that invoked zero process skills against
# verbatim triggers in the user's own words, while honoring a hook enforced gate
# thirteen times in the same session. The variable is the enforcement mechanism,
# not the wording.
#
# Two rules keep it from becoming noise the model learns to skip:
#   1. Silence is the default. No trigger means no output and exit 0.
#   2. At most one skill per prompt. Competing suggestions are exactly what the
#      always present skills listing already does badly.
#
# The table below is derived, never invented: every pattern comes from a phrase
# the skill's own frontmatter `description` names, or from one of the four misses
# the audit confirmed. Order is first match wins, and the order itself is
# derived too: where one skill's frontmatter says another comes first
# ("Diagnose unknown failures first", "Verify behavior first", "Code review
# usually comes first", "Brainstorm unclear work first") the prerequisite sits
# earlier, and rows needing a specific noun sit above broader ones.
#
# Fails OPEN: no jq, no prompt, or a malformed payload means no output, exit 0.
set -u

command -v jq >/dev/null 2>&1 || exit 0

# jq reads stdin directly: one fork, not a cat plus a jq. An empty, non object,
# or unparseable payload yields an empty prompt, which exits silently below.
prompt="$(jq -r 'if type == "object" then (.prompt // "") else "" end' 2>/dev/null)"
[ -n "$prompt" ] || exit 0

# The router speaks only for a short typed statement. A paste is not an
# instruction: if a user pastes a log and says something, they are asking about
# the log, not making a claim about their own work.
#
# No pattern can do this job. "all checks passed" is word for word a completion
# claim, so a CI tail ending in it is indistinguishable from a user typing it,
# and narrowing the phrase would cost the real trigger. Shape is the only signal
# that separates them, and it is measured on the whole prompt, never on a
# truncated head: an earlier version truncated instead of rejecting, which let a
# 3KB log match on its first line.
#
# Both thresholds come from the corpora in tests/skill-router.test.sh, not from
# taste. Across the 58 prompts that must fire and the 133 that must stay silent,
# every one is a single line of 7 to 52 characters. Every paste fixture is 3
# lines or more. The two populations do not overlap on line count, and they do
# overlap on length (the shortest paste is 54 bytes, the longest typed prompt is
# 52), so lines are the instrument and length is only a backstop for the one
# paste shape a line count misses: a single enormous line.
#
#   at most 2 lines  -> one line of headroom above everything that must fire,
#                       one line below the smallest paste
#   at most 600 chars -> more than ten times the longest prompt in either corpus
#
# Rejecting rather than truncating also bounds the cost: the table never sees an
# input over 600 bytes. Keep every quantifier in it bounded anyway. Those bounds
# are the reach limit of a trigger phrase, and glibc's matcher tolerates an
# unbound one where BSD libc backtracks.
[ "${#prompt}" -le 600 ] || exit 0
newlines="${prompt//[!$'\n']/}"
[ "${#newlines}" -le 1 ] || exit 0

# Case insensitive matching without forking a tr or a grep: this runs
# synchronously on every prompt, so the whole match loop stays in the shell.
shopt -s nocasematch

# Each row is "<skill-name> <extended-regex>". Four rules keep the patterns cheap,
# portable, and quiet:
#   - No \b. It is a GNU extension that BSD libc treats as a literal b, which
#     would silently kill a row on macOS. Boundaries are character classes.
#   - Every gap quantifier carries an upper bound. An unbounded [^.?]* between two
#     alternations backtracks quadratically, and a trigger phrase spans one clause
#     anyway, so a bound costs nothing in reach.
#   - Gaps are [a-z ] or a counted word repeat, never [^.?]. [^.?] matches commas
#     and newlines, so it welds a noun in one clause to a verb in the next and
#     turns proximity into a false match.
#   - Both ends of a phrase are bound. A pattern with a bare right edge matches
#     inside longer words: skill inside skillet, slop inside sloppy, the plan
#     inside the planner.
routes=(
  # frontmatter: "creating new skills, editing existing skills, or verifying
  # skills work". Only creat and edit appear there, so only those are here: the
  # domain noun of this repository is "skill", and a row that also took writ,
  # updat, add, or new would fire on most sentences written about the repository
  # itself. Sits above the upgrade row because it needs the literal word skill.
  "writing-skills (creat|edit)[a-z]* ([a-z-]+ ){0,3}skills?([^a-z]|\$)"
  # frontmatter: "update, upgrade, refresh, or migrate Megapowers, check for a
  # newer release, or discover new Megapowers plugins"
  "upgrading-megapowers (updat|upgrad|refresh|migrat)[a-z]*( the)? megapowers([^a-z]|\$)|megapowers[a-z ]{0,40}(newer|latest) (release|version)([^a-z]|\$)|(new|newer) megapowers (release|version|plugin)"
  # frontmatter triggers: "remember this", "note this decision", "what did we decide"
  "project-memory remember (this|that|it)([^a-z]|\$)|note (this|that) decision([^a-z]|\$)|what did we (decide|agree)([^a-z]|\$)|remember for (later|next)([^a-z]|\$)|save (this|that) decision([^a-z]|\$)"
  # frontmatter triggers: "humanize", "sounds like AI", "slop", "read naturally"
  "humanizing-prose humaniz(e|es|ing)([^a-z]|\$)|sounds like ai([^a-z]|\$)|(^|[^a-z])slop([^a-z]|\$)|reads? (more )?natural"
  # frontmatter triggers: "subagent per task", "fan out plan tasks", "multi-writer"
  "subagent-driven-development subagent per task([^a-z]|\$)|fan out [a-z ]{0,20}(plan|task)s?([^a-z]|\$)|multi.writer([^a-z]|\$)"
  # frontmatter: "Use to design a feature, capability"; triggers "add a feature",
  # "figure out the approach". Above writing-plans, which says to brainstorm
  # first. "add a feature" only ends a clause or takes a preposition, so a
  # feature flag or a feature branch is not a request to design a feature.
  "brainstorming add a feature([^a-z]*\$| (for|to|that|which)([^a-z]|\$))|figure out (the|an) approach([^a-z]|\$)|design (a|the) (feature|capability)([^a-z]*\$| (for|to|that|which)([^a-z]|\$))"
  # frontmatter triggers: "write a plan", "break into steps", "save the plan",
  # "do not implement yet"
  "writing-plans writ[a-z]* (a|an|the|out a) plans?([^a-z]|\$)|break (this|it|that) (down )?into steps([^a-z]|\$)|save the plan[^a-z]*\$|do( not|n't) implement yet([^a-z]|\$)|need a plan([^a-z]|\$)"
  # frontmatter: "Use to execute a written implementation plan inline". Only
  # execut and implement trace there; start, begin, and run were invented, and
  # they fired on running a plan file through a tool rather than executing it.
  "executing-plans (execut|implement)[a-z]* (the|this|that|our) plan([^a-z]|\$)|next task in the plan([^a-z]|\$)"
  # frontmatter triggers: "why is this failing", "find the cause", "test suite is
  # failing", "intermittent failures", plus the confirmed miss "I don't know
  # which queries cause that". Above TDD, whose frontmatter says to diagnose
  # unknown failures first.
  #
  # debug and diagnose take an object. The frontmatter word is "diagnose", a
  # verb, and in engineering prose "debug" is far more often a noun or an
  # adjective: a debug flag, a debug print, the debug log level. error and wrong
  # are gone from the why row for the same reason, since "why is the error
  # message truncated" is a question about a string, not about a defect.
  "systematic-debugging why (is|does|did|are) [a-z ]{0,40}(fail|break|broke|crash)|find the (root )?cause([^a-z]|\$)|(tests?|suite|build|ci) (is|are|keeps?) (fail|break|flak)|flaky([^a-z-]|\$)|intermittent[a-z]{0,3} ([a-z]+ ){0,2}(fail|error|break|crash|timeout)|(fail|error|break|crash|timeout)[a-z]* ([a-z]+ ){0,2}intermittent|(don'?t|do not) know (what|which|why|where|how)[a-z' ]{0,60}(caus|fail|break|broke|wrong|slow)|(^|[^a-z])(debug|diagnose) (this|it|the|that|these|those|them|why)([^a-z]|\$)|unexpected (behavio|result|output)"
  # frontmatter: "before claiming work is complete, fixed, passing, ready to
  # merge", "Triggers on any success or status claim", plus the confirmed miss
  # "check if ci green". Above requesting-code-review, which says to verify first.
  #
  # The claim is a shape, not a proximity: the subject noun and the verdict are
  # adjacent, separated only by counted copulas. A gap of arbitrary characters
  # welded "check that the caller passes a context" into a pass claim. "clean" is
  # gone entirely: in this domain it is a hygiene adjective for a config, a
  # checkout, or a build directory far more often than a verdict. "work" is gone
  # from the did/does row, which was reading "does it work on windows" as a
  # status claim.
  # The bare "done?" row and the commit row both come from the 2026-08-11 prompt
  # corpus, 275 real typed prompts, where the shipped table fired on 1.5% of them.
  #
  # "done?" on its own was the single most repeated question in the corpus and the
  # existing row missed all of it: that row wants a subject ("is it done"), and
  # nobody types the subject. The prefix set is CLOSED rather than a character
  # gap, because "work is done" must keep routing to finishing-a-development-branch
  # from the row below, and any gap wide enough for "ok, " also swallows "work is ".
  #
  # Commit, push, and merge are here rather than on finishing-a-development-branch
  # because this skill's own frontmatter claims them ("before commits or pull
  # requests") and because the prerequisite runs first: the ask is to ship, and
  # what has to happen before shipping is the check. finishing still owns the
  # The ship rows want the verb in IMPERATIVE position: starting the prompt, or
  # after punctuation, or after a short acknowledgement. Everything else is a
  # question ABOUT shipping, and firing there costs context on every later turn
  # for a prompt that asked nothing of the branch. Three that used to match:
  # "is the merge to main automated?" ("the merge" is a noun), "Does CI test and
  # push to main?" and "Can the bot commit it automatically?".
  #
  # `and` and `also` are NOT boundaries. They join clauses inside a question as
  # readily as they lead an imperative, which is exactly how the two CI examples
  # above got in.
  #
  # NEITHER IS A COMMA. It separates items inside one clause far more often than
  # it ends a sentence, so "Can CI test, push to main, and deploy?" matched at
  # the comma while still being a question. Only a sentence terminator counts.
  # A bare `commit,` alternative was tried to recover one corpus prompt
  # ("Sounds good. Commit, merge to main and push") and withdrawn: it matches a
  # NOUN LIST just as readily ("Commit, merge, and push: are these automated?").
  # One unfired suggestion is the cheaper error, which is this table's stated
  # trade, and a rule that needs an exception per sentence shape is a rule that
  # has stopped being derived from the frontmatter.
  #
  # branch mechanics and sits below. Bare "commit" ends a clause; "commit the
  # message" and "commit the plan" take an object and stay silent. "push" alone
  # is NOT here: the quiet corpus carries "push it" beside "bump the version" and
  # "rename the variable", where it is one mechanical git call and not a ship.
  # Push earns a route only when it names main or leads a compound ("push and
  # tag"), which is the shape the corpus shows for actually shipping.
  "verification-before-completion (^|[^a-z])(all )?(ci|tests?|build|lint|suite|checks?) ((is|are|was|were|all|now|still|already) ){0,2}(green|passing|pass(es|ed)?)([^a-z]|\$)|is (it|this|that|everything) (done|working|fixed|passing|complete|ready)[^a-z]*\$|(did|does) (it|that|this) pass(es|ed)?[^a-z]*\$|confirm (it|this|that) (works|passes)[^a-z]*\$|^(ok|okay|so|and|well|right)?[,. ]{0,3}(all |everything )?(done|finished)[?. ]*\$|(^|[.!?] *|(ok|okay|then|now|please|sure)[,. ]+)commit( (and|it|this|that)([^a-z]|\$)|[,. ]*\$)|(^|[.!?] *|(ok|okay|then|now|please|sure)[,. ]+)(merge|push) (to|into) main([^a-z]|\$)|(^|[.!?] *|(ok|okay|then|now|please|sure)[,. ]+)push and([^a-z]|\$)"
  # frontmatter triggers: "review this", "ready to merge", "check my work", plus
  # the confirmed miss "ready for pr". Above finishing, which says code review
  # usually comes first.
  "requesting-code-review ready (to|for) (merge|pr|a pr|review)([^a-z]|\$)|review (this|my|the (diff|change|pr))([^a-z]|\$)|check my work([^a-z]|\$)"
  # frontmatter triggers: "work is done", "ship this", "merge or PR", "wrap up
  # the branch". The branch is the object of the sentence, not a modifier, so
  # "clean up the branch names" is not a request to finish a branch.
  "finishing-a-development-branch work is done([^a-z]|\$)|ship (this|it)([^a-z]|\$)|merge or pr([^a-z]|\$)|(wrap|clean) up (the|this) branch(es)?[^a-z]*\$"
  # frontmatter: "Use for any feature or bug fix"; triggers "TDD", "test-first",
  # "write the test first", "red-green-refactor", "implement and test", plus the
  # confirmed miss "implement all 3 fixes". Broadest row, so it sits last. The
  # add row keeps the nouns that name a feature and drops flag, field, and
  # option, which name a one line edit as often as a feature. Like the
  # brainstorming row it wants the noun as the object of the sentence, so a
  # feature flag and a command line argument stay silent.
  # The fix row is the implement row's twin and comes from the same corpus: this
  # skill's frontmatter says "any feature or bug fix", and a bug fix is asked for
  # with "fix", never with "implement". It takes the same closed object set, so
  # "fix the flaky test on windows" is a request to fix something specific and
  # "fix up the wording" is not a bug fix. systematic-debugging sits above and
  # still wins an unknown failure, which is the documented order.
  "test-driven-development (^|[^a-z])tdd([^a-z]|\$)|test.first([^a-z]|\$)|writ[a-z]* the tests? first([^a-z]|\$)|red.green.refactor([^a-z]|\$)|implement and test([^a-z]|\$)|implement (all|the|this|these|those|both|it|a|an|my|our|every|each|missing)([^a-z]|\$)|fix (all|it|this|that|these|those|them|both|the (bug|test|failure))([^a-z]|\$)|add (a|an|the|another) (feature|endpoint|command)([^a-z]*\$| (for|to|that|which|with)([^a-z]|\$))|with tests[^a-z]*\$"
)

skill=""
for route in "${routes[@]}"; do
  regex="${route#* }"
  if [[ $prompt =~ $regex ]]; then skill="${route%% *}"; break; fi
done
[ -n "$skill" ] || exit 0

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib-json.sh
. "$here/lib-json.sh"
# shellcheck source=lib-toml.sh
. "$here/lib-toml.sh"

# This rule is declared advisory in ../enforcement.toml, and two fields are read
# from there rather than hardcoded here.
#
# `state` lets a repository silence the router from a project layer instead of
# patching a hook, the same off switch every other rule gets. Layers read
# project, then user, then shipped, first hit wins.
#
# `fire_prefix` is the string scripts/session-metrics counts fires by. Keeping
# one copy is the whole point: a router that changed its wording and a metric
# that kept matching the old one would report a rule nobody honors while the
# rule was in fact firing and being obeyed.
#
# Fails OPEN, like the rest of this hook. An unreadable rules file falls back to
# the shipped wording and keeps routing: an advisory line cannot block anything,
# so silence costs more here than a stale string would.
rule_state=""
fire_prefix=""
for _layer in ".megapowers/enforcement.toml" \
              "${XDG_CONFIG_HOME:-$HOME/.config}/megapowers/enforcement.toml" \
              "$here/../enforcement.toml"; do
  [ -f "$_layer" ] || continue
  [ -n "$rule_state" ]  || rule_state="$(toml_scalar_in "$_layer" "rules.skill-router" state 2>/dev/null)"
  [ -n "$fire_prefix" ] || fire_prefix="$(toml_scalar_in "$_layer" "rules.skill-router" fire_prefix 2>/dev/null)"
  [ -n "$rule_state" ] && [ -n "$fire_prefix" ] && break
done
# Anything that is not this consumer's own state is off. The lifecycle contract
# says a typo disables a rule rather than half-enabling it, and this hook read
# only the literal "off", so `state = "advisroy"` kept the router speaking while
# check-enforcement.sh would have rejected the file. An empty value means no
# layer defined the key, which is the shipped default and stays on.
case "$rule_state" in
  ""|advisory) : ;;
  *) exit 0 ;;
esac
[ -n "$fire_prefix" ] || fire_prefix="Trigger matched: megapowers:"

msg="${fire_prefix}${skill}. Invoke it with the Skill tool before answering. Its procedure governs this turn."
printf '{\n  "hookSpecificOutput": {\n    "hookEventName": "UserPromptSubmit",\n    "additionalContext": "%s"\n  }\n}\n' "$(escape_for_json "$msg")"
exit 0
