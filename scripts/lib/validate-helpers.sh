#!/usr/bin/env bash
# Shared helpers for scripts/validate.sh. The caller provides ok() and bad().
# Parallel-lane tests must keep all mutable state under a private temporary
# directory, avoid ambient credentials and fixed ports, and clean up children.
# Enrollment is explicit: every unlisted or newly added test runs quiet.

validate_jobs_error_reported=0
validate_parallel_tests_file="${VALIDATE_PARALLEL_TESTS_FILE:-$ROOT/scripts/validate-parallel-tests.txt}"
validate_parallel_audit_file="${VALIDATE_PARALLEL_AUDIT_FILE:-$ROOT/scripts/validate-parallel-tests.audit.tsv}"
validate_parallel_tests=()
validate_parallel_audited_tests=()
validate_parallel_audit_notes=()

validate_skill_body_bytes() {
  byte_len "$(cat "$1")"
}

if [ -r "$validate_parallel_tests_file" ]; then
  while IFS= read -r declared_test; do
    case "$declared_test" in ''|'#'*) continue ;; esac
    validate_parallel_tests[${#validate_parallel_tests[@]}]="$declared_test"
  done < "$validate_parallel_tests_file"
else
  bad "parallel-safe test manifest missing: $validate_parallel_tests_file"
fi
if [ -r "$validate_parallel_audit_file" ]; then
  while IFS=$'\t' read -r audited_test audit_note; do
    case "$audited_test" in ''|'#'*) continue ;; esac
    validate_parallel_audited_tests[${#validate_parallel_audited_tests[@]}]="$audited_test"
    validate_parallel_audit_notes[${#validate_parallel_audit_notes[@]}]="$audit_note"
  done < "$validate_parallel_audit_file"
else
  bad "parallel-safe isolation audit missing: $validate_parallel_audit_file"
fi

validate_test_is_parallel_safe() {
  local test_path="$1" declared_test normalized_test normalized_declared
  normalized_test="$test_path"
  case "$normalized_test" in
    "$ROOT"/*) normalized_test="${normalized_test#"$ROOT"/}" ;;
  esac
  for declared_test in "${validate_parallel_tests[@]}"; do
    normalized_declared="$declared_test"
    case "$normalized_declared" in
      "$ROOT"/*) normalized_declared="${normalized_declared#"$ROOT"/}" ;;
    esac
    [ "$normalized_test" != "$normalized_declared" ] || return 0
  done
  return 1
}

validate_test_requires_quiet() {
  ! validate_test_is_parallel_safe "$1"
}

validate_test_has_parallel_audit() {
  local test_path="$1" audit_index
  audit_index=0
  while [ "$audit_index" -lt "${#validate_parallel_audited_tests[@]}" ]; do
    if [ "$test_path" = "${validate_parallel_audited_tests[$audit_index]}" ] &&
       [ -n "${validate_parallel_audit_notes[$audit_index]}" ]
    then
      return 0
    fi
    audit_index=$((audit_index + 1))
  done
  return 1
}

validate_parallel_manifest() {
  local declared_test resolved missing=0
  if [ "${#validate_parallel_tests[@]}" -eq 0 ]; then
    bad "parallel-safe test manifest has no entries: $validate_parallel_tests_file"
    return 0
  fi
  for declared_test in "${validate_parallel_tests[@]}"; do
    case "$declared_test" in
      /*) resolved="$declared_test" ;;
      *) resolved="$ROOT/$declared_test" ;;
    esac
    if [ ! -f "$resolved" ]; then
      bad "parallel-safe test manifest names missing file: $declared_test"
      missing=1
    fi
    if ! validate_test_has_parallel_audit "$declared_test"; then
      bad "parallel-safe test has no isolation audit: $declared_test"
      missing=1
    fi
  done
  [ "$missing" -eq 0 ] && ok "parallel-safe test manifest has ${#validate_parallel_tests[@]} current entries"
  return 0
}

validate_report_test() { # $1=label $2=path $3=output $4=status
  local label="$1" test_path="$2" output="$3" status="$4"
  if [ "$status" -eq 0 ]; then
    ok "$label $test_path"
  else
    bad "$label $test_path"
    sed -n '$!{/FAIL/p};${p}' "$output" | tail -15 | sed 's/^/      /'
  fi
}

validate_restore_trap() { # $1=signal $2=output from trap -p
  if [ -n "$2" ]; then
    eval "$2"
  else
    trap - "$1"
  fi
}

validate_abort_test_group() { # called only while run_test_group locals exist
  local exit_code="$1" child_pid launch_pid="${!:-}"
  trap - INT TERM HUP EXIT
  if [ "${launch_in_progress:-0}" -eq 1 ] &&
     [ -n "$launch_pid" ] &&
     [ "$launch_pid" != "${launch_previous_pid:-}" ]
  then
    kill -TERM "$launch_pid" 2>/dev/null || true
    wait "$launch_pid" 2>/dev/null || true
  fi
  for child_pid in "${pids[@]}"; do
    [ -n "$child_pid" ] || continue
    kill -TERM "$child_pid" 2>/dev/null || true
  done
  for child_pid in "${pids[@]}"; do
    [ -n "$child_pid" ] || continue
    wait "$child_pid" 2>/dev/null || true
  done
  rm -rf "$scratch"
  exit "$exit_code"
}

run_test_group() { # $1=label $2=missing-test diagnostic; paths on stdin
  local label="$1" missing="$2" jobs="${VALIDATE_JOBS:-4}"
  local scratch count index batch_end worker status output test_path
  local saved_int saved_term saved_hup saved_exit
  local launch_in_progress=0 launch_previous_pid=""
  local -a tests pids

  case "$jobs" in
    [1-9]|[1-5][0-9]|6[0-4]) ;;
    *)
      if [ "$validate_jobs_error_reported" -eq 0 ]; then
        bad "VALIDATE_JOBS must be an integer from 1 to 64 (got '$jobs')"
        validate_jobs_error_reported=1
      fi
      jobs=1 ;;
  esac

  tests=()
  while IFS= read -r test_path; do
    [ -n "$test_path" ] || continue
    tests[${#tests[@]}]="$test_path"
  done
  count=${#tests[@]}
  if [ "$count" -eq 0 ]; then
    bad "$missing"
    return 0
  fi

  scratch="$(mktemp -d)" || {
    bad "cannot create temporary directory for $label"
    return 0
  }
  saved_int="$(trap -p INT)"
  saved_term="$(trap -p TERM)"
  saved_hup="$(trap -p HUP)"
  saved_exit="$(trap -p EXIT)"
  trap 'validate_abort_test_group 130' INT
  trap 'validate_abort_test_group 143' TERM HUP
  trap 'rm -rf "$scratch"' EXIT

  index=0
  while [ "$index" -lt "$count" ]; do
    if validate_test_requires_quiet "${tests[$index]}"; then
      output="$scratch/$index.out"
      status=0
      launch_previous_pid="${!:-}"
      launch_in_progress=1
      bash "${tests[$index]}" >"$output" 2>&1 &
      pids[$index]=$!
      launch_in_progress=0
      wait "${pids[$index]}" || status=$?
      unset "pids[$index]"
      validate_report_test "$label" "${tests[$index]}" "$output" "$status"
      index=$((index + 1))
      continue
    fi

    batch_end=$index
    pids=()
    while [ "$batch_end" -lt "$count" ] &&
          [ $((batch_end - index)) -lt "$jobs" ] &&
          ! validate_test_requires_quiet "${tests[$batch_end]}"
    do
      worker=$batch_end
      output="$scratch/$worker.out"
      launch_previous_pid="${!:-}"
      launch_in_progress=1
      (
        worker_status=0
        test_pid=0
        trap 'if [ "$test_pid" -gt 0 ]; then kill -TERM "$test_pid" 2>/dev/null || true; wait "$test_pid" 2>/dev/null || true; fi; exit 143' TERM HUP
        bash "${tests[$worker]}" >"$output" 2>&1 &
        test_pid=$!
        wait "$test_pid" || worker_status=$?
        trap - TERM HUP
        printf '%s\n' "$worker_status" > "$scratch/$worker.status"
      ) &
      pids[$worker]=$!
      launch_in_progress=0
      batch_end=$((batch_end + 1))
    done

    worker=$index
    while [ "$worker" -lt "$batch_end" ]; do
      wait "${pids[$worker]}" || true
      unset "pids[$worker]"
      output="$scratch/$worker.out"
      if [ -r "$scratch/$worker.status" ] &&
         IFS= read -r status < "$scratch/$worker.status" &&
         [[ $status =~ ^[0-9]+$ ]]
      then
        :
      else
        status=125
        printf 'test worker exited without recording a status\n' >> "$output"
      fi
      validate_report_test "$label" "${tests[$worker]}" "$output" "$status"
      worker=$((worker + 1))
    done
    index=$batch_end
  done

  rm -rf "$scratch"
  validate_restore_trap INT "$saved_int"
  validate_restore_trap TERM "$saved_term"
  validate_restore_trap HUP "$saved_hup"
  validate_restore_trap EXIT "$saved_exit"
}

run_shellcheck_group() { # paths on stdin
  local path individual_failures=0 batch_output
  local -a files
  files=()
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    files[${#files[@]}]="$path"
  done
  if [ "${#files[@]}" -eq 0 ]; then
    bad "no shell scripts found for ShellCheck"
    return 0
  fi

  batch_output="$(mktemp)" || {
    bad "cannot create temporary file for ShellCheck output"
    return 0
  }
  if shellcheck -S warning "${files[@]}" >"$batch_output" 2>&1; then
    rm -f "$batch_output"
    for path in "${files[@]}"; do ok "shellcheck $path"; done
    return 0
  fi

  for path in "${files[@]}"; do
    if shellcheck -S warning "$path" >/dev/null 2>&1; then
      ok "shellcheck $path"
    else
      bad "shellcheck $path"
      individual_failures=$((individual_failures + 1))
    fi
  done
  if [ "$individual_failures" -eq 0 ]; then
    bad "shellcheck batch failed but no single file reproduced it"
    tail -15 "$batch_output" | sed 's/^/      /'
  fi
  rm -f "$batch_output"
}
