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

if rg -ni 'OpenCode|Grok|model(s|)[.]toml|delegates[.]toml|model catalog|model routing|mega-(orchestration|guardrails|go|python|ts|frontend)' "${active_docs[@]}"; then
  fail 'active docs retain removed runtime or plugin claims'
fi

grep -q 'exactly one plugin' "$ROOT/README.md" || fail 'README does not state the one-plugin boundary'
grep -q 'Claude Code and Codex' "$ROOT/README.md" || fail 'README does not state the two supported harnesses'
grep -q 'installed-plugin A/B' "$ROOT/README.md" || fail 'README does not identify optional behavioral evidence'
grep -qE '[~]/[.]config/megapowers/agent-capabilities[.]md' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs omit the personal capability registry'
grep -Eq 'advisory.*(not|no).*authorit|does not (grant|supply) authority' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs do not keep registry data advisory'
grep -Eq 'not parser-enforced|no parser' "$ROOT/docs/orchestration.md" ||
  fail 'orchestration docs imply deterministic registry parsing'
grep -q 'report-only' "$ROOT/docs/advanced/evals.md" || fail 'eval docs do not label PR replay report-only'
grep -q 'not a security boundary' "$ROOT/SECURITY.md" || fail 'security boundary warning missing'
grep -q '| Twelve skills |' "$ROOT/SECURITY.md" || fail 'security capability count is stale'
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
