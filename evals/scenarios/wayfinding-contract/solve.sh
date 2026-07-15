#!/usr/bin/env bash
set -euo pipefail

skill_path="$ROOT/plugins/mega-orchestration/skills/wayfinding/SKILL.md"
sidecar_path="$ROOT/plugins/mega-orchestration/skills/wayfinding/agents/openai.yaml"
validator_path="$ROOT/scripts/validate-codex-skill-metadata"
orchestrating_path="$ROOT/plugins/mega-orchestration/skills/orchestrating/SKILL.md"
readme_path="$ROOT/plugins/mega-orchestration/README.md"
brainstorming_path="$ROOT/plugins/megapowers/skills/brainstorming/SKILL.md"
skill=''
sidecar=''
skill_exists=1
sidecar_exists=1

if [ -f "$skill_path" ]; then
  skill="$(tr '\n' ' ' < "$skill_path")"
  skill_exists=0
fi

if [ -f "$sidecar_path" ]; then
  sidecar="$(tr '\n' ' ' < "$sidecar_path")"
  sidecar_exists=0
fi

has() {
  text="$1"
  pattern="$2"
  printf '%s\n' "$text" | grep -Eiq "$pattern"
}

tracker_contract() {
  text="$1"
  has "$text" 'tracker.{0,80}(is[[:space:]]+)?optional|optional.{0,80}tracker' &&
    ! has "$text" 'tracker.{0,50}(is[[:space:]]+)?not[[:space:]]+optional|not[[:space:]]+optional.{0,50}tracker'
}

no_side_effect_contract() {
  text="$1"
  has "$text" 'never[[:space:]]+implement' &&
    has "$text" 'never.{0,80}execute[[:space:]]+a[[:space:]]+plan' &&
    has "$text" 'never.{0,120}start[[:space:]]+an[[:space:]]+autonomous[[:space:]]+run' &&
    has "$text" 'never.{0,140}automatically[[:space:]]+commit' &&
    ! has "$text" 'never[[:space:]]+(fail|refuse)[[:space:]]+to[[:space:]]+(implement|execute|commit)'
}

plan_ready_contract() {
  text="$1"
  has "$text" 'plan-ready.{0,120}(only|requires|when).{0,120}approved design|approved design.{0,120}(only|requires|when).{0,120}plan-ready' &&
    ! has "$text" 'plan-ready.{0,100}(no|without)[[:space:]]+(an[[:space:]]+)?approved design|no approved design.{0,100}plan-ready'
}

orchestrating_route() {
  text="$1"
  has "$text" 'long-horizon.{0,180}(unknown ownership|unresolved decisions|unclear sequencing)' &&
    has "$text" 'mega-orchestration:wayfinding'
}

readme_entry() {
  text="$1"
  has "$text" '`wayfinding`.{0,180}(unknown ownership|unresolved decisions|unclear sequencing)'
}

brainstorming_boundary() {
  text="$1"
  has "$text" '^description:.*use wayfinding.*(ownership|decisions|sequencing|uncertainty).*(spec|scoping)'
}

emit() {
  name="$1"
  if [ "$2" -eq 0 ]; then
    echo "OK $name"
  else
    echo "MISSING $name"
  fi
}

map_rc=1
if has "$skill" '\.megapowers/wayfinding/<[^>]+>/map\.md'; then
  map_rc=0
fi

decision_file_rc=1
if has "$skill" 'decisions/<[^>]+>\.md'; then
  decision_file_rc=0
fi

fog_frontier_rc=1
if has "$skill" 'fog|unknowns' && has "$skill" 'current frontier'; then
  fog_frontier_rc=0
fi

representation_rc=1
if has "$skill" 'source trust|trust.{0,60}source|source.{0,60}trust' &&
   has "$skill" 'owner' && has "$skill" 'decision' &&
   has "$skill" 'evidence' && has "$skill" 'dependenc'; then
  representation_rc=0
fi

loop_rc=1
if has "$skill" 'one decision at a time|one-decision-at-a-time' &&
   has "$skill" 'update.{0,80}(current )?frontier|frontier.{0,80}update'; then
  loop_rc=0
fi

tracker_rc=1
if tracker_contract "$skill" &&
   ! tracker_contract "$skill issue tracker is not optional"; then
  tracker_rc=0
fi

no_side_effect_rc=1
if no_side_effect_contract "$skill" &&
   ! no_side_effect_contract "$skill Never fail to implement or automatically commit." &&
   ! no_side_effect_contract "$(printf '%s\n' "$skill" | sed 's/, execute a plan//')" &&
   ! no_side_effect_contract "$(printf '%s\n' "$skill" | sed 's/, start an autonomous run//')"; then
  no_side_effect_rc=0
fi

stop_rc=1
if has "$skill" 'spec-ready|spec ready' && has "$skill" 'blocked'; then
  stop_rc=0
fi

plan_ready_rc=1
if plan_ready_contract "$skill" &&
   ! plan_ready_contract "$skill Plan-ready is valid only when no approved design exists."; then
  plan_ready_rc=0
fi

policy_rc=1
active_policy="$(awk '
  /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
  /^[^[:space:]]/ { section=$0; sub(/[[:space:]]*:.*$/, "", section); next }
  section == "policy" && /^[[:space:]]+allow_implicit_invocation[[:space:]]*:/ {
    value=$0
    sub(/^[^:]*:[[:space:]]*/, "", value)
    sub(/[[:space:]]+$/, "", value)
    print value
  }
