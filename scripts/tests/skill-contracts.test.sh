#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SKILLS="$ROOT/plugins/megapowers/skills"
EXPECTED="autonomous-run
code-quality
design-and-plan
humanizing-prose
independent-review
orchestrating
safe-effects
systematic-debugging
test-first-implementation
verify-and-finish"

pass=0
fail=0

ok() {
  pass=$((pass + 1))
  printf '  ok %s\n' "$1"
}

bad() {
  fail=$((fail + 1))
  printf '  FAIL %s\n' "$1"
}

contains() {
  local name="$1" file="$2" pattern="$3"
  if grep -Eqi -- "$pattern" "$file"; then ok "$name"; else bad "$name"; fi
}

contains_document() {
  local name="$1" file="$2" pattern="$3"
  if tr '\n' ' ' < "$file" | grep -Eqi -- "$pattern"; then ok "$name"; else bad "$name"; fi
}

not_contains() {
  local name="$1" pattern="$2"
  shift 2
  if grep -Erqi -- "$pattern" "$@"; then bad "$name"; else ok "$name"; fi
}

printf '== skill contracts ==\n'

actual="$(find "$SKILLS" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort)"
if [ "$actual" = "$EXPECTED" ]; then ok 'inventory is exactly ten skills'; else bad 'inventory is exactly ten skills'; fi

