#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPLAY="$ROOT/evals/studies/pr-replay/replay.go"
CAPTURE_LIMIT=10485760
OVERSIZE_BYTES=11534336
TRUNCATION_NOTICE=$'\n[megapowers: oracle output truncated at the 10485760-byte capture limit]\n'

tmp="${TMPDIR:-/tmp}"
tmp="$(mktemp -d "$tmp/megapowers-subprocess-bounds-XXXXXX")"

recorded_pids=()
cleanup() {
  local pid
  for pid in ${recorded_pids[@]+"${recorded_pids[@]}"}; do
    kill -9 "$pid" 2>/dev/null || true
  done
  rm -rf "$tmp"
}
trap cleanup EXIT

fail() { printf 'FAIL %s\n' "$1" >&2; exit 1; }
ok() { printf 'ok   %s\n' "$1"; }

wait_dead() {
  local pid="$1" deadline=$(( SECONDS + 5 ))
  while kill -0 "$pid" 2>/dev/null; do
    if (( SECONDS >= deadline )); then
      return 1
    fi
    sleep 0.2
  done
}

# The probe links the replay oracle runner with a scratch main entry point and
# drives runOracle directly, so the hostile cases exercise the real code path.
run_probe() {
  local tag="$1" cmd_json="$2" dir="$3" timeout_ms="$4" wall="${5:-60}"
  set +e
  timeout "$wall" env \
    MEGAPOWERS_BOUNDS_PROBE_CMD="$cmd_json" \
    MEGAPOWERS_BOUNDS_PROBE_DIR="$dir" \
    MEGAPOWERS_BOUNDS_PROBE_TIMEOUT_MS="$timeout_ms" \
    MEGAPOWERS_BOUNDS_PROBE_OUT="$tmp/$tag.out" \
    "$tmp/replay-probe" 2>"$tmp/$tag.err"
  PROBE_RC=$?
  set -e
}

mkdir -p "$tmp/work"
cat > "$tmp/probe.go" <<'EOF'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func init() {
	argvJSON := os.Getenv("MEGAPOWERS_BOUNDS_PROBE_CMD")
	if argvJSON == "" {
		return
	}
	var argv []string
	if err := json.Unmarshal([]byte(argvJSON), &argv); err != nil {
		fmt.Fprintln(os.Stderr, "probe argv:", err)
		os.Exit(2)
	}
	ctx := context.Background()
	if value := os.Getenv("MEGAPOWERS_BOUNDS_PROBE_TIMEOUT_MS"); value != "" {
		var timeoutMS int64
		if _, err := fmt.Sscanf(value, "%d", &timeoutMS); err != nil || timeoutMS < 0 {
			fmt.Fprintln(os.Stderr, "probe timeout:", value)
			os.Exit(2)
		}
		if timeoutMS > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
			defer cancel()
		}
	}
	rc, output, err := runOracle(ctx, os.Getenv("MEGAPOWERS_BOUNDS_PROBE_DIR"), argv)
	if out := os.Getenv("MEGAPOWERS_BOUNDS_PROBE_OUT"); out != "" {
		if writeErr := os.WriteFile(out, output, 0o600); writeErr != nil {
			fmt.Fprintln(os.Stderr, "probe write:", writeErr)
			os.Exit(2)
		}
	}
	fmt.Fprintf(os.Stderr, "probe rc=%d len=%d err=%v\n", rc, len(output), err)
	if err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
EOF

cp "$REPLAY" "$tmp/replay.go"
go build -o "$tmp/replay-probe" "$tmp/replay.go" "$tmp/probe.go"

case_a() {
  local dir="$tmp/case-a" pidfile="$tmp/case-a/pids"
  local child_pid="" grandchild_pid=""
  mkdir -p "$dir"
  cat > "$dir/oracle.sh" <<EOF
#!/usr/bin/env bash
sleep 300 &
printf '%s %s\n' "\$\$" "\$!" > '$pidfile'
exec sleep 300
EOF
  run_probe a '["bash","oracle.sh"]' "$dir" 1500 30
  if [[ -s "$pidfile" ]]; then
    read -r child_pid grandchild_pid <"$pidfile" || true
    if [[ "${child_pid:-}" =~ ^[0-9]+$ ]]; then
      recorded_pids+=("$child_pid")
    fi
    if [[ "${grandchild_pid:-}" =~ ^[0-9]+$ ]]; then
      recorded_pids+=("$grandchild_pid")
    fi
  fi
  if (( PROBE_RC == 124 )); then
    fail "timed-out oracle run hung instead of killing its process group"
  fi
  if (( PROBE_RC != 0 )); then
    fail "timed-out oracle probe exited $PROBE_RC: $(cat "$tmp/a.err")"
  fi
  grep -q 'rc=-1' "$tmp/a.err" || fail "timed-out oracle kill was not reported as a signal exit: $(cat "$tmp/a.err")"
  if [[ ! -s "$pidfile" ]]; then
    fail "timed-out oracle never recorded its process group"
  fi
  wait_dead "$child_pid" || fail "timed-out oracle child $child_pid survived the timeout"
  wait_dead "$grandchild_pid" || fail "timed-out oracle grandchild $grandchild_pid survived the timeout"
  ok "timed-out oracle process group dies with its descendants"
}

case_b() {
  local dir="$tmp/case-b"
  mkdir -p "$dir"
  cat > "$dir/oracle.sh" <<EOF
#!/usr/bin/env bash
head -c $OVERSIZE_BYTES /dev/zero | tr '\0' 'A'
EOF
  run_probe b '["bash","oracle.sh"]' "$dir" 0 120
  if (( PROBE_RC != 0 )); then
    fail "oversized-output oracle probe exited $PROBE_RC: $(cat "$tmp/b.err")"
  fi
  grep -q 'rc=0 ' "$tmp/b.err" || fail "oversized-output oracle did not report success to its caller: $(cat "$tmp/b.err")"
  { head -c "$CAPTURE_LIMIT" /dev/zero | tr '\0' 'A'; printf '%s' "$TRUNCATION_NOTICE"; } >"$tmp/b.expected"
  cmp -s "$tmp/b.expected" "$tmp/b.out" || fail "oversized oracle capture is not the capped prefix plus a marked truncation notice"
  ok "oversized oracle output is capped with a marked truncation notice"
}

case_c() {
  local dir="$tmp/case-c"
  mkdir -p "$dir"
  cat > "$dir/oracle.sh" <<'EOF'
#!/usr/bin/env bash
printf 'bounds-oracle-ok'
EOF
  run_probe c '["bash","oracle.sh"]' "$dir" 0 60
  if (( PROBE_RC != 0 )); then
    fail "small-output oracle probe exited $PROBE_RC: $(cat "$tmp/c.err")"
  fi
  printf 'bounds-oracle-ok' >"$tmp/c.expected"
  cmp -s "$tmp/c.expected" "$tmp/c.out" || fail "small oracle output was not captured intact"
  ok "small oracle output is captured intact"
}

selected=",$(printf '%s' "${SUBPROCESS_BOUNDS_ONLY:-a,b,c}"),"
if [[ "$selected" == *",a,"* ]]; then case_a; fi
if [[ "$selected" == *",b,"* ]]; then case_b; fi
if [[ "$selected" == *",c,"* ]]; then case_c; fi

echo "subprocess bounds: PASS"
