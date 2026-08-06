#!/usr/bin/env bash
set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$HERE/../session-metrics"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok()  { pass=$((pass + 1)); }
bad() { echo "FAIL $1"; shift; [ $# -eq 0 ] || printf '%s\n' "$@"; fail=$((fail + 1)); }

# The same slugification session-metrics uses to map a project directory to its
# transcript directory. Fixtures whose project must exist on disk (so --project
# can resolve a contract layer inside it) are named through this.
slug() { printf '%s' "$1" | sed -E 's/[^A-Za-z0-9]/-/g'; }

# --- fixture: one synthetic session, two assistant turns ------------------
#
# Turn A carries two tool_use blocks (Read, Edit) in a single record with no
# message.id. Turn B issues a single backgrounded Bash call. Expected:
# batched-tool-call ratio 1 of 2 turns = 0.5; backgrounded-Bash ratio 1 of 1
# Bash call = 1.0.
#
# This is a rare shape, not the usual one: Claude Code normally splits a
# response into one record per content block sharing a message.id (see the
# split-batch fixture at the bottom of this file, which is the common case).
# It is kept because it does occur, and because records with no message.id at
# all must each count as their own response rather than collapsing into one.
PROJECT_DIR="$TMP/root/-fixture-project"
mkdir -p "$PROJECT_DIR"
SESSION="$PROJECT_DIR/00000000-0000-0000-0000-000000000001.jsonl"

common='"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000001","version":"0.0.0"'

cat > "$SESSION" <<EOF
{"type":"user","uuid":"u0","timestamp":"2026-07-30T00:00:00.000Z","message":{"role":"user","content":"fix the thing"},$common}
{"type":"assistant","uuid":"a1","timestamp":"2026-07-30T00:00:02.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/fixture/a.go"}},{"type":"tool_use","id":"t2","name":"Edit","input":{"file_path":"/fixture/a.go"}}]},$common}
{"type":"user","uuid":"u1","timestamp":"2026-07-30T00:00:03.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"},{"type":"tool_result","tool_use_id":"t2","content":"ok"}]},$common}
{"type":"assistant","uuid":"a2","timestamp":"2026-07-30T00:00:05.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t3","name":"Bash","input":{"command":"sleep 60","run_in_background":true}}]},$common}
{"type":"user","uuid":"u2","timestamp":"2026-07-30T00:00:06.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t3","content":"started"}]},$common}
EOF

# --- RED confirmation happens by hand before session-metrics exists; this
# file is meant to fail until the script is implemented. -------------------

out="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --json 2>&1)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "script must exit 0 on a readable fixture store (got $rc)" "$out"
else
  ok
fi

batched="$(printf '%s' "$out" | jq -r '.aggregate.batched_tool_call_ratio' 2>/dev/null)"
if [ "$batched" = "0.5" ]; then
  ok
else
  bad "batched-tool-call ratio: want 0.5, got '$batched'" "$out"
fi

bg="$(printf '%s' "$out" | jq -r '.aggregate.backgrounded_bash_ratio' 2>/dev/null)"
if [ "$bg" = "1" ]; then
  ok
else
  bad "backgrounded-Bash ratio: want 1.0, got '$bg'" "$out"
fi

turns="$(printf '%s' "$out" | jq -r '.aggregate.assistant_turns' 2>/dev/null)"
if [ "$turns" = "2" ]; then
  ok
else
  bad "assistant turns: want 2, got '$turns'" "$out"
fi

# --- fixture: entrypoint set on only some records (Finding 1) -------------
#
# Real bug shape: entrypoint appears on only some records in the file, and
# every record that carries it says "sdk-py". One assistant turn lacks the
# field entirely (the old per-record filter kept it as a phantom "cli"
# turn); the other assistant turn carries entrypoint sdk-py (the old filter
# dropped only that one record, leaving a partial session behind instead of
# excluding it). The session must resolve to sdk-py as a whole and be
# excluded by default, not partially included.
SESSION2="$PROJECT_DIR/00000000-0000-0000-0000-000000000002.jsonl"
cat > "$SESSION2" <<EOF
{"type":"user","uuid":"s2u0","timestamp":"2026-07-29T00:00:00.000Z","message":{"role":"user","content":"spawn"},"isSidechain":false,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000002","version":"0.0.0"}
{"type":"assistant","uuid":"s2a1","timestamp":"2026-07-29T00:00:01.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"s2t1","name":"Read","input":{"file_path":"/fixture/b.go"}}]},"isSidechain":false,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000002","version":"0.0.0"}
{"type":"user","uuid":"s2u1","timestamp":"2026-07-29T00:00:02.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s2t1","content":"ok"}]},"isSidechain":false,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000002","version":"0.0.0"}
{"type":"assistant","uuid":"s2a2","timestamp":"2026-07-29T00:00:03.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"s2t2","name":"Bash","input":{"command":"true"}}]},"isSidechain":false,"entrypoint":"sdk-py","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000002","version":"0.0.0"}
{"type":"user","uuid":"s2u2","timestamp":"2026-07-29T00:00:04.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s2t2","content":"ok"}]},"isSidechain":false,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000002","version":"0.0.0"}
EOF

