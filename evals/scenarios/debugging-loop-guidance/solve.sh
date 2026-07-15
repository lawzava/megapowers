#!/usr/bin/env bash
set -euo pipefail

skill="$ROOT/plugins/megapowers/skills/systematic-debugging/SKILL.md"
flat="$(tr '\n' ' ' < "$skill")"

marker() {
  name="$1"
  pattern="$2"
  if printf '%s\n' "$flat" | grep -Eiq "$pattern"; then
    echo "OK $name"
  else
    echo "MISSING $name"
  fi
}

guarded_marker() {
  name="$1"
  pattern="$2"
  forbidden="$3"
  if printf '%s\n' "$flat" | grep -Eiq "$pattern" &&
     ! printf '%s\n' "$flat" | grep -Eiq "$forbidden"; then
    echo "OK $name"
  else
    echo "MISSING $name"
  fi
}

{
  marker performance-baseline '(pre-change|before.{0,40}chang).{0,80}(performance|latency|throughput).{0,80}baseline|baseline.{0,80}(performance|latency|throughput)'
  marker pre-hypothesis-loop 'before.{0,50}(forming|testing).{0,30}hypotheses.{0,100}red-capable|red-capable.{0,100}before.{0,50}(forming|testing).{0,30}hypotheses'
  marker manual-correlation 'automation.{0,80}(cannot|unavailable).{0,120}(ask|user).{0,120}(correlat|correlation key|request.{0,20}identifier|job.{0,20}identifier)'
  marker evidence-cost-order '(rank|order).{0,40}hypotheses.{0,100}evidence.{0,100}(cost|cheap|expensive)|hypotheses.{0,80}evidence.{0,80}(cost|cheap|expensive)'
  marker minimized-slow-oracle 'minimi[sz](e|ed|ing).{0,80}(slow|integration|system).{0,80}(oracle|reproducer|reproduction).{0,160}retain.{0,80}(slow|original|full).{0,80}(oracle|test|scenario).{0,80}ground truth'
  marker stable-public-seam 'stable.{0,30}(public|observable).{0,30}(seam|boundary)|public.{0,30}(seam|interface|behavior)'
  guarded_marker independent-expected-value \
    'derive.{0,100}expected (value|result|outcome).{0,100}independent|expected (value|result|outcome).{0,100}(derived|calculated|specified).{0,100}independent|independent.{0,100}expected (value|result|outcome)' \
    'expected (value|result|outcome).{0,80}(not|never|without).{0,80}independent|not.{0,50}(derive|calculate|specify).{0,80}expected (value|result|outcome).{0,80}independent'
  guarded_marker probe-cleanup \
    'tag.{0,80}temporary.{0,80}(probe|instrument).{0,180}(remove|clean|promot)|temporary.{0,80}(probe|instrument).{0,80}tag.{0,180}(remove|clean|promot)' \
    'tag.{0,80}temporary.{0,80}(probe|instrument).{0,180}(do not|never|without).{0,80}(remov|clean|promot)|temporary.{0,80}(probe|instrument).{0,80}tag.{0,180}(do not|never|without).{0,80}(remov|clean|promot)'
  marker deterministic-test-substitute 'TDD.{0,80}still appl(y|ies).{0,240}substitute evidence.{0,100}only.{0,160}explicit agreement.{0,100}human partner.{0,160}only.{0,160}no deterministic behavior.{0,160}(exercise|exercised).{0,80}locally.{0,200}(external|nondeterministic).{0,240}substitute oracle.{0,240}(conditions|correlation key).{0,160}pre-change.{0,160}post-change'
} > out.txt

cat out.txt
