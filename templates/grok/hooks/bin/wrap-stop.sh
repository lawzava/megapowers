#!/usr/bin/env bash
# Grok Stop adapter. Usage: wrap-stop.sh <plugin-relative-path>
# Translates camelCase input. Synthesizes a Claude-shaped transcript from
# lastAssistantMessage when the payload has no transcript_path.
set -u
command -v jq >/dev/null 2>&1 || exit 0
rel="${1:-}"
[ -n "$rel" ] || exit 0
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$here/lib.sh"
target="$(plugin_hook "$rel")" || exit 0
input="$(cat 2>/dev/null || true)"
[ -n "$input" ] || exit 0
normalized="$(printf '%s' "$input" | normalize_hook_input 2>/dev/null)" || exit 0

# ONLY CLAUDE SENDS A `reason`. Grok's documented Stop payload carries
# stopHookActive and lastAssistantMessage and nothing else, so an absent or
# empty reason IS an ordinary end of turn. The first shipped version required
# the literal "end_turn" and the normalizer fabricates "", which left both
# Stop hooks dead on every live Grok turn for three days before the 2026-08
# audit caught it. An explicit non-end_turn reason (an interrupt) still skips.
reason="$(printf '%s' "$normalized" | jq -r '.reason // ""' 2>/dev/null)" || reason=""
case "$reason" in ""|end_turn) : ;; *) exit 0 ;; esac

scratch=""
cleanup() { [ -n "${scratch:-}" ] && rm -rf "$scratch"; }
trap cleanup EXIT

transcript="$(printf '%s' "$normalized" | jq -r '.transcript_path // empty' 2>/dev/null)" || transcript=""
if [ -z "$transcript" ] || [ ! -r "$transcript" ]; then
  msg="$(printf '%s' "$normalized" | jq -r '.last_assistant_message // empty' 2>/dev/null)" || msg=""
  if [ -n "$msg" ]; then
    scratch="$(mktemp -d "${TMPDIR:-/tmp}/grok-hook-XXXXXX")" || exit 0
    transcript="$scratch/transcript.jsonl"
    jq -nc --arg t "$msg" \
      '{type:"assistant",message:{content:[{type:"text",text:$t}]}}' \
      >"$transcript" || exit 0
    normalized="$(printf '%s' "$normalized" | jq -c --arg p "$transcript" '.transcript_path = $p')" || exit 0
  fi
fi

printf '%s' "$normalized" | "$target" 2>/dev/null || true
exit 0