# --- fixture: isSidechain set on only some records (same resolution bug,
# the other session-level field). Missing on the first and last records,
# true on the middle two. --------------------------------------------------
SESSION3="$PROJECT_DIR/00000000-0000-0000-0000-000000000003.jsonl"
cat > "$SESSION3" <<EOF
{"type":"user","uuid":"s3u0","timestamp":"2026-07-31T00:00:00.000Z","message":{"role":"user","content":"spawn"},"entrypoint":"cli","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000003","version":"0.0.0"}
{"type":"assistant","uuid":"s3a1","timestamp":"2026-07-31T00:00:01.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"s3t1","name":"Read","input":{"file_path":"/fixture/c.go"}}]},"entrypoint":"cli","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000003","version":"0.0.0"}
{"type":"user","uuid":"s3u1","timestamp":"2026-07-31T00:00:02.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s3t1","content":"ok"}]},"entrypoint":"cli","isSidechain":true,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000003","version":"0.0.0"}
{"type":"assistant","uuid":"s3a2","timestamp":"2026-07-31T00:00:03.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"s3t2","name":"Edit","input":{"file_path":"/fixture/c.go"}}]},"entrypoint":"cli","isSidechain":true,"cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000003","version":"0.0.0"}
{"type":"user","uuid":"s3u2","timestamp":"2026-07-31T00:00:04.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s3t2","content":"ok"}]},"entrypoint":"cli","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000003","version":"0.0.0"}
EOF

# --- fixture: a session with zero assistant turns (Finding 4). ------------
SESSION4="$PROJECT_DIR/00000000-0000-0000-0000-000000000004.jsonl"
cat > "$SESSION4" <<EOF
{"type":"user","uuid":"s4u0","timestamp":"2026-07-30T12:00:00.000Z","message":{"role":"user","content":"never answered"},"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/project","sessionId":"00000000-0000-0000-0000-000000000004","version":"0.0.0"}
EOF

# --- default run: session-level resolution excludes the whole sdk-py and
# sidechain sessions (not a phantom partial view of either), the zero-turn
# session never appears, and only fixture 1 survives. -----------------------
out_default="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --json 2>&1)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "default run must exit 0" "$out_default"
else
  ok
fi

default_ids="$(printf '%s' "$out_default" | jq -c '.sessions | map(.session_id) | sort' 2>/dev/null)"
if [ "$default_ids" = '["00000000-0000-0000-0000-000000000001"]' ]; then
  ok
else
  bad "default session ids: want only session 1, got '$default_ids'" "$out_default"
fi

default_count="$(printf '%s' "$out_default" | jq -r '.aggregate.session_count' 2>/dev/null)"
if [ "$default_count" = "1" ]; then
  ok
else
  bad "default aggregate.session_count: want 1, got '$default_count'" "$out_default"
fi

# --- --include-all: the sdk-py and sidechain sessions come back in full,
# with their real turn counts, not the phantom partial view the old
# per-record filter produced; the zero-turn session still stays out. -------
out_all="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --include-all --json 2>&1)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "--include-all run must exit 0" "$out_all"
else
  ok
fi

all_ids="$(printf '%s' "$out_all" | jq -c '.sessions | map(.session_id) | sort' 2>/dev/null)"
want_all='["00000000-0000-0000-0000-000000000001","00000000-0000-0000-0000-000000000002","00000000-0000-0000-0000-000000000003"]'
if [ "$all_ids" = "$want_all" ]; then
  ok
else
  bad "--include-all session ids: want $want_all, got '$all_ids'" "$out_all"
fi

s2_turns="$(printf '%s' "$out_all" | jq -r '.sessions[] | select(.session_id == "00000000-0000-0000-0000-000000000002") | .assistant_turns' 2>/dev/null)"
if [ "$s2_turns" = "2" ]; then
  ok
else
  bad "entrypoint-fixture session assistant turns under --include-all: want 2, got '$s2_turns'" "$out_all"
fi

s3_turns="$(printf '%s' "$out_all" | jq -r '.sessions[] | select(.session_id == "00000000-0000-0000-0000-000000000003") | .assistant_turns' 2>/dev/null)"
if [ "$s3_turns" = "2" ]; then
  ok
else
  bad "isSidechain-fixture session assistant turns under --include-all: want 2, got '$s3_turns'" "$out_all"
fi

all_count="$(printf '%s' "$out_all" | jq -r '.aggregate.session_count' 2>/dev/null)"
if [ "$all_count" = "3" ]; then
  ok
else
  bad "--include-all aggregate.session_count: want 3, got '$all_count'" "$out_all"
fi

# --- Finding 2: aggregate span is the earliest start and latest end across
# included sessions, not null. ----------------------------------------------
agg_span_start="$(printf '%s' "$out_all" | jq -r '.aggregate.span_start' 2>/dev/null)"
agg_span_end="$(printf '%s' "$out_all" | jq -r '.aggregate.span_end' 2>/dev/null)"
if [ "$agg_span_start" = "2026-07-29T00:00:00.000Z" ]; then
  ok
else
  bad "aggregate span_start: want 2026-07-29T00:00:00.000Z, got '$agg_span_start'" "$out_all"
fi
if [ "$agg_span_end" = "2026-07-31T00:00:04.000Z" ]; then
  ok
else
  bad "aggregate span_end: want 2026-07-31T00:00:04.000Z, got '$agg_span_end'" "$out_all"
fi

# --- Finding 3: "n/a" text renders without a dangling unit suffix. --------
text_all="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --include-all 2>&1)"
if printf '%s' "$text_all" | grep -q 'n/as'; then
  bad "text output has a dangling unit suffix on n/a" "$text_all"
else
  ok
fi
if printf '%s' "$text_all" | grep -q 'bash p90: n/a$'; then
  ok
else
  bad "expected a genuine n/a bash p90 row (session 3 has no Bash calls)" "$text_all"
fi

# --- Finding 4: a session with zero assistant turns is never listed, even
# under --include-all. -------------------------------------------------------
if printf '%s' "$out_all" | jq -e '.sessions[] | select(.session_id == "00000000-0000-0000-0000-000000000004")' >/dev/null 2>&1; then
  bad "zero-turn session must not be listed" "$out_all"
else
  ok
