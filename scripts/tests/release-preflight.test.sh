#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
repo="$tmp/repo"
mkdir -p "$repo/scripts" "$repo/evals" "$repo/plugins/demo/.claude-plugin"
cp "$ROOT/scripts/release.sh" "$repo/scripts/release.sh"
printf '%s\n' '## 1.2.3 - test' > "$repo/CHANGELOG.md"
printf '%s\n' '{"version":"0.0.0"}' > "$repo/plugins/demo/.claude-plugin/plugin.json"
printf '%s\n' '/v0.0.0/docs/agent-install.md' > "$repo/README.md"

run_case() { # <eval rc> <fresh rc> <expected gate log>
  local eval_rc="$1" fresh_rc="$2" expected="$3" before rc
  printf '#!/usr/bin/env bash\necho eval >> gate.log\nexit %s\n' "$eval_rc" > "$repo/evals/run-all.sh"
  printf '#!/usr/bin/env bash\necho fresh >> gate.log\nexit %s\n' "$fresh_rc" > "$repo/scripts/check-freshness.sh"
  chmod +x "$repo/evals/run-all.sh" "$repo/scripts/check-freshness.sh"
  : > "$repo/gate.log"
  before="$(cat "$repo/plugins/demo/.claude-plugin/plugin.json")"
  set +e
  (cd "$repo" && bash scripts/release.sh 1.2.3) >/dev/null 2>&1
  rc=$?
  set -e
  [ "$rc" -ne 0 ]
  [ "$(cat "$repo/gate.log")" = "$expected" ]
  [ "$(cat "$repo/plugins/demo/.claude-plugin/plugin.json")" = "$before" ]
}

run_case 1 0 'eval'
run_case 0 1 $'eval\nfresh'
echo 'release preflight contract: ok'
