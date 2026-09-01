#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STYLE="$ROOT/plugins/megapowers/output-styles/megapowers.md"
HOOKS="$ROOT/plugins/megapowers/hooks/hooks.json"
CODEX_ADAPTER="$ROOT/plugins/megapowers/hooks/codex-output-style.sh"

fail() {
  printf 'output style contract: %s\n' "$*" >&2
  exit 1
}

[[ -f $STYLE ]] || fail 'missing shipped Megapowers output style'
[[ -x $CODEX_ADAPTER ]] || fail 'missing executable Codex output-style adapter'

frontmatter=$(awk 'NR == 1 { next } /^---$/ { exit } { print }' "$STYLE")
grep -qx 'name: Megapowers' <<< "$frontmatter" || fail 'style name is not Megapowers'
grep -qx 'description: Direct, concise technical communication for the operator' <<< "$frontmatter" ||
  fail 'style description changed'
grep -qx 'keep-coding-instructions: true' <<< "$frontmatter" ||
  fail 'style does not preserve Claude Code coding instructions'
grep -qx 'force-for-plugin: true' <<< "$frontmatter" ||
  fail 'style is not the Claude Code plugin default'

grep -qF 'ASD-STE100-inspired principles' "$STYLE" || fail 'style omits its clarity baseline'
grep -qF 'Do not claim formal ASD-STE100 compliance.' "$STYLE" ||
  fail 'style overstates standards compliance'
grep -qF 'Lead with the answer, result, decision, or status in the first sentence.' "$STYLE" ||
  fail 'style does not require answer-first communication'
grep -qF 'Default to 100 prose words or fewer.' "$STYLE" || fail 'style omits its default prose budget'
grep -qF 'Do not exceed 250 prose words' "$STYLE" || fail 'style omits its prose ceiling'
grep -qF 'Do not use em dashes.' "$STYLE" || fail 'style omits the em-dash rule'
grep -qF 'load
  `humanizing-prose`' "$STYLE" || fail 'style omits the publishing-prose exception'
grep -qF 'Preserve exact identifiers, commands, numbers, caveats, decisions, and material uncertainty.' "$STYLE" ||
  fail 'style can discard load-bearing facts'
grep -qF 'named source, direct observation, or explicit uncertainty' "$STYLE" ||
  fail 'style permits vague attribution'
grep -qF 'actor, mechanism, scope, condition, or measurement' "$STYLE" ||
  fail 'style permits generic evaluation'
grep -qF 'this style takes precedence' "$STYLE" ||
  fail 'style does not claim precedence over built-in communication guidance'

jq -e '
  .hooks.SessionStart
  | length == 1 and
    .[0].hooks[0].type == "command" and
    .[0].hooks[0].command == "\"${CLAUDE_PLUGIN_ROOT}\"/hooks/run-hook.cmd codex-output-style.sh" and
    (.[0].hooks[0].timeout | type) == "number" and
    (.[0].hooks[0].statusMessage | type) == "string"
' "$HOOKS" >/dev/null || fail 'Codex startup hook does not load the shared style'

grep -qF 'shared default communication style' "$ROOT/README.md" ||
  fail 'root README omits the shared default style'
tr '\n' ' ' < "$ROOT/docs/harness-support.md" |
  grep -qF 'overrides a selected Claude Code output style' ||
  fail 'harness support docs omit the forced-style tradeoff'
grep -qF 'shared source for direct, concise technical replies' "$ROOT/plugins/megapowers/README.md" ||
  fail 'plugin README omits the shared style source'
grep -qF 'trusted Codex startup hook' "$ROOT/README.md" ||
  fail 'root README omits the Codex style adapter'
grep -qF 'review and trust the plugin hooks' "$ROOT/docs/install.md" ||
  fail 'install docs omit the Codex trust step'
grep -qF 'Codex startup hook' "$ROOT/plugins/megapowers/README.md" ||
  fail 'plugin README omits the Codex style adapter'

printf 'output style contract: ok\n'