fi

# --- fixture: the split-batch shape the harness actually writes ------------
#
# One model response, four records sharing message.id msg_batch01: a thinking
# block, a text block, then one record per tool_use. This is what Claude Code
# writes for a batch of two tool calls, and it is the shape that made the old
# per-record batched ratio structurally incapable of being non-zero.
#
# Expected: 1 model response, both tool_use blocks credited to it, so the
# batched-tool-call ratio is 1.0. Counting records instead would report 4
# turns and a ratio of 0. Model latency is 2s, the gap from the human prompt
# to the response's first record, not the 0.5s gap between streamed siblings.
BATCH_DIR="$TMP/root/-fixture-batch"
mkdir -p "$BATCH_DIR"
common5='"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/batch","sessionId":"00000000-0000-0000-0000-000000000005","version":"0.0.0"'
cat > "$BATCH_DIR/00000000-0000-0000-0000-000000000005.jsonl" <<EOF
{"type":"user","uuid":"s5u0","timestamp":"2026-07-28T00:00:00.000Z","message":{"role":"user","content":"read both files"},$common5}
{"type":"assistant","uuid":"s5a1","timestamp":"2026-07-28T00:00:02.000Z","message":{"id":"msg_batch01","role":"assistant","content":[{"type":"thinking","thinking":"plan it"}]},$common5}
{"type":"assistant","uuid":"s5a2","timestamp":"2026-07-28T00:00:02.500Z","message":{"id":"msg_batch01","role":"assistant","content":[{"type":"text","text":"Reading both."}]},$common5}
{"type":"assistant","uuid":"s5a3","timestamp":"2026-07-28T00:00:03.000Z","message":{"id":"msg_batch01","role":"assistant","content":[{"type":"tool_use","id":"s5t1","name":"Read","input":{"file_path":"/fixture/d.go"}}]},$common5}
{"type":"assistant","uuid":"s5a4","timestamp":"2026-07-28T00:00:03.000Z","message":{"id":"msg_batch01","role":"assistant","content":[{"type":"tool_use","id":"s5t2","name":"Grep","input":{"pattern":"func"}}]},$common5}
{"type":"user","uuid":"s5u1","timestamp":"2026-07-28T00:00:05.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s5t1","content":"ok"},{"type":"tool_result","tool_use_id":"s5t2","content":"ok"}]},$common5}
EOF

out_batch="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --project /fixture/batch --json 2>&1)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "split-batch run must exit 0" "$out_batch"
else
  ok
fi

b_turns="$(printf '%s' "$out_batch" | jq -r '.aggregate.assistant_turns' 2>/dev/null)"
if [ "$b_turns" = "1" ]; then
  ok
else
  bad "split-batch assistant turns: want 1 (records sharing message.id are one response), got '$b_turns'" "$out_batch"
fi

b_batched="$(printf '%s' "$out_batch" | jq -r '.aggregate.batched_tool_call_ratio' 2>/dev/null)"
if [ "$b_batched" = "1" ]; then
  ok
else
  bad "split-batch batched-tool-call ratio: want 1, got '$b_batched'" "$out_batch"
fi

b_median="$(printf '%s' "$out_batch" | jq -r '.aggregate.model_latency.median_s' 2>/dev/null)"
if [ "$b_median" = "2" ]; then
  ok
else
  bad "split-batch model latency median: want 2 (prompt to first record), got '$b_median'" "$out_batch"
fi

# --- fixture: model latency excludes tool execution -----------------------
#
# One Bash call that takes 6s, then a second response 3s after its
# tool_result. Latency must be the 3s composing gap measured from the
# tool_result, not the 9s gap from the previous response's last record, which
# would swallow the tool run.
LAT_DIR="$TMP/root/-fixture-latency"
mkdir -p "$LAT_DIR"
common6='"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/latency","sessionId":"00000000-0000-0000-0000-000000000006","version":"0.0.0"'
cat > "$LAT_DIR/00000000-0000-0000-0000-000000000006.jsonl" <<EOF
{"type":"user","uuid":"s6u0","timestamp":"2026-07-27T00:00:00.000Z","message":{"role":"user","content":"run it"},$common6}
{"type":"assistant","uuid":"s6a1","timestamp":"2026-07-27T00:00:02.000Z","message":{"id":"msg_lat01","role":"assistant","content":[{"type":"tool_use","id":"s6t1","name":"Bash","input":{"command":"true"}}]},$common6}
{"type":"user","uuid":"s6u1","timestamp":"2026-07-27T00:00:08.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s6t1","content":"ok"}]},$common6}
{"type":"assistant","uuid":"s6a2","timestamp":"2026-07-27T00:00:11.000Z","message":{"id":"msg_lat02","role":"assistant","content":[{"type":"text","text":"Done."}]},$common6}
EOF

out_lat="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --project /fixture/latency --json 2>&1)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "latency run must exit 0" "$out_lat"
else
  ok
fi

l_turns="$(printf '%s' "$out_lat" | jq -r '.aggregate.assistant_turns' 2>/dev/null)"
if [ "$l_turns" = "2" ]; then
  ok
else
  bad "latency-fixture assistant turns: want 2, got '$l_turns'" "$out_lat"
fi

l_p90="$(printf '%s' "$out_lat" | jq -r '.aggregate.model_latency.p90_s' 2>/dev/null)"
if [ "$l_p90" = "3" ]; then
  ok
else
  bad "latency-fixture p90: want 3 (composing gap from tool_result, not 9 including the tool run), got '$l_p90'" "$out_lat"
fi

