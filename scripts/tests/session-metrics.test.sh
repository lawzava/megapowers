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

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
