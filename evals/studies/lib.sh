#!/usr/bin/env bash
# Shared private temporary-directory helper for shell study runners.

# Create a disposable directory with private permissions. Callers remain
# responsible for installing a trap that removes it on every exit path.
study_private_tmpdir() { # <portable-prefix>
  local prefix="${1:-megapowers-study}" parent old_umask dir
  parent="${TMPDIR:-/tmp}"
  old_umask="$(umask)"
  umask 077
  dir="$(mktemp -d "$parent/$prefix.XXXXXX")" || {
    umask "$old_umask"
    return 2
  }
  umask "$old_umask"
  chmod 700 "$dir" || { rm -rf "$dir"; return 2; }
  printf '%s\n' "$dir"
}