# --- fixture: transcripts below the top level of a project directory -------
#
# The harness writes subagent transcripts to <slug>/<session-uuid>/subagents/**
# and workflow journals one level deeper again. On the author's store that is
# 688 files against 628 at the top level, carrying 36 hook_blocking_error
# records and 33 delegate-run round lines a top-level-only finder could not
# reach under any flag.
#
# The subagent transcript declares isSidechain true, like every one of them in
# the real store, so discovery finding it must not change the default report:
# the existing exclusion contract still decides, and --include-all is what
# brings it in. The journal has no timestamped records and must contribute
# nothing rather than crash the run.
DEEP_PROJECT="$TMP/proj/deep"
mkdir -p "$DEEP_PROJECT"
DEEP_DIR="$TMP/root/$(slug "$DEEP_PROJECT")"
DEEP_SUB="$DEEP_DIR/00000000-0000-0000-0000-000000000010/subagents"
mkdir -p "$DEEP_SUB/workflows/wf_deadbeef"
common10='"isSidechain":false,"entrypoint":"cli","cwd":"'"$DEEP_PROJECT"'","sessionId":"00000000-0000-0000-0000-000000000010","version":"0.0.0"'
common11='"isSidechain":true,"entrypoint":"cli","cwd":"'"$DEEP_PROJECT"'","sessionId":"00000000-0000-0000-0000-000000000011","version":"0.0.0"'

cat > "$DEEP_DIR/00000000-0000-0000-0000-000000000010.jsonl" <<EOF
{"type":"user","uuid":"s10u0","timestamp":"2026-07-23T00:00:00.000Z","message":{"role":"user","content":"fan it out"},$common10}
{"type":"assistant","uuid":"s10a1","timestamp":"2026-07-23T00:00:02.000Z","message":{"id":"msg_deep01","role":"assistant","content":[{"type":"tool_use","id":"s10t1","name":"Task","input":{"prompt":"work"}}]},$common10}
{"type":"user","uuid":"s10u1","timestamp":"2026-07-23T00:00:20.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s10t1","content":"done"}]},$common10}
EOF

cat > "$DEEP_SUB/agent-worker-90fb7d1da01640c3.jsonl" <<EOF
{"type":"user","uuid":"s11u0","timestamp":"2026-07-23T00:00:03.000Z","message":{"role":"user","content":"work"},$common11}
{"type":"assistant","uuid":"s11a1","timestamp":"2026-07-23T00:00:05.000Z","message":{"id":"msg_deep02","role":"assistant","content":[{"type":"tool_use","id":"s11t1","name":"Bash","input":{"command":"scripts/delegate-run --role verify --artifact worktree"}}]},$common11}
{"type":"user","uuid":"s11u1","timestamp":"2026-07-23T00:00:15.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s11t1","content":"verdict: needs_attention\nround:   4 (consecutive dispatch 4 of role 'verify' on 'feat/deep' without an approve)"}]},$common11}
{"type":"attachment","uuid":"s11x1","timestamp":"2026-07-23T00:00:16.000Z","attachment":{"type":"hook_blocking_error","hookName":"Stop","hookEvent":"Stop","blockingError":{"blockingError":"Risky logic (auth/billing/concurrency) changed without an independent review receipt.","command":"delegate-nudge.sh"}},$common11}
EOF

cat > "$DEEP_SUB/workflows/wf_deadbeef/journal.jsonl" <<'EOF'
{"type":"started","key":"v2:851f5a6158abf48752e31888a1b931f9","agentId":"aac53c7f0efad973b"}
{"type":"completed","key":"v2:851f5a6158abf48752e31888a1b931f9","agentId":"aac53c7f0efad973b"}
EOF

out_deep="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --project "$DEEP_PROJECT" --json 2>&1)"
deep_ids="$(printf '%s' "$out_deep" | jq -c '.sessions | map(.session_id) | sort' 2>/dev/null)"
if [ "$deep_ids" = '["00000000-0000-0000-0000-000000000010"]' ]; then
  ok
else
  bad "deep default ids: want only the lead session (the subagent is a sidechain), got '$deep_ids'" "$out_deep"
fi

out_deep_all="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --project "$DEEP_PROJECT" --include-all --json 2>&1)"
deep_all_ids="$(printf '%s' "$out_deep_all" | jq -c '.sessions | map(.session_id) | sort' 2>/dev/null)"
want_deep='["00000000-0000-0000-0000-000000000010","00000000-0000-0000-0000-000000000011"]'
if [ "$deep_all_ids" = "$want_deep" ]; then
  ok
else
  bad "deep --include-all ids: want the subagent transcript too, got '$deep_all_ids'" "$out_deep_all"
fi

# The journal is not a transcript. It must add no session and no error.
if printf '%s' "$out_deep_all" | jq -e '.aggregate.session_count == 2' >/dev/null 2>&1; then
  ok
else
  bad "workflow journal must contribute no session" "$out_deep_all"
fi

# Same finder, unscoped: a run with no --project reaches the same file.
out_deep_root="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --include-all --json 2>&1)"
if printf '%s' "$out_deep_root" | jq -e '.sessions[] | select(.session_id == "00000000-0000-0000-0000-000000000011")' >/dev/null 2>&1; then
  ok
else
  bad "unscoped --include-all must reach a subagent transcript below the top level" "$out_deep_root"
fi

# --- --rules: the enforcement-lifecycle report ----------------------------
#
# Every --rules run below is made from a scratch cwd with an empty
# XDG_CONFIG_HOME, so the contract is pinned instead of inherited from whatever
# the machine running the tests has installed.
#
# The cwd carries a DECOY .megapowers/enforcement.toml: a fire_prefix that
# matches nothing in any fixture and a window of 9. The measured project carries
# the real one. Resolving the project layer against the cwd (the old behavior)
# therefore reports zero fires and window 9 for every run below, which is the
# whole point: measuring one repository against another's contract has no
# symptom of its own.
CFG="$TMP/cfg"
mkdir -p "$CFG/.megapowers" "$TMP/xdg"
cat > "$CFG/.megapowers/enforcement.toml" <<'EOF'
[rules.skill-router]
state        = "advisory"
fire_prefix  = "DECOY, the cwd layer must not win: megapowers:"
honor_window = "9"
EOF

