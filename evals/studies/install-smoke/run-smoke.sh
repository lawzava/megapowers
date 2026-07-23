#!/usr/bin/env bash
# run-smoke.sh — fresh-environment install + first-task smoke test, per harness.
#
#   run-smoke.sh --out DIR [--harnesses claude,codex,opencode,agy] [--repo DIR]
#   run-smoke.sh --out DIR --source lawzava/megapowers --ref v0.5.0
#                --version 0.5.0 --harnesses claude,codex
#
# For each harness this follows the REPO'S OWN DOCUMENTED install flow
# (docs/setup.md) in a fresh config home (credentials copied in, nothing else),
# then runs a first task that can only succeed if the installed skill is
# actually discoverable and loadable: the agent must quote, verbatim, the
# test-driven-development skill's core-principle sentence. Verdicts per
# assertion: PASS / FAIL / SKIP(reason). Requires real credentials — run
# OUTSIDE any credential-blocking sandbox. Exact-ref remote mode is the release
# gate: it fails on SKIP and proves the fetched commit and manifest versions.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_DEFAULT="$(cd "$HERE/../../.." && pwd)"

# The load probe: the agent must reproduce the test-driven-development skill's
# core-principle sentence VERBATIM (exact, case-sensitive, whole clause). This
# is the sentence committed at
# plugins/megapowers/skills/test-driven-development/SKILL.md ("Core principle:").
# A case-insensitive 5-word substring ("watch the test fail") a model can emit
# from generic TDD lore does NOT satisfy it, so a pass is evidence the installed
# skill text was actually loaded, not recited from prior knowledge.
QUOTE_SENTENCE="if you didn't watch the test fail, you don't know whether it tests the right thing."
quote_ok() { grep -qF "$QUOTE_SENTENCE" "$1" 2>/dev/null; }  # fixed-string, case-sensitive

results_ok() { # <results.tsv> <fail-on-skip:0|1>
  local results="$1" fail_on_skip="$2"
  grep -q $'\tFAIL\t' "$results" && return 1
  [ "$fail_on_skip" -eq 1 ] && grep -q $'\tSKIP\t' "$results" && return 1
  grep -q $'\tPASS\t' "$results"
}

# Oracle self-test (mutation suite): the verbatim sentence passes; generic TDD
# phrasing that satisfied the old loose grep must now FAIL. Needs no credentials.
if [ "${1:-}" = "--selftest" ]; then
  st="$(mktemp -d)"; trap 'rm -rf "$st"' EXIT; sf=0
  printf '%s\n' "$QUOTE_SENTENCE" > "$st/verbatim.out"
  # phrasing a model reconstructs from prior knowledge; contains the old 5-word
  # substring "watch the test fail" (which the previous grep -qi accepted).
  printf 'The core TDD principle: always watch the test fail first so you know it works.\n' > "$st/generic.out"
  if quote_ok "$st/verbatim.out"; then echo "ok   verbatim sentence matches"; else echo "FAIL verbatim sentence not matched"; sf=1; fi
  if quote_ok "$st/generic.out"; then echo "FAIL generic phrasing matched (nonce not enforced)"; sf=1; else echo "ok   generic phrasing rejected"; fi
  printf 'claude\tSKIP\tno credentials\ncodex\tSKIP\tno auth\n' > "$st/all-skip.tsv"
  if results_ok "$st/all-skip.tsv" 0; then echo "FAIL all-SKIP results accepted"; sf=1; else echo "ok   all-SKIP results rejected"; fi
  printf 'claude\tPASS\tloaded\ncodex\tSKIP\tno auth\n' > "$st/mixed.tsv"
  if results_ok "$st/mixed.tsv" 0; then echo "ok   optional SKIP accepted with a PASS"; else echo "FAIL optional SKIP rejected"; sf=1; fi
  if results_ok "$st/mixed.tsv" 1; then echo "FAIL strict SKIP accepted"; sf=1; else echo "ok   strict SKIP rejected"; fi
  if [ "$sf" -eq 0 ]; then echo "install-smoke selftest: PASS"; else echo "install-smoke selftest: FAIL"; fi
  exit "$sf"
fi

OUT="" REPO="$REPO_DEFAULT" HARNESSES="claude,codex,opencode,agy"
SOURCE="" REF="" VERSION="" FAIL_ON_SKIP=0 FETCHED=""
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --source) SOURCE="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --fail-on-skip) FAIL_ON_SKIP=1; shift ;;
    --harnesses) HARNESSES="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ -n "$OUT" ] || { echo "usage: run-smoke.sh --out DIR [--harnesses ..] [--repo DIR | --source OWNER/REPO --ref TAG --version VERSION]" >&2; exit 2; }
if [ -n "$SOURCE" ]; then
  [ "$REPO" = "$REPO_DEFAULT" ] || { echo "--repo and --source are mutually exclusive" >&2; exit 2; }
  [ -n "$REF" ] && [ -n "$VERSION" ] || { echo "--source requires --ref and --version" >&2; exit 2; }
  FAIL_ON_SKIP=1
elif [ -n "$REF$VERSION" ]; then
  echo "--ref and --version require --source" >&2
  exit 2
fi
mkdir -p "$OUT"; OUT="$(cd "$OUT" && pwd)"
: > "$OUT/results.tsv"

note() { printf '%s\t%s\t%s\n' "$1" "$2" "$3" | tee -a "$OUT/results.tsv"; }

