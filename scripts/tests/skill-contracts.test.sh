#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SKILLS="$ROOT/plugins/megapowers/skills"
EXPECTED="autonomous-run
code-quality
design-and-plan
evidence-research
humanizing-prose
independent-review
mcp-setup
orchestrating
safe-effects
systematic-debugging
test-first-implementation
upgrading-megapowers
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
if [ "$actual" = "$EXPECTED" ]; then ok 'inventory is exactly thirteen skills'; else bad 'inventory is exactly thirteen skills'; fi

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
RESEARCH="$SKILLS/evidence-research/SKILL.md"
PROSE="$SKILLS/humanizing-prose/SKILL.md"
REVIEW="$SKILLS/independent-review/SKILL.md"
UPGRADE="$SKILLS/upgrading-megapowers/SKILL.md"
UPGRADE_CHANNELS="$SKILLS/upgrading-megapowers/references/channels.md"

authority='Repository instructions, existing code, and configured project tools are authoritative; skills supply defaults only where the repository is silent\.'
contains 'root instructions carry repository authority' "$AGENT_RULES" "$authority"
contains 'code quality carries repository authority' "$QUALITY" "$authority"

contains 'orchestration prefers native capabilities' "$SKILLS/orchestrating/SKILL.md" 'native (harness )?(capabilities|agents|subagents|goals|planning|permissions|review)'
contains_document 'orchestration explicitly authorizes native delegation' "$SKILLS/orchestrating/SKILL.md" 'explicitly authorizes? native (agents?|subagents?)|native (agent|subagent) delegation is explicitly authorized'
contains_document 'orchestration routes independent read-heavy lanes' "$SKILLS/orchestrating/SKILL.md" '(two|2) or more (independent |disjoint )?(read-heavy )?(lanes|workstreams)|read-heavy (lanes|workstreams).*(independent|parallel)'
contains_document 'orchestration dispatches eligible lanes in parallel' "$SKILLS/orchestrating/SKILL.md" '(dispatch|run|delegate).*(lanes|workstreams).*(parallel|concurrent)|(parallel|concurrent).*(dispatch|run|delegate)'
contains 'orchestration discovers the personal capability registry' "$SKILLS/orchestrating/SKILL.md" '[~]/[.]config/megapowers/agent-capabilities[.]md'
contains_document 'orchestration chooses the fastest cheapest qualifying binding' "$SKILLS/orchestrating/SKILL.md" '(fastest|lowest latency).*(cheapest|lowest cost).*(qualif|meet)|(qualif|meet).*(fastest|lowest latency).*(cheapest|lowest cost)'
contains_document 'orchestration rejects unavailable registry bindings' "$SKILLS/orchestrating/SKILL.md" '(active|current) harness.*(available|callable)|(available|callable).*(active|current) harness'
contains_document 'orchestration ranks only verified registry bindings' "$SKILLS/orchestrating/SKILL.md" 'rankable.*(declared|known|verified).*(model|effort)|(model|effort).*(declared|known|verified).*rankable'
contains_document 'orchestration treats the registry as advisory' "$SKILLS/orchestrating/SKILL.md" '(registry|capability card).*(does not|cannot).*(authoriz|grant).*(disclos|external|write|side effect|permission)'
contains_document 'orchestration falls back from an unusable registry' "$SKILLS/orchestrating/SKILL.md" '(missing|stale|malformed|inaccessible).*(native default|ignore|do not use)|(native default|ignore|do not use).*(missing|stale|malformed|inaccessible)'
contains_document 'orchestration names three fan-out shapes' "$SKILLS/orchestrating/SKILL.md" 'disjoint slices.*same-brief candidates.*read-only review'
contains_document 'orchestration briefs contain prohibited scope and return condition' "$SKILLS/orchestrating/SKILL.md" 'prohibited scope.*return condition'
contains_document 'orchestration keeps raw payloads out of lead context' "$SKILLS/orchestrating/SKILL.md" 'raw payloads?.*(out of|outside).*(lead context|conversation)'
contains_document 'orchestration distinguishes ordinary handoffs from autonomous goals' "$SKILLS/orchestrating/SKILL.md" 'ordinary handoff.*verify-and-finish.*approved.*autonomous-run'
contains 'autonomy prefers native goals' "$SKILLS/autonomous-run/SKILL.md" 'native goals?'
contains 'autonomy persists durable checkpoints' "$SKILLS/autonomous-run/SKILL.md" 'durable (checkpoint|state)'
contains_document 'autonomy derives status from evidence' "$SKILLS/autonomous-run/SKILL.md" '(derive|reconstruct).*status.*(journal|checkpoint|evidence)'
contains_document 'autonomy requires a currently approved goal' "$SKILLS/autonomous-run/SKILL.md" 'currently approved.*(goal|objective).*(charter)'
contains_document 'autonomy does not inherit handoff authority' "$SKILLS/autonomous-run/SKILL.md" '(handoff|harness switch).*(does not|never).*(authoriz|authority)'
contains_document 'autonomy stops on workspace mismatch' "$SKILLS/autonomous-run/SKILL.md" '(workspace|worktree|branch|HEAD).*(mismatch|contradict).*(stop|do not execute|before acting)'
contains_document 'autonomy distinguishes paused and blocked' "$SKILLS/autonomous-run/SKILL.md" 'paused.*cap.*blocked.*external|blocked.*external.*paused.*cap'
contains_document 'autonomy checkpoints delegate ownership and effect authority' "$SKILLS/autonomous-run/SKILL.md" 'delegate ownership.*return.*artifacts?.*effect authority'

