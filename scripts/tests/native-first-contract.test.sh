#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLAUDE_MARKET="$ROOT/.claude-plugin/marketplace.json"
CODEX_MARKET="$ROOT/.agents/plugins/marketplace.json"

fail() {
  printf 'native-first contract: %s\n' "$*" >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || fail "jq is required"

expected_skills=$(printf '%s\n' \
  autonomous-run \
  code-quality \
  design-and-plan \
  humanizing-prose \
  independent-review \
  orchestrating \
  safe-effects \
  systematic-debugging \
  test-first-implementation \
  upgrading-megapowers \
  verify-and-finish)

assert_single_marketplace_plugin() {
  local manifest=$1 source_query=$2
  jq -e '.plugins | length == 1 and .[0].name == "megapowers"' "$manifest" >/dev/null ||
    fail "${manifest#"$ROOT/"} must expose only megapowers"
  jq -er "$source_query" "$manifest" | grep -qx './plugins/megapowers' ||
    fail "${manifest#"$ROOT/"} must point at ./plugins/megapowers"
}

assert_single_marketplace_plugin "$CLAUDE_MARKET" '.plugins[0].source'
assert_single_marketplace_plugin "$CODEX_MARKET" '.plugins[0].source.path'

actual_plugins=$(git -C "$ROOT" ls-files 'plugins/*' | cut -d/ -f2 | sort -u)
[[ $actual_plugins == megapowers ]] || fail "plugins/ must contain only megapowers, got: $actual_plugins"

for manifest in \
  "$ROOT/plugins/megapowers/.claude-plugin/plugin.json" \
  "$ROOT/plugins/megapowers/.codex-plugin/plugin.json"; do
  jq -e '.name == "megapowers"' "$manifest" >/dev/null ||
    fail "${manifest#"$ROOT/"} must declare megapowers"
done

actual_skills=$(git -C "$ROOT" ls-files 'plugins/megapowers/skills/*/SKILL.md' | cut -d/ -f4 | sort)
[[ $actual_skills == "$expected_skills" ]] ||
  fail "plugin skill inventory differs from the eleven native-first skills"

actual_links=$(git -C "$ROOT" ls-files -s '.agents/skills/*' | awk '$1 == "120000" { sub(".*/", "", $4); print $4 }' | sort)
[[ $actual_links == "$expected_skills" ]] ||
  fail ".agents/skills links differ from the plugin skill inventory"
while IFS= read -r skill; do
  target=$(readlink "$ROOT/.agents/skills/$skill")
  [[ $target == "../../plugins/megapowers/skills/$skill" ]] ||
    fail ".agents/skills/$skill points at unexpected target: $target"
  [[ -f "$ROOT/.agents/skills/$skill/SKILL.md" ]] ||
    fail ".agents/skills/$skill does not resolve to SKILL.md"
done <<< "$expected_skills"

hooks="$ROOT/plugins/megapowers/hooks/hooks.json"
jq -e '
  (.hooks | keys) == ["PreToolUse", "SessionStart"] and
  (.hooks.PreToolUse | length) == 1 and
  .hooks.PreToolUse[0].matcher == "Bash" and
  (.hooks.PreToolUse[0].hooks | length) == 1 and
  (.hooks.PreToolUse[0].hooks[0].command | contains("deny-destructive.sh")) and
  (.hooks.PreToolUse[0].hooks[0].command | contains("codex-deny-destructive.sh"))
' "$hooks" >/dev/null || fail "hooks.json must expose the native adapters"

expected_hook_files=$(printf '%s\n' \
  codex-deny-destructive.sh \
  codex-output-style.sh \
  deny-destructive.sh \
  dispatch.sh \
  hooks.json \
  run-hook.cmd)
actual_hook_files=$(find "$ROOT/plugins/megapowers/hooks" -maxdepth 1 -type f -printf '%f\n' | sort)
[[ $actual_hook_files == "$expected_hook_files" ]] ||
  fail "hooks/ differs from the supported adapter inventory"

expected_hook_tests=$(printf '%s\n' \
  codex-deny-destructive.test.sh \
  codex-output-style.test.sh \
  deny-destructive.test.sh \
  dispatch.test.sh)
actual_hook_tests=$(find "$ROOT/plugins/megapowers/hooks/tests" -maxdepth 1 -type f -printf '%f\n' | sort)
[[ $actual_hook_tests == "$expected_hook_tests" ]] ||
  fail "hooks/tests contains obsolete hook coverage"

removed_paths=(
  plugins/mega-orchestration
  plugins/mega-go
  plugins/mega-python
  plugins/mega-ts
  plugins/mega-frontend
  plugins/mega-guardrails
  plugins/megapowers/opencode
  plugins/megapowers/assets
  plugins/megapowers/agents
  plugins/megapowers/models.toml
  plugins/megapowers/enforcement.toml
  templates/OPENCODE.md
  templates/opencode.json
  templates/opencode-agents
  templates/grok
  templates/codex-agents
  templates/codex-config.toml
  templates/codex-complex.config.toml
  templates/settings.example.json
  scripts/check-enforcement.go
  scripts/check-enforcement.sh
  scripts/lib/validate-helpers.sh
  scripts/validate-codex-skill-metadata
  scripts/validate-parallel-tests.txt
  scripts/validate-parallel-tests.audit.tsv
)
for path in "${removed_paths[@]}"; do
  if git -C "$ROOT" ls-files -- "$path" "$path/*" | grep -q .; then
    fail "$path must be removed from the release artifact"
  fi
done

if git -C "$ROOT" ls-files plugins templates .agents | grep -Eqi '(^|/)(opencode|grok)(/|$)'; then
  fail "parallel-runtime integration paths remain"
fi

printf 'native-first contract: ok\n'
