#!/usr/bin/env bash
# Exercises the Codex skill-metadata sidecar contract for wayfinding: the shipped
# sidecar keeps implicit invocation off and names the skill in its default prompt,
# and scripts/validate-codex-skill-metadata actually enforces the schema.
#
# Every validator marker is a mutation: the shipped sidecar must be ACCEPTED and
# each broken variant REJECTED, so the validator cannot pass by being a no-op.
# This scenario deliberately asserts nothing about the wording of SKILL.md; skill
# prose is meant to be trimmed, and pinning phrases here only taxes that work.
set -euo pipefail

skill_path="$ROOT/plugins/mega-orchestration/skills/wayfinding/SKILL.md"
sidecar_path="$ROOT/plugins/mega-orchestration/skills/wayfinding/agents/openai.yaml"
validator_path="$ROOT/scripts/validate-codex-skill-metadata"

skill_exists=1
sidecar_exists=1
[ -f "$skill_path" ]   && skill_exists=0
[ -f "$sidecar_path" ] && sidecar_exists=0

emit() {
  if [ "$2" -eq 0 ]; then echo "OK $1"; else echo "MISSING $1"; fi
}

# wayfinding is explicit-only on Codex. scripts/validate.sh exempts it from the
# .agents/skills discovery links on exactly that basis, so this marker is what
# keeps the exemption honest.
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
[ "$active_policy" = false ] && policy_rc=0

prompt_rc=1
if grep -q 'default_prompt:' "$sidecar_path" 2>/dev/null &&
   grep -Fq '$wayfinding' "$sidecar_path" 2>/dev/null; then
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

{
  emit wayfinding-skill-exists "$skill_exists"
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
} > out.txt

cat out.txt
