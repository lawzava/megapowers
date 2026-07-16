#!/usr/bin/env bash
set -euo pipefail

skills="$ROOT/plugins/megapowers/skills"
plan="$(tr '\n' ' ' < "$skills/writing-plans/SKILL.md")"
debug="$(tr '\n' ' ' < "$skills/systematic-debugging/SKILL.md")"
memory="$(tr '\n' ' ' < "$skills/project-memory/SKILL.md")"

has() {
  text="$1"
  pattern="$2"
  printf '%s\n' "$text" | grep -Eiq "$pattern"
}

marker() {
  name="$1"
  text="$2"
  pattern="$3"
  if ! has "$text" "$pattern"; then
    echo "MISSING $name"
    return
  fi
  if [ "$name" = overlap-forces-sequential ] &&
     has "$text" 'overlap.{0,40}(allow|permit|safe)'; then
    echo "MISSING $name"
    return
  fi
  echo "OK $name"
}

source_roles() {
  text="$1"
  has "$text" 'repository instructions.{0,100}(process|workflow|procedure|govern)' &&
    ! has "$text" 'repository instructions.{0,100}(do not|does not|not).{0,60}(govern|define|control).{0,60}(process|workflow|procedure)' &&
    has "$text" 'CONTEXT\.md.{0,100}(vocabulary|domain context|current domain|domain vocabulary)' &&
    ! has "$text" 'CONTEXT\.md.{0,100}(is not|does not|do not|not).{0,80}(vocabulary|domain context|current domain|domain vocabulary)' &&
    has "$text" '(accepted|approved).{0,40}ADR.{0,100}(design|intent)|ADR.{0,100}(accepted|approved).{0,100}(design|intent)' &&
    ! has "$text" '(accepted|approved).{0,40}ADR.{0,100}(do not|does not|not).{0,60}(govern|define|record).{0,60}(design|intent)' &&
    has "$text" 'project memor.{0,120}(hidden|histor|hint).{0,120}(reverify|re-verify|verify)' &&
    ! has "$text" 'project memor.{0,120}(is not|are not|not).{0,80}(hidden|histor|hint)|project memor.{0,120}(should not|do not|does not).{0,40}(reverify|re-verify|verify)'
}

conflict_rule() {
  text="$1"
  has "$text" 'conflict' && has "$text" 'surface|report|name' &&
    has "$text" '(do not|never|not) silently'
}

context_rc=1
if has "$plan" 'repository instructions|AGENTS\.md' &&
   has "$plan" 'if present.{0,80}CONTEXT\.md|CONTEXT\.md.{0,80}(if|when) present' &&
   has "$plan" 'relevant.{0,40}ADR.{0,60}when present|when present.{0,60}relevant.{0,40}ADR' &&
   has "$plan" 'matching project memor.{0,60}when present|when present.{0,60}matching project memor' &&
   has "$debug" 'repository instructions|AGENTS\.md' &&
   has "$debug" 'if present.{0,80}CONTEXT\.md|CONTEXT\.md.{0,80}(if|when) present' &&
   has "$debug" 'relevant.{0,40}ADR.{0,60}when present|when present.{0,60}relevant.{0,40}ADR' &&
   has "$debug" 'matching project memor.{0,60}when present|when present.{0,60}matching project memor'; then
  context_rc=0
fi

roles_rc=1
if source_roles "$plan" && source_roles "$debug" &&
   has "$debug" 'observed (behavior|state|evidence).{0,100}(govern|authorit|source of truth)|actual (behavior|state|evidence).{0,100}(govern|authorit|diagnos)' &&
   ! has "$debug" 'actual behavior is not authoritative|observed behavior is not authoritative'; then
  roles_rc=0
fi

conflict_rc=1
if conflict_rule "$plan" && conflict_rule "$debug" && conflict_rule "$memory"; then
  conflict_rc=0
fi

diagnosis_rc=1
if has "$debug" 'diagnos(is|e).{0,100}(before|precedes).{0,100}(change )?plan|plan.{0,100}(only )?after.{0,100}diagnos' &&
   ! has "$debug" 'diagnosis does not precede planning|diagnosis does not come before planning'; then
  diagnosis_rc=0
fi

memory_rc=1
if has "$memory" 'CONTEXT\.md' && has "$memory" 'ADR' &&
   has "$memory" 'canonical' &&
   has "$memory" '(do not|never|don.t).{0,100}(duplicate|copy|save)|(duplicate|copy).{0,100}(do not|never)'; then
  memory_rc=0
fi

memory_recall_rc=1
if has "$memory" 'every recall.{0,120}(contradiction|conflict)|each recall.{0,120}(contradiction|conflict)' &&
   has "$memory" 'drift-prone.{0,40}actionable.{0,100}(independent|verify)|actionable.{0,40}drift-prone.{0,100}(independent|verify)|(independent|verify).{0,100}drift-prone.{0,40}actionable' &&
   ! has "$memory" 'reverify every recalled fact'; then
  memory_recall_rc=0
fi

{
  marker blocked-by "$plan" 'Blocked by'
  marker parallel-safety "$plan" 'Parallel safety'
  marker ownership "$plan" 'Ownership'
  marker may-decompose "$plan" 'May decompose'
  marker overlap-forces-sequential "$plan" 'overlap.{0,100}sequential|sequential.{0,100}overlap'
  marker blocker-owner "$plan" 'Owner:|owner.{0,80}(unresolved|blocker|input)'
  marker unblock-condition "$plan" 'Unblocks when:|unblock condition'
  marker expand-migrate-contract "$plan" 'expand.{0,180}migrate.{0,180}contract'
  marker context-and-adr-pass "$context_rc" '^0$'
  marker source-role-authority "$roles_rc" '^0$'
  marker conflict-surfaced "$conflict_rc" '^0$'
  marker diagnosis-before-plan "$diagnosis_rc" '^0$'
  marker memory-not-duplicate-canonical-docs "$memory_rc" '^0$'
  marker memory-recall-verification-scope "$memory_recall_rc" '^0$'
} > out.txt

cat out.txt