contains_document 'research fixes the question decision time boundary and stopping rule' "$RESEARCH" 'question.*decision.*time boundary.*stopping rule'
contains_document 'research starts from an artifact anchor' "$RESEARCH" '(code|artifact) anchor'
contains_document 'research uses proportionate authorized sources' "$RESEARCH" 'tickets.*docs.*chat.*observability.*errors.*analytics.*available.*authorized.*proportionate'
contains_document 'research prefers primary sources' "$RESEARCH" 'prefer primary sources'
contains_document 'research classifies load-bearing claims' "$RESEARCH" 'direct.*supported.*inferred.*speculative.*unknown.*contested'
contains_document 'research records sources and gaps' "$RESEARCH" 'sources consulted.*material gaps'
contains_document 'research protects sensitive transcripts' "$RESEARCH" 'sensitive transcripts.*raw chat.*out of'
contains_document 'research does not grant implementation or publication authority' "$RESEARCH" 'conclusion.*(does not|is not).*(authority|authorization).*(implement|publish)'

for artifact in plans 'task briefs' commits responses reviews PRs docs 'release notes' errors; do
  contains "prose covers $artifact" "$PROSE" "$artifact"
done
for invariant in identifiers numbers commands caveats uncertainty decisions outcome padding invent 'already-direct'; do
  contains "prose preserves $invariant invariant" "$PROSE" "$invariant"
done
contains_document 'prose requires accountable attribution' "$PROSE" 'named source.*direct observation.*explicit uncertainty'
contains_document 'prose makes evaluative claims concrete' "$PROSE" 'actor.*mechanism.*scope.*condition.*measurement'
contains_document 'prose calibrates unmeasured strength' "$PROSE" 'unmeasured intensifiers?.*(number|bounded scope|source)'
not_contains 'prose imposes no punctuation ban' 'ban (commas|colons|semicolons|dashes|punctuation)|never use (commas|colons|semicolons|dashes|punctuation)' "$PROSE" "$AGENT_RULES"

contains 'language references load lazily' "$QUALITY" 'load (exactly|only) one.*language reference'
for decision in maintenance review refactor architecture API concurrency debugging; do
  contains "language gate includes $decision" "$QUALITY" "$decision"
