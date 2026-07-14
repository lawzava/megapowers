#!/usr/bin/env bash
# Dependency-free test for delegate-nudge.sh. Builds a throwaway git repo, feeds
# Stop-hook JSON on stdin, and asserts BLOCK (nudge fires) or ALLOW (no nudge).
# Run: plugins/mega-orchestration/hooks/tests/delegate-nudge.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
TMP="$(mktemp -d)"
cd "$TMP" || exit 1
git init -q
git config user.email t@t; git config user.name t; git config commit.gpgsign false
git commit -q --allow-empty -m init
TR="$TMP/transcript.jsonl"

# The hook now records a once-per-diff-state sentinel (see below), so each of
# these scenarios needs to start from "not yet nudged" to test what it always
# tested: whether THIS diff/transcript combination would trigger a nudge on a
# fresh Stop. The dedup behavior itself gets its own scenarios further down,
# where NOT resetting between two checks is the point.
SENTINEL="$(git rev-parse --git-path megapowers-delegate-nudge-seen)"
reset_sentinel() { rm -f "$SENTINEL" 2>/dev/null || true; }

pass=0; fail=0
verdict() {
  local out; out="$(printf '%s' "$1" | bash "$HOOK" 2>/dev/null)"
  if printf '%s' "$out" | grep -q '"decision":"block"'; then echo BLOCK; else echo ALLOW; fi
}
j() { printf '{"stop_hook_active":%s,"transcript_path":"%s"}' "$1" "$2"; }
check() {
  if [ "$1" = "$(verdict "$2")" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s :: %s\n' "$1" "$3"; fi
}

echo "== delegate-nudge tests =="
: > "$TR"
check ALLOW "$(j true "$TR")"  "stop_hook_active loop guard"
check ALLOW "$(j false "$TR")" "clean repo, nothing to review"

printf 'func handler() {}\n' > svc.go; git add svc.go; git commit -qm add
printf 'func handler() { billing() }\n' > svc.go   # keep this risky diff for the cases below
: > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "risky tracked change, no delegate -> nudge"

# A real MCP delegate tool_use (JSON name field) suppresses the nudge.
printf '{"type":"tool_use","name":"mcp__codex__codex","input":{"prompt":"review"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "real mcp__codex__codex tool_use suppresses nudge"

# A real Bash CLI invocation (inside a command field) suppresses.
printf '{"type":"tool_use","name":"Bash","input":{"command":"codex exec \\"review the diff\\""}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "real codex exec Bash command suppresses nudge"

# The first-party codex-plugin-cc wraps review commands through its companion
# script, so those invocations must count as an independent Codex review too.
printf '{"type":"tool_use","name":"Bash","input":{"command":"node \\"${CLAUDE_PLUGIN_ROOT}/scripts/codex-companion.mjs\\" review --wait"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "codex-plugin-cc review command suppresses nudge"

printf '{"type":"tool_use","name":"Bash","input":{"command":"node \\"${CLAUDE_PLUGIN_ROOT}/scripts/codex-companion.mjs\\" adversarial-review --wait"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "codex-plugin-cc adversarial review command suppresses nudge"

printf '{"type":"tool_use","name":"Bash","input":{"command":"node \\"${CLAUDE_PLUGIN_ROOT}/scripts/codex-companion.mjs\\" status"}}\n' > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "codex-plugin-cc status command does NOT suppress review nudge"

# A real delegate subagent dispatch suppresses.
printf '{"type":"tool_use","name":"Task","input":{"subagent_type":"model-delegate"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "model-delegate subagent dispatch suppresses nudge"

# Codex rollout transcripts record shell calls as exec_command({cmd:\"...\"})
# inside a custom_tool_call input string (observed shape, codex-cli 0.144), so a
# delegate CLI invoked from a Codex lead must suppress through that shape too.
printf '{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","input":"const r = await tools.exec_command({cmd:\\"claude -p --model claude-fable-5 (refute this diff)\\",\\"workdir\\":\\"/x\\"}); text(r.output);"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "codex rollout exec_command cmd shape suppresses nudge"

# Rollouts also carry the JSON-serialized key form ("cmd" escaped): both occur
# in real codex-cli 0.144 session files, so both must suppress.
printf '{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\\"cmd\\":\\"claude -p --model claude-fable-5 (verify the diff)\\",\\"workdir\\":\\"/x\\"}"}}\n' > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "codex rollout escaped-json cmd shape suppresses nudge"

# But a mere prose mention of the same CLI (no cmd: anchor) must NOT suppress.
printf 'the docs say you can run claude -p to get a review\n' > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "prose claude -p mention does NOT suppress"

# The static fallback (no detect entries parse) must still recognize a
# rollout-shaped claude -p review rather than nudging over a done review.
NODETECT="$TMP/nodetect.toml"
printf '[providers.alpha]\nbinary = "sh"\n' > "$NODETECT"
printf '{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","input":"const r = await tools.exec_command({cmd:\\"claude -p --model claude-fable-5 (refute)\\"}); text(r.output);"}}\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | MODELS_TOML="$NODETECT" DELEGATES_TOML="$NODETECT" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then fail=$((fail + 1)); printf '  FAIL static fallback misses rollout claude -p review\n'; else pass=$((pass + 1)); fi

# REGRESSION (C8/C25): merely MENTIONING delegate names/CLIs in prose or a read
# doc must NOT suppress — this is the whole point of the fix.
printf 'assistant discussed mcp__codex__codex, codex exec, and model-delegate from the docs\n' > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "prose mention of delegate names/CLIs does NOT suppress"

# A read code block whose content contains "codex exec" (as file text, not a command field).
printf '{"type":"tool_result","content":"...run `codex exec` and `agy exec` per the SKILL..."}\n' > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "delegate CLI inside read file content does NOT suppress"

printf 'plain notes, no delegate here\n' > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "risky change, only prose transcript -> nudge"

# REGRESSION: the word 'author' must not trip the risky detector.
git checkout -q -- svc.go
printf 'func handler() {}\n// author: someone\n' > svc.go
: > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "'author' does not match as risky (auth false positive)"
# But real auth code does.
printf 'func authenticate(u User) {}\n' > svc.go
: > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "'authenticate' is correctly risky"
# Restore the risky diff for the remaining cases.
printf 'func handler() { billing() }\n' > svc.go

git checkout -q -- svc.go
printf 'func newPaymentWebhook() {}\n' > payment_handler.go   # untracked
: > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "untracked risky new file (git diff HEAD misses it)"

rm -f payment_handler.go
printf 'hello world\n' > notes.txt
: > "$TR"
reset_sentinel
check ALLOW "$(j false "$TR")" "untracked benign file -> no nudge"
rm -f notes.txt

echo "== once-per-diff-state tests =="

# (a) same risky diff, two consecutive Stop events: first blocks, second must NOT.
printf 'func handler() { billing() }\n' > svc.go
: > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "(a) first stop on a risky diff -> nudge"
check ALLOW "$(j false "$TR")" "(a) second stop, SAME risky diff -> already nudged, no re-block"

# (b) the diff then changes (a new/different risky hunk) -> nudges again.
printf 'func handler() { billing(); stripe() }\n' > svc.go
check BLOCK "$(j false "$TR")" "(b) diff changed (new risky hunk) -> nudges again"
# and immediately re-checking that SAME new state does not re-block either.
check ALLOW "$(j false "$TR")" "(b) same new diff-state again -> no re-block"

# (c) existing delegate-call suppression still wins even once a sentinel is stored:
# a real delegate invocation allows regardless of diff-state history.
printf 'func handler() { billing(); stripe(); webhook() }\n' > svc.go
: > "$TR"
check BLOCK "$(j false "$TR")" "(c) setup: new risky diff nudges and stores a sentinel"
printf '{"type":"tool_use","name":"mcp__codex__codex","input":{"prompt":"review"}}\n' > "$TR"
check ALLOW "$(j false "$TR")" "(c) delegate call still suppresses with a sentinel on disk"

# Reverting to a diff-state that was already seen and blocked earlier (case (a)'s
# state) must nudge again: the sentinel only remembers the MOST RECENT state, not
# a history of every state ever seen, so returning to an old risky diff after
# something else happened in between is treated as new again.
printf 'func handler() { billing() }\n' > svc.go
: > "$TR"
check BLOCK "$(j false "$TR")" "reverting to an earlier (but not last-seen) risky state nudges again"

git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go

echo "== sentinel survives commit (reviewer repro) =="

# Reviewer-reported bug: a risky diff nags once; if it's then committed (the
# clean-tree early exit allows, but the sentinel used to be left on disk); and
# the SAME hunk reappears uncommitted later (revert, cherry-pick, unstash),
# the stale sentinel used to still match and silently ALLOW a new unreviewed
# risky diff. Any disappearance of the risky state must re-arm the nudge.
git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go
BASE_SHA="$(git rev-parse HEAD)"

printf 'func handler() { billing() }\n' > svc.go   # risky diff D, uncommitted
: > "$TR"
reset_sentinel
check BLOCK "$(j false "$TR")" "(repro-1) risky diff D nags once"

git commit -qam "commit risky diff D"
check ALLOW "$(j false "$TR")" "(repro-2) D committed, clean tree -> allow"
if [ -f "$SENTINEL" ]; then
  fail=$((fail + 1)); printf '  FAIL sentinel still on disk after clean-tree allow (should be cleared)\n'
else
  pass=$((pass + 1))
fi

git checkout -q "$BASE_SHA" -- svc.go
git commit -qm "revert D"
printf 'func handler() { billing() }\n' > svc.go   # same hunk as D, uncommitted again
check BLOCK "$(j false "$TR")" "(repro-3) same risky hunk reappears uncommitted -> must nag again, not silently allow"

git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go

echo "== detect-from-config tests =="

# A provider declared only in a custom config layer must suppress the nudge
# when its CLI appears in a real Bash command field.
CUSTOM_TOML="$TMP/custom-delegates.toml"
cat > "$CUSTOM_TOML" <<'EOF'
[providers.mycli]
vendor = "acme"
binary = "mycli"
channel = "cli"
detect = ["mycli review"]
EOF
printf 'func handler() { billing() }\n' > svc.go
printf '{"type":"tool_use","name":"Bash","input":{"command":"mycli review the diff"}}\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | DELEGATES_TOML="$CUSTOM_TOML" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then
  fail=$((fail + 1)); printf '  FAIL custom-provider detect should suppress the nudge\n'
else
  pass=$((pass + 1))
fi

# A prose mention of the same CLI must still nudge (command-field anchored).
printf 'notes say run mycli review sometime\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | DELEGATES_TOML="$CUSTOM_TOML" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL prose mention of a detect entry must not suppress\n'
fi

# Unreadable config falls back to the static regex: codex exec still suppresses.
printf '{"type":"tool_use","name":"Bash","input":{"command":"codex exec \\"review\\""}}\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | DELEGATES_TOML="$TMP/does-not-exist.toml" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then
  fail=$((fail + 1)); printf '  FAIL static-regex fallback should still suppress codex exec\n'
else
  pass=$((pass + 1))
fi
git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go

# A whitespace-only detect entry must not become a match-anything pattern.
cat > "$TMP/blank-detect.toml" <<'EOF'
[providers.blank]
vendor = "acme"
binary = "blankcli"
channel = "cli"
detect = [" "]
EOF
printf 'func handler() { billing() }\n' > svc.go
printf '{"type":"tool_use","name":"Bash","input":{"command":"echo hello world"}}\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | DELEGATES_TOML="$TMP/blank-detect.toml" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL whitespace-only detect entry must not suppress the nudge\n'
fi
git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go

# detect entries sourced from a models.toml catalog layer.
CUSTOM_MODELS="$TMP/custom-models.toml"
cat > "$CUSTOM_MODELS" <<'EOF'
[providers.newcli]
vendor = "acme"
detect = ["newcli verify"]
EOF
EMPTY_DELEGATES="$TMP/empty-delegates.toml"
printf '[roles]\n' > "$EMPTY_DELEGATES"
printf 'func handler() { billing() }\n' > svc.go
printf '{"type":"tool_use","name":"Bash","input":{"command":"newcli verify the diff"}}\n' > "$TR"
reset_sentinel
out="$(printf '%s' "$(j false "$TR")" | MODELS_TOML="$CUSTOM_MODELS" DELEGATES_TOML="$EMPTY_DELEGATES" bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | grep -q '"decision":"block"'; then
  fail=$((fail + 1)); printf '  FAIL catalog-sourced detect entry should suppress the nudge\n'
else
  pass=$((pass + 1))
fi
git checkout -q -- svc.go 2>/dev/null || printf 'func handler() {}\n' > svc.go

echo "== $pass passed, $fail failed =="
rm -rf "$TMP"
[ "$fail" -eq 0 ]