while IFS= read -r skill; do
  file="$SKILLS/$skill/SKILL.md"
  if [ ! -f "$file" ]; then
    bad "$skill has SKILL.md"
    continue
  fi

  name="$(sed -n '2s/^name: //p' "$file")"
  description="$(sed -n '3s/^description: //p' "$file")"
  fields="$(awk 'NR == 1 { next } /^---$/ { exit } /^[A-Za-z0-9_-]+:/ { sub(/:.*/, ""); print }' "$file")"
  if [ "$name" = "$skill" ]; then ok "$skill name matches directory"; else bad "$skill name matches directory"; fi
  if printf '%s' "$description" | grep -Eq '^Use when .+'; then ok "$skill description is task-triggering"; else bad "$skill description is task-triggering"; fi
  if [ "$fields" = "name
description" ]; then ok "$skill frontmatter is portable"; else bad "$skill frontmatter is portable"; fi
done <<EOF
$EXPECTED
EOF

AGENT_RULES="$ROOT/AGENTS.md"
QUALITY="$SKILLS/code-quality/SKILL.md"
PROSE="$SKILLS/humanizing-prose/SKILL.md"
REVIEW="$SKILLS/independent-review/SKILL.md"

authority='Repository instructions, existing code, and configured project tools are authoritative; skills supply defaults only where the repository is silent\.'
contains 'root instructions carry repository authority' "$AGENT_RULES" "$authority"
contains 'code quality carries repository authority' "$QUALITY" "$authority"

contains 'orchestration prefers native capabilities' "$SKILLS/orchestrating/SKILL.md" 'native (harness )?(capabilities|agents|subagents|goals|planning|permissions|review)'
contains_document 'orchestration explicitly authorizes native delegation' "$SKILLS/orchestrating/SKILL.md" 'explicitly authorizes? native (agents?|subagents?)|native (agent|subagent) delegation is explicitly authorized'
contains_document 'orchestration routes independent read-heavy lanes' "$SKILLS/orchestrating/SKILL.md" '(two|2) or more (independent |disjoint )?(read-heavy )?(lanes|workstreams)|read-heavy (lanes|workstreams).*(independent|parallel)'
contains_document 'orchestration dispatches eligible lanes in parallel' "$SKILLS/orchestrating/SKILL.md" '(dispatch|run|delegate).*(lanes|workstreams).*(parallel|concurrent)|(parallel|concurrent).*(dispatch|run|delegate)'
contains 'autonomy prefers native goals' "$SKILLS/autonomous-run/SKILL.md" 'native goals?'
contains 'autonomy persists durable checkpoints' "$SKILLS/autonomous-run/SKILL.md" 'durable (checkpoint|state)'
contains 'autonomy derives status from evidence' "$SKILLS/autonomous-run/SKILL.md" '(derive|reconstruct).*status.*(journal|checkpoint|evidence)'

for artifact in plans 'task briefs' commits responses reviews PRs docs 'release notes' errors; do
  contains "prose covers $artifact" "$PROSE" "$artifact"
done
for invariant in identifiers numbers commands caveats uncertainty decisions outcome padding invent 'already-direct'; do
  contains "prose preserves $invariant invariant" "$PROSE" "$invariant"
done
not_contains 'prose imposes no punctuation ban' 'ban (commas|colons|semicolons|dashes|punctuation)|never use (commas|colons|semicolons|dashes|punctuation)' "$PROSE" "$AGENT_RULES"

contains 'language references load lazily' "$QUALITY" 'load (exactly|only) one.*language reference'
for decision in maintenance review refactor architecture API concurrency debugging; do
  contains "language gate includes $decision" "$QUALITY" "$decision"
done
contains 'mechanical style stays with tools' "$QUALITY" 'formatter.*linter.*tests|formatter, linter, and tests'

references="$(find "$SKILLS/code-quality/references" -maxdepth 1 -type f -printf '%f\n' | sort)"
if [ "$references" = "go.md
python.md
typescript.md" ]; then ok 'code quality has exactly three lazy references'; else bad 'code quality has exactly three lazy references'; fi

contains 'implementation requires red before code' "$SKILLS/test-first-implementation/SKILL.md" '(failing test|verify red).*(before|precedes).*(implementation|production code)|production code.*follows.*failing test'
contains 'implementation requires green evidence' "$SKILLS/test-first-implementation/SKILL.md" 'run.*focused test|verify green'
contains 'debugging requires root cause first' "$SKILLS/systematic-debugging/SKILL.md" 'root cause.*before.*fix|before.*fix.*root cause'
contains 'debugging fixes through regression' "$SKILLS/systematic-debugging/SKILL.md" 'failing regression test|regression test.*before'
contains 'effects require exact authorization' "$SKILLS/safe-effects/SKILL.md" 'authorization.*exact (target|effect)|exact (target|effect).*authorization'
contains_document 'effects require exact tracker-comment authorization' "$SKILLS/safe-effects/SKILL.md" '(public )?(tracker|issue|PR) comments?.*(exact|explicit) authorization|(exact|explicit) authorization.*(public )?(tracker|issue|PR) comments?'
contains_document 'implementation authority excludes outward writes' "$SKILLS/safe-effects/SKILL.md" '(implement|implementation|investigate|investigation|proceed).*(does not|do not|is not).*(authoriz|permission).*(comment|message|update|write)'
contains 'effects require duplicate prevention' "$SKILLS/safe-effects/SKILL.md" 'idempotenc|duplicate-prevention'
contains 'effects require target readback' "$SKILLS/safe-effects/SKILL.md" '(target|external).*readback|readback.*(target|external)'
contains_document 'prose prohibits routine progress comments' "$PROSE" '(do not|never).*(publish|post|send).*(routine progress|progress narration).*(tracker|issue|PR)? ?comments?|comments?.*(do not|never).*(routine progress|progress narration)'
contains_document 'prose prohibits published test transcripts' "$PROSE" '(do not|never).*(publish|post|send|dump).*(test transcripts?)|test transcripts?.*(do not|never).*(publish|post|send|dump)'
contains 'completion uses fresh oracle evidence' "$SKILLS/verify-and-finish/SKILL.md" 'fresh.*oracle|oracle.*fresh'
contains 'completion separates external proof' "$SKILLS/verify-and-finish/SKILL.md" 'external (verification|oracle|proof)'

contains 'review resolves the packaged tool beside the skill' "$REVIEW" 'scripts/megapowers-review\.go.*beside this.*SKILL\.md|beside this.*SKILL\.md.*scripts/megapowers-review\.go'
contains 'review exposes inspect mode' "$REVIEW" 'go run "\$review_tool" inspect'
contains 'review exposes explicit file mode' "$REVIEW" '--file'
contains 'review exposes immutable range mode' "$REVIEW" '--base.*--head'
contains 'review requires external approval' "$REVIEW" '--approve-external'
contains 'review exposes the bound approval token' "$REVIEW" 'approval_token'
contains 'review names file or binary substitution' "$REVIEW" 'file or provider-binary change'
contains 'review requires reinspection after substitution' "$REVIEW" 'requires a new inspection'
contains 'review requires different providers' "$REVIEW" '(author|provider).*differ|different-provider'
contains 'review labels receipts advisory' "$REVIEW" 'receipt.*advisory|advisory.*receipt'
contains 'review disables transcript retention by default' "$REVIEW" 'transcript.*(off|not retained).*default|not retain.*transcript.*default'

deleted='brainstorming|executing-plans|finishing-a-development-branch|project-memory|receiving-code-review|requesting-code-review|subagent-driven-development|test-driven-development|upgrading-megapowers|using-git-worktrees|using-megapowers|verification-before-completion|writing-plans|writing-skills|best-of-n|configuring-model-routes|council-adjudication|cross-model-verification|effect-broker|multi-agent-delegation|wayfinding|designing-frontends|golang-patterns|greenfield-(go|python|ts)-stack|python-patterns|scripting-in-go|typescript-patterns'
not_contains 'deleted skill names are absent' "$deleted" "$SKILLS" "$AGENT_RULES"

lineage='superpowers|obra/superpowers|jesse vincent|derived from|inspired by|^origin:'
not_contains 'agent-loaded guidance omits historical lineage' "$lineage" "$SKILLS" "$AGENT_RULES" "$ROOT/plugins/megapowers/hooks"

total_words="$(find "$SKILLS" -mindepth 2 -maxdepth 2 -type f -name SKILL.md -print0 | xargs -0 cat | wc -w)"
if [ "$total_words" -le 2500 ]; then ok 'primary skill guidance stays within 2500 words'; else bad 'primary skill guidance stays within 2500 words'; fi

while IFS= read -r skill; do
  words="$(wc -w < "$SKILLS/$skill/SKILL.md")"
  if [ "$words" -le 400 ]; then ok "$skill stays within 400 words"; else bad "$skill stays within 400 words"; fi
done <<EOF
$EXPECTED
EOF

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
