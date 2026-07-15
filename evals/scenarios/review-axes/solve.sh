#!/usr/bin/env bash
set -euo pipefail

reviewer="$ROOT/plugins/megapowers/skills/requesting-code-review/code-reviewer.md"
requesting="$ROOT/plugins/megapowers/skills/requesting-code-review/SKILL.md"
receiving="$ROOT/plugins/megapowers/skills/receiving-code-review/SKILL.md"
sdd="$ROOT/plugins/megapowers/skills/subagent-driven-development/SKILL.md"
flat="$(tr '\n' ' ' < "$reviewer" | tr -s '[:space:]' ' ')"

axis_block() {
  heading="$1"
  awk -v heading="$heading" '
    $0 ~ "^[[:space:]]*##[[:space:]]+Output Format[[:space:]]*$" {
      format=1
      next
    }
    format && $0 ~ "^[[:space:]]*##[[:space:]]+Review Standards[[:space:]]*$" { exit }
    format && $0 ~ "^[[:space:]]*###[[:space:]]+" heading "[[:space:]]*$" && !seen {
      seen=1
      active=1
      next
    }
    active && $0 ~ "^[[:space:]]*###[[:space:]]+" { exit }
    active { print }
  ' "$reviewer"
}

final_block() {
  awk '
    $0 ~ "^[[:space:]]*##[[:space:]]+Output Format[[:space:]]*$" {
      format=1
      next
    }
    format && $0 ~ "^[[:space:]]*##[[:space:]]+Review Standards[[:space:]]*$" { exit }
    format && $0 ~ "^[[:space:]]*###[[:space:]]+Final Assessment[[:space:]]*$" {
      active=1
      next
    }
    active && $0 ~ "^[[:space:]]*###[[:space:]]+" { exit }
    active { print }
  ' "$reviewer"
}

marker() {
  name="$1"
  if [ "$2" -eq 0 ]; then
    echo "OK $name"
  else
    echo "MISSING $name"
  fi
}

has() {
  text="$1"
  pattern="$2"
  printf '%s\n' "$text" | grep -Eiq "$pattern"
}

unauthorized_rule_reversed() {
  has "$flat" 'unauthorized deviation.{0,100}explicit requirement.{0,50}(is|counts as).{0,20}(not|never).{0,40}(specification )?noncompliance|(do not|never|avoid).{0,80}(treat|classify|count|report).{0,100}unauthorized deviation.{0,100}(noncompliance|fail)'
}

separation_rule_reversed() {
  has "$flat" '(^|[.!?][[:space:]])(merge|average|re-?rank|rerank).{0,80}(findings|severities).{0,50}axes|(do not|never|avoid).{0,60}(keep|preserve|report).{0,80}(findings|severities).{0,50}(separate|local|inside their axis)'
}

headings_rc=1
if grep -Eq '^[[:space:]]*###[[:space:]]+Specification Compliance[[:space:]]*$' "$reviewer" &&
   grep -Eq '^[[:space:]]*###[[:space:]]+Engineering Standards[[:space:]]*$' "$reviewer"; then
  headings_rc=0
fi

spec="$(axis_block 'Specification Compliance')"
spec_rc=1
if has "$spec" '^[[:space:]]*####[[:space:]]+Strengths[[:space:]]*$' &&
   has "$spec" '^[[:space:]]*####[[:space:]]+Findings[[:space:]]*$' &&
   has "$spec" '^[[:space:]]*#####[[:space:]]+Critical \(Must Fix\)[[:space:]]*$' &&
   has "$spec" '^[[:space:]]*#####[[:space:]]+Important \(Should Fix\)[[:space:]]*$' &&
   has "$spec" '^[[:space:]]*#####[[:space:]]+Minor \(Nice to Have\)[[:space:]]*$' &&
   has "$spec" '^[[:space:]]*####[[:space:]]+Verdict[[:space:]]*$' &&
   has "$spec" '\*\*Specification Compliance:\*\* \[Pass \| Fail\]' &&
   has "$flat" 'unauthorized deviation.{0,100}explicit requirement.{0,50}is.{0,40}specification noncompliance' &&
   has "$flat" 'Specification Compliance verdict is Fail.{0,120}deviation an improvement' &&
   has "$flat" 'Specification Compliance.{0,30}Pass only when all requirements are met.{0,80}no unauthorized deviation remains' &&
   has "$flat" 'Specification.{0,100}severit.{0,80}requirement impact' &&
   ! unauthorized_rule_reversed; then
  spec_rc=0