' "$sidecar_path" 2>/dev/null)"
if [ "$active_policy" = false ]; then
  policy_rc=0
fi

prompt_rc=1
if has "$sidecar" 'default_prompt:' && printf '%s\n' "$sidecar" | grep -Fq "\$wayfinding"; then
  prompt_rc=0
fi

validator_exists=1
valid_sidecar_rc=1
drifted_prompt_rc=1
drifted_policy_rc=1
invalid_boolean_rc=1
quoted_boolean_rc=1
invalid_short_description_rc=1
missing_required_field_rc=1
official_optional_keys_rc=1
if [ -x "$validator_path" ]; then
  validator_exists=0
  fixture="$PWD/metadata-fixture"
  fixture_skill="$fixture/plugins/fixture/skills/wayfinding"
  mkdir -p "$fixture_skill/agents"
  cp "$skill_path" "$fixture_skill/SKILL.md"
  cp "$sidecar_path" "$fixture_skill/agents/openai.yaml"

  if "$validator_path" "$fixture" >/dev/null 2>&1; then
    valid_sidecar_rc=0
  fi

  sed 's/\$wayfinding/\$wrong-skill/' "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    drifted_prompt_rc=0
  fi

  awk '
    /^[[:space:]]*allow_implicit_invocation:[[:space:]]*false[[:space:]]*$/ {
      print "  allow_implicit_invocation: true"
      print "  # allow_implicit_invocation: false"
      next
    }
    { print }
  ' "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    drifted_policy_rc=0
  fi

  sed 's/allow_implicit_invocation: false/allow_implicit_invocation: sometimes/' \
    "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    invalid_boolean_rc=0
  fi

  sed 's/allow_implicit_invocation: false/allow_implicit_invocation: "false"/' \
    "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    quoted_boolean_rc=0
  fi

  sed 's/short_description: .*/short_description: "Too short"/' \
    "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    invalid_short_description_rc=0
  fi

  sed '/display_name:/d' "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if ! "$validator_path" "$fixture" >/dev/null 2>&1; then
    missing_required_field_rc=0
  fi

  awk '
    /^[[:space:]]*short_description:/ {
      print
      print "  icon_small: \"./assets/icon-small.svg\""
      print "  icon_large: \"./assets/icon-large.svg\""
      print "  brand_color: \"#336699\""
      next
    }
    /^policy:/ { print "dependencies: []" }
    { print }
  ' "$sidecar_path" > "$fixture_skill/agents/openai.yaml"
  if "$validator_path" "$fixture" >/dev/null 2>&1; then
    official_optional_keys_rc=0
  fi
fi

orchestrating_text="$(tr '\n' ' ' < "$orchestrating_path")"
readme_text="$(tr '\n' ' ' < "$readme_path")"
brainstorming_frontmatter="$(awk 'NR == 1 && /^---/ { fm=1; next } fm && /^---/ { exit } fm' "$brainstorming_path")"
orchestrating_route_rc=1
readme_entry_rc=1
brainstorming_boundary_rc=1
integration_mutations_rc=1
if orchestrating_route "$orchestrating_text"; then orchestrating_route_rc=0; fi
if readme_entry "$readme_text"; then readme_entry_rc=0; fi
if brainstorming_boundary "$brainstorming_frontmatter"; then brainstorming_boundary_rc=0; fi
if ! orchestrating_route "$(printf '%s\n' "$orchestrating_text" | sed 's/[^.]*mega-orchestration:wayfinding[^.]*\.//g')" &&
   ! readme_entry "$(printf '%s\n' "$readme_text" | sed 's/`wayfinding`[^.]*\.//g')" &&
   ! brainstorming_boundary "$(printf '%s\n' "$brainstorming_frontmatter" | sed '/wayfinding/d')"; then
  integration_mutations_rc=0
fi

{
  emit wayfinding-skill-exists "$skill_exists"
  emit local-map-contract "$map_rc"
  emit decision-file-contract "$decision_file_rc"
  emit fog-and-current-frontier "$fog_frontier_rc"
  emit source-owner-decision-evidence-dependency "$representation_rc"
  emit one-decision-loop "$loop_rc"
  emit tracker-optional "$tracker_rc"
  emit no-automatic-commit-or-execution "$no_side_effect_rc"
  emit stop-spec-ready-or-blocked "$stop_rc"
  emit plan-ready-requires-approved-design "$plan_ready_rc"
  emit codex-sidecar-exists "$sidecar_exists"
  emit implicit-invocation-disabled "$policy_rc"
  emit default-prompt-names-wayfinding "$prompt_rc"
  emit codex-metadata-validator-exists "$validator_exists"
  emit valid-sidecar-accepted "$valid_sidecar_rc"
  emit drifted-default-prompt-rejected "$drifted_prompt_rc"
  emit drifted-implicit-policy-rejected "$drifted_policy_rc"
  emit invalid-implicit-boolean-rejected "$invalid_boolean_rc"
  emit quoted-implicit-boolean-rejected "$quoted_boolean_rc"
  emit invalid-short-description-rejected "$invalid_short_description_rc"
  emit missing-required-interface-field-rejected "$missing_required_field_rc"
  emit official-optional-metadata-accepted "$official_optional_keys_rc"
  emit orchestrating-route-exists "$orchestrating_route_rc"
  emit readme-wayfinding-entry-exists "$readme_entry_rc"
  emit brainstorming-wayfinding-boundary-exists "$brainstorming_boundary_rc"
  emit integration-removal-mutations-rejected "$integration_mutations_rc"
} > out.txt

cat out.txt