rules_run() { (cd "$CFG" && SESSION_METRICS_ROOT="$TMP/root" XDG_CONFIG_HOME="$TMP/xdg" "$SCRIPT" --rules "$@" 2>&1); }

# --- fixture: three advisory fires, one honored ---------------------------
#
# Fire 1 is answered by the named skill two assistant records later: honored.
# Fire 2 is answered five assistant records later, outside the window of 3, so
# it is a miss; it is deliberately inside the shipped window of 12, which is
# what makes it a test of the layered contract and not just of arithmetic.
# Fire 3 draws a Skill call for a different skill inside the window: also a
# miss, because honoring means invoking the skill the hook named. The honoring
# call says "megapowers:test-driven-development" while the hook names the bare
# skill, which is the plugin-prefix strip the comparison depends on.
#
# The project is a real directory holding the layer this run must be measured
# against, and it is not the cwd the command runs from.
ADV_PROJECT="$TMP/proj/advisory"
mkdir -p "$ADV_PROJECT/.megapowers"
cat > "$ADV_PROJECT/.megapowers/enforcement.toml" <<'EOF'
[rules.skill-router]
state        = "advisory"
fire_prefix  = "Trigger matched: megapowers:"
honor_window = "3"
EOF
ADV_DIR="$TMP/root/$(slug "$ADV_PROJECT")"
mkdir -p "$ADV_DIR"
common7='"isSidechain":false,"entrypoint":"cli","cwd":"'"$ADV_PROJECT"'","sessionId":"00000000-0000-0000-0000-000000000007","version":"0.0.0"'
fire() { printf '{"type":"attachment","uuid":"%s","timestamp":"%s","attachment":{"type":"hook_additional_context","hookName":"UserPromptSubmit","hookEvent":"UserPromptSubmit","content":["Trigger matched: megapowers:%s. Invoke it with the Skill tool before answering. Its procedure governs this turn."]},%s}\n' "$1" "$2" "$3" "$common7"; }
say()  { printf '{"type":"assistant","uuid":"%s","timestamp":"%s","message":{"id":"%s","role":"assistant","content":[{"type":"text","text":"thinking about it"}]},%s}\n' "$1" "$2" "$1" "$common7"; }
skill(){ printf '{"type":"assistant","uuid":"%s","timestamp":"%s","message":{"id":"%s","role":"assistant","content":[{"type":"tool_use","id":"%s","name":"Skill","input":{"skill":"megapowers:%s"}}]},%s}\n{"type":"user","uuid":"%s-r","timestamp":"%s","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"ok"}]},%s}\n' "$1" "$2" "$1" "$1" "$3" "$common7" "$1" "$2" "$1" "$common7"; }

{
  printf '{"type":"user","uuid":"s7u0","timestamp":"2026-07-26T00:00:00.000Z","message":{"role":"user","content":"implement the parser"},%s}\n' "$common7"
  fire  s7f1 2026-07-26T00:00:01.000Z test-driven-development
  say   s7a1 2026-07-26T00:00:02.000Z
  skill s7a2 2026-07-26T00:00:03.000Z test-driven-development
  fire  s7f2 2026-07-26T00:00:05.000Z systematic-debugging
  say   s7b1 2026-07-26T00:00:06.000Z
  say   s7b2 2026-07-26T00:00:07.000Z
  say   s7b3 2026-07-26T00:00:08.000Z
  say   s7b4 2026-07-26T00:00:09.000Z
  skill s7b5 2026-07-26T00:00:10.000Z systematic-debugging
  fire  s7f3 2026-07-26T00:00:12.000Z writing-plans
  skill s7c1 2026-07-26T00:00:13.000Z brainstorming
  # The model quoting the hook's line back into its own answer. Not a fire: the
  # hook fired once, and counting the echo would inflate the denominator every
  # time a session talks about the router.
  printf '{"type":"assistant","uuid":"s7c2","timestamp":"2026-07-26T00:00:14.000Z","message":{"id":"s7c2","role":"assistant","content":[{"type":"text","text":"The hook said Trigger matched: megapowers:writing-plans, so I will."}]},%s}\n' "$common7"
  # A tool_result echoing the literal out of the hook's own source. This is the
  # shape that dominates a real store (29 of 36 counted "fires" on the author's,
  # nearly all of them greps and cats run while auditing this very rule), and it
  # is a user record, so the old "any non-assistant record" rule counted both
  # occurrences as fires and put the honor rate at a third of its real value.
  printf '{"type":"user","uuid":"s7d1","timestamp":"2026-07-26T00:00:15.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s7d0","content":"skill-router.sh:12:  printf \\"Trigger matched: megapowers:%%s\\\\n\\"\\nenforcement.toml:9:fire_prefix = \\"Trigger matched: megapowers:\\""}]},%s}\n' "$common7"
  # A queue-operation record carrying a queued prompt that quotes the literal.
  # Neither an assistant record nor a hook delivery, so nothing but a delivery
  # check keeps it out.
  printf '{"type":"queue-operation","uuid":"s7d2","timestamp":"2026-07-26T00:00:16.000Z","operation":"add","prompt":"check whether Trigger matched: megapowers:writing-plans ever fires",%s}\n' "$common7"
} > "$ADV_DIR/00000000-0000-0000-0000-000000000007.jsonl"

