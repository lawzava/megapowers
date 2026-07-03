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
check BLOCK "$(j false "$TR")" "risky tracked change, no delegate -> nudge"

# A real MCP delegate tool_use (JSON name field) suppresses the nudge.
printf '{"type":"tool_use","name":"mcp__codex__codex","input":{"prompt":"review"}}\n' > "$TR"
check ALLOW "$(j false "$TR")" "real mcp__codex__codex tool_use suppresses nudge"

# A real Bash CLI invocation (inside a command field) suppresses.
printf '{"type":"tool_use","name":"Bash","input":{"command":"codex exec \\"review the diff\\""}}\n' > "$TR"
check ALLOW "$(j false "$TR")" "real codex exec Bash command suppresses nudge"

# A real delegate subagent dispatch suppresses.
printf '{"type":"tool_use","name":"Task","input":{"subagent_type":"codex-delegate"}}\n' > "$TR"
check ALLOW "$(j false "$TR")" "codex-delegate subagent dispatch suppresses nudge"

# REGRESSION (C8/C25): merely MENTIONING delegate names/CLIs in prose or a read
# doc must NOT suppress — this is the whole point of the fix.
printf 'assistant discussed mcp__codex__codex, codex exec, and codex-delegate from the docs\n' > "$TR"
check BLOCK "$(j false "$TR")" "prose mention of delegate names/CLIs does NOT suppress"

# A read code block whose content contains "codex exec" (as file text, not a command field).
printf '{"type":"tool_result","content":"...run `codex exec` and `gemini -p` per the SKILL..."}\n' > "$TR"
check BLOCK "$(j false "$TR")" "delegate CLI inside read file content does NOT suppress"

printf 'plain notes, no delegate here\n' > "$TR"
check BLOCK "$(j false "$TR")" "risky change, only prose transcript -> nudge"

# REGRESSION: the word 'author' must not trip the risky detector.
git checkout -q -- svc.go
printf 'func handler() {}\n// author: someone\n' > svc.go
: > "$TR"
check ALLOW "$(j false "$TR")" "'author' does not match as risky (auth false positive)"
# But real auth code does.
printf 'func authenticate(u User) {}\n' > svc.go
: > "$TR"
check BLOCK "$(j false "$TR")" "'authenticate' is correctly risky"
# Restore the risky diff for the remaining cases.
printf 'func handler() { billing() }\n' > svc.go

git checkout -q -- svc.go
printf 'func newPaymentWebhook() {}\n' > payment_handler.go   # untracked
: > "$TR"
check BLOCK "$(j false "$TR")" "untracked risky new file (git diff HEAD misses it)"

rm -f payment_handler.go
printf 'hello world\n' > notes.txt
: > "$TR"
check ALLOW "$(j false "$TR")" "untracked benign file -> no nudge"

echo "== $pass passed, $fail failed =="
rm -rf "$TMP"
[ "$fail" -eq 0 ]
