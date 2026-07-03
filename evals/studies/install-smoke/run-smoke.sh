#!/usr/bin/env bash
# run-smoke.sh — fresh-environment install + first-task smoke test, per harness.
#
#   run-smoke.sh --out DIR [--harnesses claude,codex,opencode,agy] [--repo DIR]
#
# For each harness this follows the REPO'S OWN DOCUMENTED install flow
# (docs/setup.md) in a fresh config home (credentials copied in, nothing else),
# then runs a first task that can only succeed if the installed skill is
# actually discoverable and loadable: the agent must quote, verbatim, the
# test-driven-development skill's core-principle sentence. Verdicts per
# assertion: PASS / FAIL / SKIP(reason). Requires real credentials — run
# OUTSIDE any credential-blocking sandbox. Exit 1 if anything FAILs (SKIPs ok).
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_DEFAULT="$(cd "$HERE/../../.." && pwd)"

OUT="" REPO="$REPO_DEFAULT" HARNESSES="claude,codex,opencode,agy"
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --harnesses) HARNESSES="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ -n "$OUT" ] || { echo "usage: run-smoke.sh --out DIR [--harnesses ..] [--repo DIR]" >&2; exit 2; }
mkdir -p "$OUT"; OUT="$(cd "$OUT" && pwd)"
: > "$OUT/results.tsv"

note() { printf '%s\t%s\t%s\n' "$1" "$2" "$3" | tee -a "$OUT/results.tsv"; }

# The first task: only answerable by loading the installed skill (fresh homes
# contain no other copy of this text).
QUOTE_PROMPT='A plugin named "megapowers" that provides a skill called test-driven-development is installed in this environment. Load that skill and quote verbatim its one-sentence core principle (the sentence about watching a test fail). Output only that sentence.'
QUOTE_RE='watch the test fail'

smoke_claude() {
  local h=claude cfg proj
  command -v claude >/dev/null || { note $h SKIP "claude CLI not installed"; return; }
  [ -f "$HOME/.claude/.credentials.json" ] || { note $h SKIP "no credentials"; return; }
  cfg="$(mktemp -d)"; proj="$(mktemp -d)"
  cp "$HOME/.claude/.credentials.json" "$cfg/"
  if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin marketplace add "$REPO" >"$OUT/claude-marketplace.log" 2>&1; then
    note $h PASS "marketplace add (local path)"
  else note $h FAIL "marketplace add — see claude-marketplace.log"; return; fi
  if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin install megapowers@megapowers >"$OUT/claude-install.log" 2>&1; then
    note $h PASS "plugin install megapowers@megapowers"
  else note $h FAIL "plugin install — see claude-install.log"; return; fi
  if CLAUDE_CONFIG_DIR="$cfg" timeout 120 claude plugin list 2>/dev/null | grep -qi megapowers; then
    note $h PASS "plugin listed"
  else note $h FAIL "installed plugin missing from plugin list"; fi
  ( cd "$proj" && CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude -p "$QUOTE_PROMPT" \
      --max-turns 8 --dangerously-skip-permissions --no-session-persistence \
      > "$OUT/claude-task.out" 2> "$OUT/claude-task.err" )
  if grep -qi "$QUOTE_RE" "$OUT/claude-task.out"; then
    note $h PASS "first task loaded the installed skill"
  else note $h FAIL "first task did not surface the skill — see claude-task.out"; fi
}

