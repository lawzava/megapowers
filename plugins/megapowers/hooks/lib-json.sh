#!/usr/bin/env bash
# lib-json.sh — shared helper sourced by this plugin's hook scripts.

# Escape a string for JSON embedding using bash parameter substitution.
# Each ${s//old/new} is a single C-level pass, orders of magnitude faster
# than a character-by-character loop.
escape_for_json() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\t'/\\t}"
    printf '%s' "$s"
}
