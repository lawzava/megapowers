#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf 'docs contract: %s\n' "$*" >&2
  exit 1
}

for file in \
  README.md \
  docs/install.md \
  docs/orchestration.md \
  docs/harness-support.md \
  docs/advanced/independent-review.md \
  docs/advanced/evals.md \
  docs/advanced/verification-maps.md \
  plugins/megapowers/README.md \
  evals/README.md \
  evals/RESULTS.md; do
  [[ -f "$ROOT/$file" ]] || fail "missing $file"
done

for path in docs/setup.md docs/agent-install.md docs/session-metrics.md templates; do
  [[ ! -e "$ROOT/$path" ]] || fail "obsolete surface remains: $path"
done

active_docs=(
  "$ROOT/README.md"
  "$ROOT/SECURITY.md"
  "$ROOT/CONTRIBUTING.md"
  "$ROOT/docs/install.md"
  "$ROOT/docs/orchestration.md"
  "$ROOT/docs/harness-support.md"
  "$ROOT/docs/advanced/independent-review.md"
  "$ROOT/docs/advanced/evals.md"
  "$ROOT/docs/advanced/verification-maps.md"
  "$ROOT/plugins/megapowers/README.md"
  "$ROOT/evals/README.md"
)

if rg -ni 'OpenCode|model(s|)[.]toml|delegates[.]toml|model catalog|model routing|mega-(orchestration|guardrails|go|python|ts|frontend)' "${active_docs[@]}"; then
  fail 'active docs retain removed runtime or plugin claims'
fi

if rg -ni 'dirty worktrees?|implicit dirty tree|untracked files|routing overrides' \
  "$ROOT/plugins/megapowers/skills/independent-review/SKILL.md" \
  "$ROOT/docs/advanced/independent-review.md"; then
  fail 'independent-review docs claim rejections the tool does not implement'
fi

if rg -n '\]\(\.\./\.\./' "$ROOT/plugins/megapowers/README.md"; then
  fail 'installed plugin README contains repository-relative links'
fi

grep -qF 'four different questions' "$ROOT/evals/README.md" ||
  fail 'eval README does not identify all four evidence layers'
grep -Eqi 'Claude.*enforce|enforce.*Claude' "$ROOT/evals/README.md" ||
  fail 'eval README does not state the Claude trigger-recall gate'
grep -Eqi 'Codex.*report-only|report-only.*Codex' "$ROOT/evals/README.md" ||
  fail 'eval README does not state Codex trigger-recall policy'
grep -qF 'Aggregate recall 116/117.' "$ROOT/evals/RESULTS.md" ||
  fail 'current trigger-recall aggregate does not match its table'
if grep -qF 'Only one forwarded segment may be open at a time.' \
  "$ROOT/evals/tools/sandbox-broker/README.md"; then
  fail 'sandbox broker docs retain the obsolete sequential-segment contract'
fi

while IFS= read -r markdown; do
  while IFS= read -r link; do
    target="${link#']('}"
    target="${target%')'}"
    target="${target%%#*}"
    case "$target" in *://*) continue ;; esac
    [[ -e "$(dirname "$markdown")/$target" ]] ||
      fail "$markdown links to missing $target"
  done < <(grep -oE '\]\([^ )]+[.]md(#[^)]*)?\)' "$markdown" || true)
done < <(git -C "$ROOT" ls-files '*.md' | sed "s#^#$ROOT/#")

grep -q 'exactly one plugin' "$ROOT/README.md" || fail 'README does not state the one-plugin boundary'
grep -q 'Claude Code and Codex' "$ROOT/README.md" || fail 'README does not state the two supported harnesses'
grep -q 'installed-plugin A/B' "$ROOT/README.md" || fail 'README does not identify optional behavioral evidence'
grep -qE '[~]/[.]config/megapowers/agent-capabilities[.]md' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs omit the personal capability registry'
grep -Eq 'advisory.*(not|no).*authorit|does not (grant|supply) authority' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs do not keep registry data advisory'
grep -Eq 'not parser-enforced|no parser' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs imply deterministic registry parsing'
if grep -Fqi 'Inline work remains the default' "$ROOT/docs/orchestration.md"; then
  fail 'orchestration docs retain the inline-default rule'
fi
grep -Eqi 'output-only lane|output-only work' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs omit output-only context isolation'
grep -Eqi 'native (team|task)' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs omit native durable coordination'
grep -q 'report-only' "$ROOT/docs/advanced/evals.md" || fail 'eval docs do not label PR replay report-only'
grep -q 'not a security boundary' "$ROOT/SECURITY.md" || fail 'security boundary warning missing'
grep -q '^| Skills |' "$ROOT/SECURITY.md" || fail 'security table omits the skills row'

# Counts derive from skills/catalog.json; no current document may restate one.
catalog_count="$(jq '.skills | length' "$ROOT/plugins/megapowers/skills/catalog.json")"
dir_count="$(find "$ROOT/plugins/megapowers/skills" -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l | tr -d ' ')"
[[ "$catalog_count" == "$dir_count" ]] || fail "catalog lists $catalog_count skills but $dir_count SKILL.md files ship"
grep -q 'skills/catalog.json' "$ROOT/README.md" || fail 'README does not point at the catalog as the inventory'
count_words='(ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen)[- ]skill'
for file in README.md .agents/skills/README.md SECURITY.md CONTRIBUTING.md docs/harness-support.md plugins/megapowers/README.md; do
  if grep -Eqi "$count_words" "$ROOT/$file"; then
    fail "$file restates a skill count; derive it from catalog.json"
  fi
done
# RESULTS.md keeps dated counts below its historical divider.
if sed -n '1,/^## Historical record$/p' "$ROOT/evals/RESULTS.md" | grep -Eqi "$count_words"; then
  fail 'evals/RESULTS.md current-candidate section restates a skill count'
fi
experimental_list="$(jq -r '[.skills[] | select(.status == "experimental") | "`" + .name + "`"] | join(", ")' \
  "$ROOT/plugins/megapowers/skills/catalog.json")"
while IFS= read -r name; do
  grep -q "\`$name\`" "$ROOT/README.md" ||
    fail "README omits experimental skill $name (catalog experimental set: $experimental_list)"
done < <(jq -r '.skills[] | select(.status == "experimental") | .name' "$ROOT/plugins/megapowers/skills/catalog.json")
grep -Eq 'inspect --file .* --provider claude' "$ROOT/docs/advanced/independent-review.md" ||
  fail 'review docs do not bind inspection to the provider'
grep -q 'approval_token' "$ROOT/docs/advanced/independent-review.md" ||
  fail 'review docs omit the inspection token'
grep -q -- '--approve-external "$approval_token"' "$ROOT/docs/advanced/independent-review.md" ||
  fail 'review docs do not bind dispatch to the inspected token'
grep -q 'claude plugin update megapowers@megapowers --scope <scope>' "$ROOT/docs/install.md" ||
  fail 'install docs omit the observed Claude scope on update'
grep -q 'codex plugin marketplace upgrade megapowers --json' "$ROOT/docs/install.md" ||
  fail 'install docs omit the JSON Codex marketplace refresh'
grep -q 'codex plugin add megapowers@megapowers --json' "$ROOT/docs/install.md" ||
  fail 'install docs omit the Codex reinstall after refresh'
grep -q 'git -C <marketplace-install-location> rev-parse HEAD' "$ROOT/docs/install.md" ||
  fail 'install docs omit refreshed-snapshot verification before registration'

printf 'docs contract: ok\n'
