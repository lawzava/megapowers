#!/usr/bin/env bash
set -euo pipefail

base="$ROOT/plugins/megapowers/skills/writing-skills"
guide="$(tr '\n' ' ' < "$base/authoring-best-practices.md" | tr -s '[:space:]' ' ')"
rubric="$(tr '\n' ' ' < "$base/de-prescription-rubric.md" | tr -s '[:space:]' ' ')"

has() {
  text="$1"
  pattern="$2"
  printf '%s\n' "$text" | grep -Eiq "$pattern"
}

deletion_rule_reversed() {
  text="$1"
  has "$text" '(do not|don.t|never|skip|avoid)( (run|use|apply))?.{0,40}guidance-unit deletion|guidance-unit deletion.{0,40}(must not|should not|need not) (run|use|apply)'
}

hard_rule_reversed() {
  text="$1"
  has "$text" 'hard dependenc.{0,100}(may|can|should)( be)? skip|hard dependenc.{0,100}(do(es)? not|must not|should not|need not) (block|require|need)|continue.{0,60}(without|before).{0,40}(required )?setup'
}

optional_rule_reversed() {
  text="$1"
  has "$text" 'optional enrichment.{0,100}(blocks|must block|should block|may block|can block).{0,80}(correct )?core workflow|(correct )?core workflow.{0,100}(blocks|waits for|requires).{0,80}optional enrichment'
}

leading_rule_reversed() {
  text="$1"
  has "$text" '(do not|don.t|never|avoid).{0,40}prefer.{0,100}(observable|concrete).{0,80}(lead|vocabulary|term)'
}

emit() {
  name="$1"
  if [ "$2" -eq 0 ]; then
    echo "OK $name"
  else
    echo "MISSING $name"
  fi
}

sentence_rc=1
if has "$guide" 'guidance-unit deletion (test|check)' &&
   has "$guide" 'instruction' && has "$guide" 'bullet' &&
   has "$guide" 'field' && has "$guide" 'fragment' &&
   has "$guide" 'remov(al|e|ing)' && has "$guide" 'permitted behavior' &&
   has "$guide" 'decision' && has "$guide" 'output' &&
   has "$guide" 'evidence' && has "$guide" 'likely mistake' &&
   has "$guide" 'no-op' &&
   has "$rubric" 'guidance-unit.{0,40}deletion (test|check)|guidance-unit.{0,80}no-op' &&
   has "$rubric" 'instruction' && has "$rubric" 'bullet' &&
   has "$rubric" 'field' && has "$rubric" 'fragment' &&
   has "$rubric" 'remov(al|e|ing)' && has "$rubric" 'permitted behavior' &&
   has "$rubric" 'decision' && has "$rubric" 'output' &&
   has "$rubric" 'evidence' && has "$rubric" 'likely mistake' &&
   ! deletion_rule_reversed "$guide" &&
   ! deletion_rule_reversed "$rubric"; then
  sentence_rc=0
fi

hard_rc=1
if has "$guide" 'hard dependenc.{0,120}required' &&
   has "$guide" 'hard dependenc.{0,80}must not be skipped' &&
   has "$guide" 'hard dependenc.{0,180}block.{0,80}(explicit )?setup' &&
   has "$rubric" 'hard dependenc.{0,120}required' &&
   has "$rubric" 'hard dependenc.{0,80}must not be skipped' &&
   has "$rubric" 'hard dependenc.{0,180}block.{0,80}(explicit )?setup' &&
   ! hard_rule_reversed "$guide" &&
   ! hard_rule_reversed "$rubric"; then
  hard_rc=0
fi

optional_rc=1
if has "$guide" 'optional enrichment.{0,80}(does not|must not|never) block.{0,80}(correct )?core workflow' &&
   has "$guide" 'optional enrichment.{0,260}(unavailable|missing).{0,80}(skip|fallback)' &&
   has "$rubric" 'optional enrichment.{0,80}(does not|must not|never) block.{0,80}(correct )?core workflow' &&
   has "$rubric" 'optional enrichment.{0,260}(unavailable|missing).{0,80}(skip|fallback)' &&
   ! optional_rule_reversed "$guide" &&
   ! optional_rule_reversed "$rubric"; then
  optional_rc=0
fi

leading_rc=1
if has "$guide" 'scan-heavy workflow guidance.{0,120}prefer.{0,100}(lead|first)' &&
   has "$guide" 'improve(s)? recognition' && has "$guide" 'observable' &&
   has "$guide" 'predicate' && has "$guide" 'action' &&
   has "$guide" 'artifact' && has "$guide" 'gate' && has "$guide" 'concrete concept' &&
   has "$guide" 'intensifier' && has "$guide" 'mental-state' &&
   has "$guide" 'defin(e|ing).{0,40}nonstandard term.{0,40}first use|nonstandard term.{0,40}defin(e|ing).{0,40}first use' &&
   has "$rubric" 'scan-heavy workflow guidance.{0,120}prefer.{0,100}(lead|first)' &&
   has "$rubric" 'improve(s)? recognition' && has "$rubric" 'observable' &&
   has "$rubric" 'predicate' &&
   has "$rubric" 'action' && has "$rubric" 'artifact' &&
   has "$rubric" 'gate' && has "$rubric" 'concrete concept' &&
   has "$rubric" 'intensifier' &&
   has "$rubric" 'mental-state' &&
   has "$rubric" 'defin(e|ing).{0,40}nonstandard term.{0,40}first use|nonstandard term.{0,40}defin(e|ing).{0,40}first use' &&
   ! has "$guide" '(start|lead) (each|every) workflow sentence' &&
   ! has "$rubric" '(start|lead) (each|every) workflow sentence' &&
   ! leading_rule_reversed "$guide" &&
   ! leading_rule_reversed "$rubric"; then
  leading_rc=0
fi

{
  emit guidance-unit-deletion-no-op "$sentence_rc"
  emit hard-dependency "$hard_rc"
  emit optional-enrichment-graceful-degradation "$optional_rc"
  emit observable-leading-vocabulary "$leading_rc"
} > out.txt

cat out.txt