fi

standards="$(axis_block 'Engineering Standards')"
standards_rc=1
if has "$standards" '^[[:space:]]*####[[:space:]]+Strengths[[:space:]]*$' &&
   has "$standards" '^[[:space:]]*####[[:space:]]+Findings[[:space:]]*$' &&
   has "$standards" '^[[:space:]]*#####[[:space:]]+Critical \(Must Fix\)[[:space:]]*$' &&
   has "$standards" '^[[:space:]]*#####[[:space:]]+Important \(Should Fix\)[[:space:]]*$' &&
   has "$standards" '^[[:space:]]*#####[[:space:]]+Minor \(Nice to Have\)[[:space:]]*$' &&
   has "$standards" '^[[:space:]]*####[[:space:]]+Verdict[[:space:]]*$' &&
   has "$standards" '\*\*Engineering Standards:\*\* \[Pass \| Fail\]' &&
   has "$flat" 'Engineering Standards.{0,30}Pass only when no Critical or Important engineering findings remain'; then
  standards_rc=0
fi

separation_rc=1
if has "$flat" 'clean engineering cannot compensate.{0,100}(missed|unauthorized) requirement' &&
   has "$flat" 'specification compliance cannot hide.{0,80}engineering defect' &&
   has "$flat" 'Do not merge, average, or (re-?rank|rerank) findings or severities across axes' &&
   has "$flat" 'one reviewer.{0,120}does not make.{0,80}independent of cross-axis anchoring' &&
   ! separation_rule_reversed; then
  separation_rc=0
fi

final="$(final_block)"
ready_rc=1
if has "$final" '\*\*Axis verdicts:\*\* Specification Compliance: \[Pass \| Fail\]; Engineering Standards: \[Pass \| Fail\]' &&
   has "$final" '\*\*Ready to merge\?\*\* \[Yes \| No \| With fixes\]' &&
   has "$flat" 'Ready to merge.{0,30}Yes.{0,20}only when both axes Pass' &&
   has "$flat" 'With fixes.{0,20}only when.{0,100}(failures|findings).{0,80}locally fixable.{0,120}no Critical.{0,100}human requirement decision' &&
   has "$flat" '(otherwise|all other cases).{0,30}(Ready to merge.{0,20})?No'; then
  ready_rc=0
fi

requesting_flat="$(tr '\n' ' ' < "$requesting" | tr -s '[:space:]' ' ')"
receiving_flat="$(tr '\n' ' ' < "$receiving" | tr -s '[:space:]' ' ')"
sdd_flat="$(tr '\n' ' ' < "$sdd" | tr -s '[:space:]' ' ')"
downstream_rc=1
if has "$requesting_flat" 'Specification Compliance.{0,20}Fail.{0,80}blocks proceeding.{0,80}regardless.{0,50}severity' &&
   has "$requesting_flat" '(correct|fix).{0,80}(explicit )?(authorization|approval).{0,80}re-review' &&
   has "$receiving_flat" 'Specification Compliance.{0,20}Fail.{0,80}blocks proceeding.{0,80}regardless.{0,50}severity' &&
   has "$receiving_flat" '(correct|fix).{0,80}(explicit )?(authorization|approval).{0,80}re-review' &&
   has "$sdd_flat" 'spec(ification)? (Compliance )?Fail.{0,100}(correction|correct|fix).{0,80}(authorization|approval)' &&
   has "$sdd_flat" 'spec(ification)? (Compliance )?Fail.{0,160}(never|cannot|must not).{0,60}Minor'; then
  downstream_rc=0
fi

{
  marker separate-axis-headings "$headings_rc"
  marker specification-axis-severities "$spec_rc"
  marker engineering-axis-severities "$standards_rc"
  marker findings-not-merged-or-reranked "$separation_rc"
  marker ready-to-merge-preserved "$ready_rc"
  marker specification-fail-blocks-downstream "$downstream_rc"
} > out.txt

cat out.txt
