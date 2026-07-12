#!/usr/bin/env bash
S="$ROOT/plugins/mega-orchestration/skills"
present() { if grep -qiE "$1" "$2" 2>/dev/null; then echo "OK  $3"; else echo "BAD $3"; fi; }
{
  bon="$S/best-of-n/SKILL.md"
  present 'information restriction' "$bon" "best-of-n: information restriction (workers blind to each other)"
  present 'oracle first'            "$bon" "best-of-n: oracle first"
  present 'blind judge'             "$bon" "best-of-n: blind judge"
  present 'never (average|merge|blend)|not (average|blend)|SELECTION, not consensus' "$bon" "best-of-n: never-average guard"

  cmv="$S/cross-model-verification/SKILL.md"
  present 'different vendor'         "$cmv" "cross-verify: different vendor"
  present 'not see the author|withhold|blind to prior|information restriction' "$cmv" "cross-verify: blind to author reasoning"
  present 'refute'                   "$cmv" "cross-verify: refute posture"
  present 'oracle'                   "$cmv" "cross-verify: prefer oracle"
  present 'coverage, not voting|any credible refutation' "$cmv" "cross-verify: panel for coverage not voting"
  # the majority-vote loophole must stay removed (a real defect must not be outvoted)
  if grep -qiE 'majority refute' "$cmv"; then echo "BAD cross-verify: majority-vote loophole reintroduced"; else echo "OK  cross-verify: no majority-vote loophole"; fi

  cnc="$S/council-adjudication/SKILL.md"
  present 'answer independently|blind to each other' "$cnc" "council: independent answers"
  present 'anonymiz'                 "$cnc" "council: anonymized ranking"
  present 'never averages|not average|synthesize from the best' "$cnc" "council: synthesize-from-best not average"

  # the cross-vendor discipline must be EXECUTABLE, not just prose: the shipped
  # routing config has to express a real second-vendor route the different-vendor
  # roles can fall back to (the audit's unsatisfiable-constraint gap). Provider
  # vendor data lives in models.toml at the plugin root; role/fallback data stays
  # in the skill's delegates.toml.
  mt="$ROOT/plugins/mega-orchestration/models.toml"
  dt="$S/multi-agent-delegation/delegates.toml"
  present 'vendor[[:space:]]*=[[:space:]]*"anthropic"' "$mt" "delegation: a second model vendor route is shipped (anthropic)"
  present '\[fallbacks\]'            "$dt" "delegation: per-role fallback chains exist in config"
  present 'verify[[:space:]]*=[[:space:]]*\[[^]]*claude' "$dt" "delegation: verify carries a cross-vendor fallback (codex -> claude)"
} > inv.out
cat inv.out
