#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE="$HERE/../run.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/repo/evals/scenarios"
cp "$SOURCE" "$TMP/repo/evals/run.sh"
chmod +x "$TMP/repo/evals/run.sh"

pass=0
fail=0
check_result() {
  local id="$1" verdict="$2" status="$3" phase="$4" stage_rc="$5" command_rc out
  set +e
  out="$("$TMP/repo/evals/run.sh" "$id" 2>/dev/null)"
  command_rc=$?
  set -e
  if [ "$command_rc" -ne 0 ] &&
     printf '%s' "$out" | jq -e --arg verdict "$verdict" --arg status "$status" --arg phase "$phase" --argjson stage_rc "$stage_rc" \
       '.schema_version == "1" and .evidence_class == "regression" and .arm == "regression" and
        .verdict == $verdict and .status == $status and .phase == $phase and .rc == $stage_rc' >/dev/null 2>&1; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    printf '  FAIL %s: command_rc=%s output=%s\n' "$id" "$command_rc" "$out"
  fi
}
make_scenario() {
  local id="$1"
  mkdir -p "$TMP/repo/evals/scenarios/$id"
  cat > "$TMP/repo/evals/scenarios/$id/scenario.toml" <<EOF
id = "$id"
title = "$id"
skill = "test"
kind = "artifact"
EOF
  cat > "$TMP/repo/evals/scenarios/$id/check.sh" <<'EOF'
#!/usr/bin/env bash
touch check-ran
exit "${CHECK_RC:-0}"
EOF
  chmod +x "$TMP/repo/evals/scenarios/$id/check.sh"
}

echo "== eval run fail-closed tests =="
make_scenario setup-fails
cat > "$TMP/repo/evals/scenarios/setup-fails/setup.sh" <<'EOF'
#!/usr/bin/env bash
exit 23
EOF
cat > "$TMP/repo/evals/scenarios/setup-fails/solve.sh" <<'EOF'
#!/usr/bin/env bash
touch actor-ran
EOF
check_result setup-fails harness_error harness_error setup 23

make_scenario actor-fails
cat > "$TMP/repo/evals/scenarios/actor-fails/solve.sh" <<'EOF'
#!/usr/bin/env bash
exit 24
EOF
check_result actor-fails harness_error harness_error actor 24

make_scenario oracle-indeterminate
cat > "$TMP/repo/evals/scenarios/oracle-indeterminate/solve.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$TMP/repo/evals/scenarios/oracle-indeterminate/check.sh" <<'EOF'
#!/usr/bin/env bash
exit 77
EOF
check_result oracle-indeterminate indeterminate completed oracle 77

make_scenario actor-times-out
cat > "$TMP/repo/evals/scenarios/actor-times-out/solve.sh" <<'EOF'
#!/usr/bin/env bash
sleep 2
EOF
set +e
out="$("$TMP/repo/evals/run.sh" actor-times-out --timeout 1 2>/dev/null)"
command_rc=$?
set -e
if [ "$command_rc" -ne 0 ] &&
   printf '%s' "$out" | jq -e '.status == "timeout" and .verdict == "harness_error" and .rc == 124' >/dev/null; then
  pass=$((pass + 1))
else
  fail=$((fail + 1))
  printf '  FAIL actor-times-out: command_rc=%s output=%s\n' "$command_rc" "$out"
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
