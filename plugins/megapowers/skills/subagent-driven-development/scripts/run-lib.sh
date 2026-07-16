#!/usr/bin/env bash

mp_die() { printf '%s: %s\n' "${MP_PROGRAM:-sdd-run}" "$1" >&2; return "${2:-2}"; }
mp_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
mp_zero_oid() {
  case "$(git rev-parse --show-object-format)" in
    sha1) printf '%040d\n' 0 ;;
    sha256) printf '%064d\n' 0 ;;
    *) mp_die "unsupported Git object format" 2 ;;
  esac
}

mp_validate_run_id() {
  if ! [[ "$1" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
    mp_die "run-id must be lowercase kebab case" 2
    return
  fi
}

mp_validate_node_path() {
  if ! [[ "$1" =~ ^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$ ]]; then
    mp_die "node path must contain lowercase kebab segments" 2
    return
  fi
  local segment
  while IFS= read -r segment; do
    case "$segment" in
      head|brief|claim|result) mp_die "node path uses reserved segment: $segment" 2 || return ;;
    esac
  done < <(printf '%s\n' "$1" | tr '/' '\n')
}

mp_validate_session_id() {
  if ! [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._:@-]{0,127}$ ]]; then
    mp_die "session id contains unsupported characters" 2
    return
  fi
}

mp_run_ref() {
  local run=$1 suffix=$2 ref
  mp_validate_run_id "$run" || return
  ref="refs/megapowers/runs/$run/$suffix"
  git check-ref-format "$ref" >/dev/null 2>&1 ||
    mp_die "invalid run ref: $ref" 2 || return
  printf '%s\n' "$ref"
}

mp_read_ref() { git rev-parse --verify "$1" 2>/dev/null; }
mp_read_json_ref() { git cat-file blob "$1" 2>/dev/null | jq -ce .; }
mp_hash_json_file() { jq -ce . "$1" | git hash-object -w --stdin; }

mp_create_ref() {
  local ref=$1 oid=$2
  git update-ref "$ref" "$oid" "$(mp_zero_oid)"
}

mp_cas_ref() {
  local ref=$1 oid=$2 expected=$3
  git update-ref "$ref" "$oid" "$expected"
}

mp_delete_ref() {
  local ref=$1 expected=$2
  git update-ref -d "$ref" "$expected"
}

mp_manifest_json() {
  mp_read_json_ref "$(mp_run_ref "$1" manifest)"
}