smoke_codex() {
  local h=codex ch proj
  command -v codex >/dev/null || { note $h SKIP "codex CLI not installed"; return; }
  [ -f "${CODEX_HOME:-$HOME/.codex}/auth.json" ] || { note $h SKIP "no auth.json"; return; }
  ch="$(mktemp -d)"; proj="$(mktemp -d)"
  cp "${CODEX_HOME:-$HOME/.codex}/auth.json" "$ch/"
  if CODEX_HOME="$ch" timeout 300 codex plugin marketplace add "$REPO" >"$OUT/codex-marketplace.log" 2>&1; then
    note $h PASS "marketplace add (local path)"
  else note $h FAIL "marketplace add — see codex-marketplace.log"; return; fi
  if CODEX_HOME="$ch" timeout 300 codex plugin add megapowers@megapowers >"$OUT/codex-install.log" 2>&1; then
    note $h PASS "plugin add megapowers@megapowers"
  else note $h FAIL "plugin add — see codex-install.log"; return; fi
  if CODEX_HOME="$ch" timeout 120 codex plugin list 2>/dev/null | grep -qi megapowers; then
    note $h PASS "plugin listed"
  else note $h FAIL "installed plugin missing from plugin list"; fi
  ( cd "$proj" && CODEX_HOME="$ch" timeout 300 codex exec --ephemeral --skip-git-repo-check \
      -C "$proj" -s read-only "$QUOTE_PROMPT" \
      -o "$OUT/codex-task.out" > "$OUT/codex-task.log" 2> "$OUT/codex-task.err" </dev/null )
  if grep -qi "$QUOTE_RE" "$OUT/codex-task.out" 2>/dev/null; then
    note $h PASS "first task loaded the installed skill"
  else note $h FAIL "first task did not surface the skill — see codex-task.out"; fi
}

smoke_opencode() {
  local h=opencode proj
  command -v opencode >/dev/null || { note $h SKIP "opencode CLI not installed"; return; }
  proj="$(mktemp -d)"
  if ! ( cd "$proj" && timeout 120 opencode run "Reply with exactly: OK" \
           > "$OUT/opencode-auth.out" 2> "$OUT/opencode-auth.err" ) \
     || ! grep -q OK "$OUT/opencode-auth.out"; then
    note $h SKIP "no working provider auth (opencode run failed)"; return
  fi
  # docs/setup.md: symlink the canonical skill dir; OpenCode reads Claude-compatible paths
  mkdir -p "$proj/.claude/skills"
  ln -s "$REPO/plugins/megapowers/skills/test-driven-development" "$proj/.claude/skills/test-driven-development"
  ( cd "$proj" && timeout 300 opencode run "$QUOTE_PROMPT" \
      > "$OUT/opencode-task.out" 2> "$OUT/opencode-task.err" )
  if grep -qi "$QUOTE_RE" "$OUT/opencode-task.out"; then
    note $h PASS "first task loaded the symlinked skill"
  else note $h FAIL "first task did not surface the skill — see opencode-task.out"; fi
}

smoke_agy() {
  local h=agy proj
  command -v agy >/dev/null || { note $h SKIP "agy CLI not installed"; return; }
  proj="$(mktemp -d)"
  if ! ( cd "$proj" && timeout 180 agy -p "Reply with exactly: OK" \
           > "$OUT/agy-auth.out" 2> "$OUT/agy-auth.err" ) \
     || ! grep -q OK "$OUT/agy-auth.out"; then
    note $h SKIP "no working auth (agy -p failed)"; return
  fi
  # docs/setup.md: Antigravity native skills are flat markdown under .agents/skills/
  mkdir -p "$proj/.agents/skills"
  ln -s "$REPO/plugins/megapowers/skills/test-driven-development" "$proj/.agents/skills/test-driven-development"
  ( cd "$proj" && timeout 300 agy -p "$QUOTE_PROMPT" --dangerously-skip-permissions \
      > "$OUT/agy-task.out" 2> "$OUT/agy-task.err" )
  if grep -qi "$QUOTE_RE" "$OUT/agy-task.out"; then
    note $h PASS "first task loaded the skill from .agents/skills"
  else note $h FAIL "first task did not surface the skill — see agy-task.out"; fi
}

for hh in ${HARNESSES//,/ }; do "smoke_$hh"; done

echo
echo "== install-smoke summary =="
column -t -s $'\t' "$OUT/results.tsv"
grep -q $'\tFAIL\t' "$OUT/results.tsv" && exit 1 || exit 0