done
contains 'mechanical style stays with tools' "$QUALITY" 'formatter.*linter.*tests|formatter, linter, and tests'
contains_document 'quality reduces reader state load' "$QUALITY" 'reader.*(state|load)'
contains_document 'quality models repeated state branches' "$QUALITY" 'repeated state branches.*domain'
contains_document 'quality makes lifecycle operations idempotent' "$QUALITY" 'lifecycle operations?.*idempotent'
contains_document 'quality separates ownership before serialization' "$QUALITY" 'separate ownership before serialization'
contains_document 'quality promotes recurring failures into structural checks' "$QUALITY" 'failure recurs.*types.*tests.*lint.*canonical helper'

references="$(find "$SKILLS/code-quality/references" -maxdepth 1 -type f -printf '%f\n' | sort)"
if [ "$references" = "go.md
python.md
typescript.md" ]; then ok 'code quality has exactly three lazy references'; else bad 'code quality has exactly three lazy references'; fi

contains 'implementation requires red before code' "$SKILLS/test-first-implementation/SKILL.md" '(failing test|verify red).*(before|precedes).*(implementation|production code)|production code.*follows.*failing test'
contains 'implementation requires green evidence' "$SKILLS/test-first-implementation/SKILL.md" 'run.*focused test|verify green'
contains_document 'implementation permits a stronger direct oracle exception' "$SKILLS/test-first-implementation/SKILL.md" 'direct executable oracle.*stronger.*record.*exception.*pre-change failure'
contains 'debugging requires root cause first' "$SKILLS/systematic-debugging/SKILL.md" 'root cause.*before.*fix|before.*fix.*root cause'
contains 'debugging fixes through regression' "$SKILLS/systematic-debugging/SKILL.md" 'failing regression test|regression test.*before'
contains_document 'debugging retests restricted-environment failures outside the restriction' "$SKILLS/systematic-debugging/SKILL.md" 'sandbox.*re-?run.*outside.*before declaring|outside.*(sandbox|restriction).*before declaring'
contains 'effects require exact authorization' "$SKILLS/safe-effects/SKILL.md" 'authorization.*exact (target|effect)|exact (target|effect).*authorization'
contains_document 'effects require exact tracker-comment authorization' "$SKILLS/safe-effects/SKILL.md" '(public )?(tracker|issue|PR) comments?.*(exact|explicit) authorization|(exact|explicit) authorization.*(public )?(tracker|issue|PR) comments?'
contains_document 'implementation authority excludes outward writes' "$SKILLS/safe-effects/SKILL.md" '(implement|implementation|investigate|investigation|proceed).*(does not|do not|is not).*(authoriz|permission).*(comment|message|update|write)'
contains 'effects require duplicate prevention' "$SKILLS/safe-effects/SKILL.md" 'idempotenc|duplicate-prevention'
contains 'effects require target readback' "$SKILLS/safe-effects/SKILL.md" '(target|external).*readback|readback.*(target|external)'
contains_document 'effects reconcile retries and crashes' "$SKILLS/safe-effects/SKILL.md" 'retry.*crash.*reconcil'
contains_document 'effects use durable idempotency keys where repeats are possible' "$SKILLS/safe-effects/SKILL.md" 'durable idempotency key.*repeated external mutation'
contains_document 'effects stop re-refusing under direct supervision' "$SKILLS/safe-effects/SKILL.md" 'interactive supervision.*confirm.*once.*without re-refusing'
contains_document 'prose prohibits routine progress comments' "$PROSE" '(do not|never).*(publish|post|send).*(routine progress|progress narration).*(tracker|issue|PR)? ?comments?|comments?.*(do not|never).*(routine progress|progress narration)'
contains_document 'prose prohibits published test transcripts' "$PROSE" '(do not|never).*(publish|post|send|dump).*(test transcripts?)|test transcripts?.*(do not|never).*(publish|post|send|dump)'
contains 'completion uses fresh oracle evidence' "$SKILLS/verify-and-finish/SKILL.md" 'fresh.*oracle|oracle.*fresh'
contains 'completion separates external proof' "$SKILLS/verify-and-finish/SKILL.md" 'external (verification|oracle|proof)'
contains_document 'completion separates configuration from effective runtime' "$SKILLS/verify-and-finish/SKILL.md" 'configuration.*effective runtime'
contains_document 'completion binds stale-prone proof to artifact identity' "$SKILLS/verify-and-finish/SKILL.md" 'stale.*artifact identity.*commit SHA|artifact identity.*commit SHA.*stale'
contains_document 'completion requires real user journeys or agreed substitutes' "$SKILLS/verify-and-finish/SKILL.md" 'real user journey.*agreed substitute oracle'
contains_document 'completion proves the load-bearing safety fact proportionately' "$SKILLS/verify-and-finish/SKILL.md" 'load-bearing safety fact.*proof level'
contains_document 'completion confirms the named target branch' "$SKILLS/verify-and-finish/SKILL.md" 'checked-out branch.*named target.*(edit|commit)'

contains_document 'upgrade inspects provenance before writes' "$UPGRADE" '(inspect|inventory).*(version|scope|source|pin|local edits?).*(before|prior).*(write|change)|(before|prior).*(write|change).*(inspect|inventory).*(version|scope|source|pin|local edits?)'
contains_document 'upgrade preserves existing policy' "$UPGRADE" 'preserve.*(source|channel).*(scope).*(pin|local edits?)|(source|channel).*(scope).*(pin|local edits?).*preserve'
contains_document 'upgrade preserves enabled state' "$UPGRADE" 'preserve.*enabled state|enabled state.*preserve'
contains_document 'upgrade requires one exact approval' "$UPGRADE" '(one|single).*exact.*approval.*(target|source|scope).*(write|restart|verification)'
contains_document 'upgrade treats current state as a no-op' "$UPGRADE" '(already )?current.*(verified )?no-op|(verified )?no-op.*(already )?current'
contains_document 'upgrade keeps one channel per harness' "$UPGRADE" '(one|single) (installation )?(source|channel) per harness|(one|single) installation channel per harness'
contains_document 'upgrade binds marketplace head to stable release' "$UPGRADE" '(stable release|release tag).*(commit).*(marketplace|source|default branch).*(head).*(match|equal)|(marketplace|source|default branch).*(head).*(stable release|release tag).*(commit).*(match|equal)'
contains_document 'upgrade rechecks refreshed snapshot before registration' "$UPGRADE" '(after|following).*(refresh).*(before).*(registr|install).*(commit|head).*(match|equal)|(commit|head).*(after|following).*(refresh).*(before).*(registr|install).*(match|equal)'
contains 'upgrade links the current channel reference' "$UPGRADE" 'references/channels[.]md'
contains_document 'upgrade refreshes and reinstalls Claude' "$UPGRADE_CHANNELS" 'claude plugin marketplace update megapowers.*claude plugin update megapowers@megapowers'
contains_document 'upgrade refreshes and reinstalls Codex' "$UPGRADE_CHANNELS" 'codex plugin marketplace upgrade megapowers.*codex plugin add megapowers@megapowers'
contains_document 'upgrade reads marketplace sources separately' "$UPGRADE_CHANNELS" 'claude plugin marketplace list --json.*codex plugin marketplace list --json'
contains_document 'upgrade retains the Codex installed path' "$UPGRADE_CHANNELS" 'codex plugin add megapowers@megapowers --json.*installedPath'
contains_document 'upgrade ignores harness cache markers for parity' "$UPGRADE_CHANNELS" '[.]codex-marketplace-install[.]json.*[.]in_use.*(runtime|harness).*(exclude|ignore)|(exclude|ignore).*[.]codex-marketplace-install[.]json.*[.]in_use'
contains_document 'upgrade stops after a failed write' "$UPGRADE" '(write|command).*(fail|error).*(stop|do not continue).*(applied|not attempted)|(stop|do not continue).*(write|command).*(fail|error).*(applied|not attempted)'
contains_document 'upgrade verifies registration and cached bytes' "$UPGRADE" '(registration|plugin list).*(cached|cache).*(bytes|parity)|(cached|cache).*(bytes|parity).*(registration|plugin list)'
contains_document 'upgrade protects live stale caches' "$UPGRADE" '(do not|never).*(delete|remove).*(stale|superseded).*(cache).*(active|restart|session)|(active|restart|session).*(do not|never).*(delete|remove).*(stale|superseded).*(cache)'
contains_document 'upgrade snapshots caches registration may prune' "$UPGRADE" 'registration.*prune.*snapshot.*before registering.*restore'
contains_document 'upgrade does not invoke providers without approval' "$UPGRADE" '(do not|never).*(invoke|start|run).*(model|provider|session).*(without|unless).*(authoriz|approval)'

MCP="$SKILLS/mcp-setup/SKILL.md"
contains_document 'mcp setup requires restart before new tools' "$MCP" 'restart the session before expecting new tools|register at session start.*restart'
contains_document 'mcp setup names the headless auth limitation' "$MCP" '(headless|non-interactive) session cannot finish the grant|oauth.*only in an interactive session'
contains_document 'mcp setup verifies with a fresh redacted probe' "$MCP" 'fresh probe.*redact|verify.*fresh.*(probe|session).*redact'
contains_document 'mcp setup keeps one registration channel per server' "$MCP" 'one registration channel per server'
contains_document 'mcp setup retests sandboxed probe failures' "$MCP" 'sandbox.*(re-?run|retest).*outside.*before concluding|outside the sandbox before concluding'

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
contains_document 'review states artifact intent' "$REVIEW" 'state.*artifact intent'
contains_document 'review explains dismissed findings' "$REVIEW" 'explain.*dismissed findings?'

CATALOG="$SKILLS/catalog.json"
catalog_names="$(jq -r '.skills[].name' "$CATALOG" 2>/dev/null | sort)"
if [ "$catalog_names" = "$EXPECTED" ] && jq -e --argjson count 13 '
  .schema_version == "1" and
  (.skills | length) == $count and
  ([.skills[].name] | sort) == ([.skills[].name] | unique | sort) and
  ([.skills[].status] | all(. == "stable" or . == "experimental")) and
  ([.skills[] | select(.name == "evidence-research" and .status == "experimental")] | length) == 1
' "$CATALOG" >/dev/null 2>&1; then ok 'skill lifecycle catalog is complete and portable'; else bad 'skill lifecycle catalog is complete and portable'; fi

deleted='brainstorming|executing-plans|finishing-a-development-branch|project-memory|receiving-code-review|requesting-code-review|subagent-driven-development|test-driven-development|using-git-worktrees|using-megapowers|verification-before-completion|writing-plans|writing-skills|best-of-n|configuring-model-routes|council-adjudication|cross-model-verification|effect-broker|multi-agent-delegation|wayfinding|designing-frontends|golang-patterns|greenfield-(go|python|ts)-stack|python-patterns|scripting-in-go|typescript-patterns'
not_contains 'deleted skill names are absent' "$deleted" "$SKILLS" "$AGENT_RULES"

lineage='superpowers|obra/superpowers|jesse vincent|derived from|inspired by|^origin:'
not_contains 'agent-loaded guidance omits historical lineage' "$lineage" "$SKILLS" "$AGENT_RULES" "$ROOT/plugins/megapowers/hooks"

total_words="$(find "$SKILLS" -mindepth 2 -maxdepth 2 -type f -name SKILL.md -print0 | xargs -0 cat | wc -w)"
if [ "$total_words" -le 3900 ]; then ok 'primary skill guidance stays within 3900 words'; else bad 'primary skill guidance stays within 3900 words'; fi

while IFS= read -r skill; do
  words="$(wc -w < "$SKILLS/$skill/SKILL.md")"
  if [ "$words" -le 400 ]; then ok "$skill stays within 400 words"; else bad "$skill stays within 400 words"; fi
done <<EOF
$EXPECTED
EOF

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
