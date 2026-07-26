#!/usr/bin/env bash
# Shared helpers for the run-* scripts. Sourced, never executed.

validate_run_id() {
  local id="$1" caller="$2"
  [[ "$id" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] && return 0
  echo "$caller: run-id must be lowercase-kebab, e.g. release-check (a-z, 0-9, single hyphens)" >&2
  return 2
}

# plan_digest <plan.md> — emit "<tag> <sha256>" per milestone: the hash covers the
# milestone heading plus its acceptance line(s), whitespace-normalized.
#
# run-init freezes this fingerprint; run-derive-status and run-verify-status compare
# the live plan against it. The point is DRIFT DETECTION, not tamper-proofing: it
# catches a milestone that was quietly deleted or an acceptance check that was
# reworded after the fact, so a done-claim always names the criteria it was actually
# judged against. It is not a security control. The digest lives in the same
# agent-writable run directory it describes, and `run-init <id> --replan` re-freezes
# it on request, so anything that can edit the plan can also re-baseline it. That is
# the intended escape hatch for a deliberate re-plan; the value is that the re-plan
# becomes explicit instead of silent.
#
# Lives here so all three callers hash identically. A divergent copy would make
# every comparison fail.
plan_digest() {
  awk '
    function norm(s) { gsub(/[ \t]+/, " ", s); sub(/^ +/, "", s); sub(/ +$/, "", s); return s }
    /^## [A-Za-z][A-Za-z0-9_-]*:/ {
      if (tag != "") print tag "\t" buf
      tag = substr($0, 4, index($0, ":") - 4); buf = norm($0); next
    }
    /^## / { if (tag != "") print tag "\t" buf; tag = ""; buf = ""; next }
    tag != "" && $0 ~ /^[ \t]*-?[ \t]*acceptance:/ { buf = buf " ][ " norm($0) }
    END { if (tag != "") print tag "\t" buf }
  ' "$1" | while IFS="$(printf '\t')" read -r t c; do
    printf '%s %s\n' "$t" "$(printf '%s' "$c" | sha256sum | cut -d' ' -f1)"
  done
}
