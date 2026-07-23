#!/usr/bin/env bash
# Shared Stop-hook context gate. Return 0 when workflow hooks must no-op.

stop_context_is_exempt() {
  local input="$1" role="${MEGAPOWERS_ROLE:-}" preset="${MEGAPOWERS_PRESET:-}"
  [ "${MEGAPOWERS_EXACT_OUTPUT:-0}" = "1" ] && return 0
  [ "$preset" = "read_only" ] && return 0
  case "$role" in
    reviewer|code_review|plan_review|verify|judge|council_member|visual_verify|plan|read_only)
      return 0
      ;;
  esac
  command -v jq >/dev/null 2>&1 || return 1
  [ "$(printf '%s' "$input" | jq -r '.permission_mode // empty' 2>/dev/null)" = "plan" ] && return 0
  case "$(printf '%s' "$input" | jq -r '.agent_type // empty' 2>/dev/null)" in
    reviewer|code_review|plan_review|verify|judge|council_member|visual_verify|plan|read_only)
      return 0
      ;;
  esac
  return 1
}