out_adv="$(rules_run --project "$ADV_PROJECT" --json)"; rc=$?
if [ "$rc" -ne 0 ]; then
  bad "--rules run must exit 0" "$out_adv"
else
  ok
fi

adv_fires="$(printf '%s' "$out_adv" | jq -r '.rules.advisory_honor.fires' 2>/dev/null)"
if [ "$adv_fires" = "3" ]; then
  ok
else
  bad "advisory fires: want 3, got '$adv_fires'" "$out_adv"
fi

adv_honored="$(printf '%s' "$out_adv" | jq -r '.rules.advisory_honor.honored' 2>/dev/null)"
if [ "$adv_honored" = "1" ]; then
  ok
else
  bad "advisory honored: want 1 (in-window only, matching skill only), got '$adv_honored'" "$out_adv"
fi

adv_rate="$(printf '%s' "$out_adv" | jq -r '.rules.advisory_honor.honor_rate' 2>/dev/null)"
if [ "$adv_rate" = "0.33" ]; then
  ok
else
  bad "advisory honor rate: want 0.33, got '$adv_rate'" "$out_adv"
fi

adv_window="$(printf '%s' "$out_adv" | jq -r '.rules.advisory_honor.rules[] | select(.rule == "skill-router") | .honor_window' 2>/dev/null)"
if [ "$adv_window" = "3" ]; then
  ok
else
  bad "honor window: want 3 from the project layer, got '$adv_window'" "$out_adv"
fi

# Four prefix occurrences sit outside a hook delivery: one assistant echo, two
# lines in one tool_result, one queued prompt. None is a fire, all four are
# reported, so a harness that renames the delivery record shows up as zero fires
# beside a live echo count instead of as silence.
adv_echoes="$(printf '%s' "$out_adv" | jq -r '.rules.advisory_honor.echoes' 2>/dev/null)"
if [ "$adv_echoes" = "4" ]; then
  ok
else
  bad "advisory echoes: want 4 (1 assistant, 2 in one tool_result, 1 queue-operation), got '$adv_echoes'" "$out_adv"
fi

# The layer that produced those numbers came from --project, not from the cwd
# the command ran in. The decoy in $CFG defines the same keys and must lose.
adv_layer="$(printf '%s' "$out_adv" | jq -r '.rules.contract_layers[0]' 2>/dev/null)"
if [ "$adv_layer" = "$ADV_PROJECT/.megapowers/enforcement.toml" ]; then
  ok
else
  bad "contract project layer: want $ADV_PROJECT/.megapowers/enforcement.toml, got '$adv_layer'" "$out_adv"
fi

if printf '%s' "$out_adv" | jq -e --arg cfg "$CFG/.megapowers/enforcement.toml" \
     '.rules.contract_layers | index($cfg)' >/dev/null 2>&1; then
  bad "the cwd .megapowers layer was read instead of the measured project's" "$out_adv"
else
  ok
fi

# --- fixture: echoes with no delivery at all ------------------------------
#
# This is what a harness that renamed the delivery record shape looks like:
# the literal is all over the transcript and not one occurrence is a fire. The
# report must say zero fires beside a live echo count. It reads as silence if
# echo-only sessions are dropped before pooling, which is the failure mode that
# makes narrowing to a delivery risky in the first place.
ECHO_PROJECT="$TMP/proj/echo"
mkdir -p "$ECHO_PROJECT/.megapowers"
cat > "$ECHO_PROJECT/.megapowers/enforcement.toml" <<'EOF'
[rules.skill-router]
state        = "advisory"
fire_prefix  = "Trigger matched: megapowers:"
honor_window = "3"
EOF
ECHO_DIR="$TMP/root/$(slug "$ECHO_PROJECT")"
mkdir -p "$ECHO_DIR"
common12='"isSidechain":false,"entrypoint":"cli","cwd":"'"$ECHO_PROJECT"'","sessionId":"00000000-0000-0000-0000-000000000012","version":"0.0.0"'
cat > "$ECHO_DIR/00000000-0000-0000-0000-000000000012.jsonl" <<EOF
{"type":"user","uuid":"s12u0","timestamp":"2026-07-22T00:00:00.000Z","message":{"role":"user","content":"audit the router"},$common12}
{"type":"assistant","uuid":"s12a1","timestamp":"2026-07-22T00:00:02.000Z","message":{"id":"msg_echo01","role":"assistant","content":[{"type":"tool_use","id":"s12t1","name":"Bash","input":{"command":"grep -rn 'Trigger matched' hooks/"}}]},$common12}
{"type":"user","uuid":"s12u1","timestamp":"2026-07-22T00:00:03.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s12t1","content":"skill-router.sh:12: Trigger matched: megapowers:brainstorming\nskill-router.sh:40: Trigger matched: megapowers:writing-plans"}]},$common12}
EOF

out_echo="$(rules_run --project "$ECHO_PROJECT" --json)"
echo_fires="$(printf '%s' "$out_echo" | jq -r '.rules.advisory_honor.fires' 2>/dev/null)"
echo_echoes="$(printf '%s' "$out_echo" | jq -r '.rules.advisory_honor.echoes' 2>/dev/null)"
if [ "$echo_fires" = "0" ] && [ "$echo_echoes" = "2" ]; then
  ok
else
  bad "echo-only session: want 0 fires and 2 echoes, got '$echo_fires' and '$echo_echoes'" "$out_echo"
fi

echo_rate="$(printf '%s' "$out_echo" | jq -r '.rules.advisory_honor.honor_rate' 2>/dev/null)"
if [ "$echo_rate" = "null" ]; then
  ok
else
  bad "echo-only session honor rate: want null (no fire to honor), got '$echo_rate'" "$out_echo"
