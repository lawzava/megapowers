#!/usr/bin/env bash

validate_run_id() {
  local id="$1" caller="$2"
  [[ "$id" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] && return 0
  echo "$caller: run-id must be lowercase-kebab, e.g. release-check (a-z, 0-9, single hyphens)" >&2
  return 2
}