if [ -n "$SOURCE" ]; then
  FETCHED="$(mktemp -d)"
  case "$SOURCE" in
    *://*|git@*) remote="$SOURCE" ;;
    *) remote="https://github.com/$SOURCE.git" ;;
  esac
  if ! timeout 300 git clone --quiet --depth 1 --branch "$REF" "$remote" "$FETCHED/repo" >"$OUT/fetch.log" 2>&1; then
    note source FAIL "fetch exact ref $SOURCE@$REF — see fetch.log"
    exit 1
  fi
  REPO="$FETCHED/repo"
  sha="$(git -C "$REPO" rev-parse HEAD)"
  if ! git -C "$REPO" tag --points-at HEAD | grep -Fxq "$REF"; then
    note source FAIL "fetched HEAD $sha is not exact tag $REF"
    exit 1
  fi
  bad_versions="$(
    find "$REPO/plugins" -type f \( -path '*/.claude-plugin/plugin.json' -o -path '*/.codex-plugin/plugin.json' \) -print0 |
      xargs -0 jq -r --arg v "$VERSION" 'select(.version != $v) | input_filename + "=" + (.version // "missing")'
  )"
  if [ -n "$bad_versions" ]; then
    printf '%s\n' "$bad_versions" > "$OUT/version-mismatches.log"
    note source FAIL "manifest version mismatch — see version-mismatches.log"
    exit 1
  fi
  jq -n --arg source "$SOURCE" --arg ref "$REF" --arg version "$VERSION" --arg sha "$sha" \
    '{source:$source, ref:$ref, version:$version, sha:$sha, mode:"exact-remote-ref"}' > "$OUT/source.json"
  note source PASS "fetched $SOURCE@$REF at $sha; all manifests are $VERSION"
fi

mapfile -t PLUGINS < <(find "$REPO/plugins" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort)

# The first task: only answerable by loading the installed skill (fresh homes
# contain no other copy of this text).
QUOTE_PROMPT='A plugin named "megapowers" that provides a skill called test-driven-development is installed in this environment. Load that skill and quote verbatim its one-sentence core principle (the sentence about watching a test fail). Output only that sentence.'

smoke_claude() {
  local h=claude cfg proj
  command -v claude >/dev/null || { note $h SKIP "claude CLI not installed"; return; }
  [ -f "$HOME/.claude/.credentials.json" ] || { note $h SKIP "no credentials"; return; }
  cfg="$(mktemp -d)"; proj="$(mktemp -d)"
  cp "$HOME/.claude/.credentials.json" "$cfg/"
  if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin marketplace add "$REPO" >"$OUT/claude-marketplace.log" 2>&1; then
    note $h PASS "marketplace add (local path)"
  else note $h FAIL "marketplace add — see claude-marketplace.log"; return; fi
  for plugin in "${PLUGINS[@]}"; do
    if CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude plugin install "$plugin@megapowers" >>"$OUT/claude-install.log" 2>&1; then
      note $h PASS "plugin install $plugin@megapowers"
    else note $h FAIL "plugin install $plugin — see claude-install.log"; return; fi
  done
  listed="$(CLAUDE_CONFIG_DIR="$cfg" timeout 120 claude plugin list 2>/dev/null || true)"
  for plugin in "${PLUGINS[@]}"; do
    if grep -qi "$plugin" <<< "$listed"; then note $h PASS "plugin listed: $plugin"
    else note $h FAIL "installed plugin missing from plugin list: $plugin"; fi
  done
  ( cd "$proj" && CLAUDE_CONFIG_DIR="$cfg" timeout 300 claude -p "$QUOTE_PROMPT" \
      --max-turns 8 --dangerously-skip-permissions --no-session-persistence \
      > "$OUT/claude-task.out" 2> "$OUT/claude-task.err" )
  if quote_ok "$OUT/claude-task.out"; then
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
  for plugin in "${PLUGINS[@]}"; do
    if CODEX_HOME="$ch" timeout 300 codex plugin add "$plugin@megapowers" >>"$OUT/codex-install.log" 2>&1; then
      note $h PASS "plugin add $plugin@megapowers"
    else note $h FAIL "plugin add $plugin — see codex-install.log"; return; fi
  done
  listed="$(CODEX_HOME="$ch" timeout 120 codex plugin list 2>/dev/null || true)"
  for plugin in "${PLUGINS[@]}"; do
    if grep -qi "$plugin" <<< "$listed"; then note $h PASS "plugin listed: $plugin"
    else note $h FAIL "installed plugin missing from plugin list: $plugin"; fi
  done
  ( cd "$proj" && CODEX_HOME="$ch" timeout 300 codex exec --ephemeral --skip-git-repo-check \
      -C "$proj" -s read-only "$QUOTE_PROMPT" \
      -o "$OUT/codex-task.out" > "$OUT/codex-task.log" 2> "$OUT/codex-task.err" </dev/null )
  if quote_ok "$OUT/codex-task.out"; then
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
  if quote_ok "$OUT/opencode-task.out"; then
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
  if quote_ok "$OUT/agy-task.out"; then
    note $h PASS "first task loaded the skill from .agents/skills"
  else note $h FAIL "first task did not surface the skill — see agy-task.out"; fi
}

for hh in ${HARNESSES//,/ }; do "smoke_$hh"; done

echo
echo "== install-smoke summary =="
column -t -s $'\t' "$OUT/results.tsv"
if results_ok "$OUT/results.tsv" "$FAIL_ON_SKIP"; then
  exit 0
fi
echo "install smoke failed: FAIL, strict SKIP, or no PASS result" >&2
exit 1