fi

# --- fixture: Stop-hook gate fires ----------------------------------------
#
# Three blocks: the current risky-logic wording, the legacy wording of the same
# rule (one rule, not two), and the autonomous-run gate. The stop_hook_summary
# record the harness writes alongside the first block is included verbatim,
# because counting it too would double every gate fire.
GATE_DIR="$TMP/root/-fixture-gate"
mkdir -p "$GATE_DIR"
common8='"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/gate","sessionId":"00000000-0000-0000-0000-000000000008","version":"0.0.0"'
cat > "$GATE_DIR/00000000-0000-0000-0000-000000000008.jsonl" <<EOF
{"type":"user","uuid":"s8u0","timestamp":"2026-07-25T00:00:00.000Z","message":{"role":"user","content":"ship it"},$common8}
{"type":"attachment","uuid":"s8x1","timestamp":"2026-07-25T00:00:01.000Z","attachment":{"type":"hook_blocking_error","hookName":"Stop","hookEvent":"Stop","blockingError":{"blockingError":"Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run delegate-run --role verify.","command":"\${CLAUDE_PLUGIN_ROOT}/hooks/run-hook.cmd dispatch.sh delegate-nudge.sh delegate-nudge.sh"}},$common8}
{"type":"system","subtype":"stop_hook_summary","uuid":"s8x1s","timestamp":"2026-07-25T00:00:01.500Z","hookErrors":["Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run delegate-run --role verify."],$common8}
{"type":"attachment","uuid":"s8x2","timestamp":"2026-07-25T00:00:02.000Z","attachment":{"type":"hook_blocking_error","hookName":"Stop","hookEvent":"Stop","blockingError":{"blockingError":"Risky logic (auth/billing/concurrency) changed without an independent review receipt.","command":"\${CLAUDE_PLUGIN_ROOT}/hooks/delegate-nudge.sh"}},$common8}
{"type":"attachment","uuid":"s8x3","timestamp":"2026-07-25T00:00:03.000Z","attachment":{"type":"hook_blocking_error","hookName":"Stop","hookEvent":"Stop","blockingError":{"blockingError":"Autonomous run backlog-2026-07 is active (STATE=working, CURSOR=3).","command":"\${CLAUDE_PLUGIN_ROOT}/hooks/run-loop.sh"}},$common8}
{"type":"assistant","uuid":"s8a1","timestamp":"2026-07-25T00:00:04.000Z","message":{"id":"msg_gate01","role":"assistant","content":[{"type":"text","text":"Blocked."}]},$common8}
EOF

out_gate="$(rules_run --project /fixture/gate --json)"
gate_total="$(printf '%s' "$out_gate" | jq -r '.rules.gate_fires.total' 2>/dev/null)"
if [ "$gate_total" = "3" ]; then
  ok
else
  bad "gate fires: want 3 (the stop_hook_summary twin is not a second block), got '$gate_total'" "$out_gate"
fi

gate_by_rule="$(printf '%s' "$out_gate" | jq -c '.rules.gate_fires.by_rule' 2>/dev/null)"
if [ "$gate_by_rule" = '{"autonomous-run-continuation":1,"risky-logic-review":2}' ]; then
  ok
else
  bad "gate fires by rule: want 2 risky-logic (current plus legacy wording) and 1 autonomous-run, got '$gate_by_rule'" "$out_gate"
fi

gate_top="$(printf '%s' "$out_gate" | jq -c '.rules.gate_fires.by_project[0]' 2>/dev/null)"
if [ "$gate_top" = '{"project":"/fixture/gate","count":3}' ]; then
  ok
else
  bad "gate fires top project: want /fixture/gate 3, got '$gate_top'" "$out_gate"
fi

# --- fixture: verify rounds and delegate-run wall clock --------------------
#
# Three delegate-run calls: 10s, 20s, and one that hit the harness Bash cap and
# was moved to the background at 600s. The second prints round 22, so a parser
# reading a single digit would report 2. A Read of delegate-run's own source
# prints the same round sentence with a larger number and must not count: only
# a result belonging to a Bash call is a dispatch.
DR_DIR="$TMP/root/-fixture-delegate"
mkdir -p "$DR_DIR"
common9='"isSidechain":false,"entrypoint":"cli","cwd":"/fixture/delegate","sessionId":"00000000-0000-0000-0000-000000000009","version":"0.0.0"'
cat > "$DR_DIR/00000000-0000-0000-0000-000000000009.jsonl" <<EOF
{"type":"user","uuid":"s9u0","timestamp":"2026-07-24T00:00:00.000Z","message":{"role":"user","content":"verify it"},$common9}
{"type":"assistant","uuid":"s9a1","timestamp":"2026-07-24T00:00:02.000Z","message":{"id":"msg_dr01","role":"assistant","content":[{"type":"tool_use","id":"s9t1","name":"Bash","input":{"command":"scripts/delegate-run --role verify --artifact worktree"}}]},$common9}
{"type":"user","uuid":"s9u1","timestamp":"2026-07-24T00:00:12.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s9t1","content":"verdict: needs_attention\nround:   3 (consecutive dispatch 3 of role 'verify' on 'main' without an approve)"}]},$common9}
{"type":"assistant","uuid":"s9a2","timestamp":"2026-07-24T00:00:20.000Z","message":{"id":"msg_dr02","role":"assistant","content":[{"type":"tool_use","id":"s9t2","name":"Bash","input":{"command":"timeout 900 scripts/delegate-run --role verify --artifact worktree"}}]},$common9}
{"type":"user","uuid":"s9u2","timestamp":"2026-07-24T00:00:40.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s9t2","content":"verdict: needs_attention\nround:   22 (consecutive dispatch 22 of role 'verify' on 'main' without an approve)"}]},$common9}
{"type":"assistant","uuid":"s9a3","timestamp":"2026-07-24T00:01:00.000Z","message":{"id":"msg_dr03","role":"assistant","content":[{"type":"tool_use","id":"s9t3","name":"Bash","input":{"command":"timeout 1800 scripts/delegate-run --role code_review --artifact worktree"}}]},$common9}
{"type":"user","uuid":"s9u3","timestamp":"2026-07-24T00:11:00.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s9t3","content":"Command timed out: the command did not complete within its 600s timeout and was moved to the background (ID: b1)."}]},$common9}
{"type":"assistant","uuid":"s9a4","timestamp":"2026-07-24T00:11:10.000Z","message":{"id":"msg_dr04","role":"assistant","content":[{"type":"tool_use","id":"s9t4","name":"Read","input":{"file_path":"/fixture/delegate-run"}}]},$common9}
{"type":"user","uuid":"s9u4","timestamp":"2026-07-24T00:11:12.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s9t4","content":"echo \"round:   99 (consecutive dispatch 99 of role 'verify' on 'main' without an approve)\""}]},$common9}
{"type":"assistant","uuid":"s9a5","timestamp":"2026-07-24T00:20:00.000Z","message":{"id":"msg_dr05","role":"assistant","content":[{"type":"tool_use","id":"s9t5","name":"Bash","input":{"command":"scripts/delegate-run --role council_member --artifact worktree"}}]},$common9}
{"type":"user","uuid":"s9u5","timestamp":"2026-07-24T02:30:00.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"s9t5","content":"verdict: approve"}]},$common9}
EOF

out_dr="$(rules_run --project /fixture/delegate --json)"

dr_max="$(printf '%s' "$out_dr" | jq -r '.rules.verify_rounds.max_round' 2>/dev/null)"
if [ "$dr_max" = "22" ]; then
  ok
else
  bad "max round: want 22 (two digits, and the Read of the launcher's source does not count), got '$dr_max'" "$out_dr"
fi

dr_dist="$(printf '%s' "$out_dr" | jq -r '.rules.verify_rounds.distribution | to_entries | sort_by(.key | tonumber) | map("\(.key)x\(.value)") | join(",")' 2>/dev/null)"
if [ "$dr_dist" = '3x1,22x1' ]; then
  ok
else
  bad "round distribution: want one round 3 and one round 22, got '$dr_dist'" "$out_dr"
fi

dr_key="$(printf '%s' "$out_dr" | jq -c '.rules.verify_rounds.by_key[0]' 2>/dev/null)"
if [ "$dr_key" = '{"role":"verify","branch":"main","dispatches":2,"max_round":22}' ]; then
  ok
else
  bad "rounds by key: want verify on main, 2 dispatches, max 22, got '$dr_key'" "$out_dr"
fi

dr_calls="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.invocations' 2>/dev/null)"
if [ "$dr_calls" = "4" ]; then
  ok
else
  bad "delegate-run invocations: want 4 (the Read is not one), got '$dr_calls'" "$out_dr"
fi

# The fourth call sat 2h10m between dispatch and result, which is an approval
# wait, not a run. Same GAP_CAP_SECONDS exclusion the tool wall clock uses, and
# the excluded count is reported so it is dropped in the open.
dr_excluded="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.excluded_long_gaps' 2>/dev/null)"
dr_measured="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.measured' 2>/dev/null)"
if [ "$dr_excluded" = "1" ] && [ "$dr_measured" = "3" ]; then
  ok
else
  bad "delegate-run long-gap exclusion: want 1 excluded and 3 measured, got '$dr_excluded' and '$dr_measured'" "$out_dr"
fi

dr_median="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.median_s' 2>/dev/null)"
if [ "$dr_median" = "20" ]; then
  ok
else
  bad "delegate-run median: want 20s (10, 20, 600 paired by tool id), got '$dr_median'" "$out_dr"
fi

dr_total="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.total_h' 2>/dev/null)"
if [ "$dr_total" = "0.18" ]; then
  ok
else
  bad "delegate-run total: want 0.18h (630s), got '$dr_total'" "$out_dr"
fi

dr_bg="$(printf '%s' "$out_dr" | jq -r '.rules.delegate_run.backgrounded' 2>/dev/null)"
if [ "$dr_bg" = "1" ]; then
  ok
else
  bad "backgrounded delegate-run calls: want 1, got '$dr_bg'" "$out_dr"
fi

# --- --rules reaches below the top level too ------------------------------
#
# The gate and round metrics are where the missing files hurt most: subagents
# are what run delegate-run, so a top-level-only finder undercounted both.
out_deep_rules="$(rules_run --project "$DEEP_PROJECT" --include-all --json)"
deep_gate="$(printf '%s' "$out_deep_rules" | jq -r '.rules.gate_fires.total' 2>/dev/null)"
if [ "$deep_gate" = "1" ]; then
  ok
else
  bad "--rules gate fires from a subagent transcript: want 1, got '$deep_gate'" "$out_deep_rules"
fi

deep_round="$(printf '%s' "$out_deep_rules" | jq -r '.rules.verify_rounds.max_round' 2>/dev/null)"
if [ "$deep_round" = "4" ]; then
  ok
else
  bad "--rules verify rounds from a subagent transcript: want max round 4, got '$deep_round'" "$out_deep_rules"
fi

# --- the default report is untouched by any of this -----------------------
out_after="$(SESSION_METRICS_ROOT="$TMP/root" "$SCRIPT" --project /fixture/batch --json 2>&1)"
if [ "$out_after" = "$out_batch" ]; then
  ok
else
  bad "default report changed once --rules existed" "$out_after"
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
