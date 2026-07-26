#!/usr/bin/env bash
# Shared restricted-TOML readers. Sourced, never executed.
#
# BYTE-TWIN. Identical copies ship at:
#   plugins/megapowers/hooks/lib-toml.sh
#   plugins/mega-orchestration/skills/multi-agent-delegation/scripts/lib-toml.sh
# Plugins cannot locate each other at runtime, so the file is duplicated rather
# than shared; scripts/validate.sh fails on any drift between the copies. Edit
# one, copy it to the other.
#
# Grammar (the subset models.toml and delegates.toml actually use): [section] and
# [section.name] headers with internal whitespace tolerated, one `key = value` per
# line, values bare or single/double quoted, `#` comments outside quotes, and
# single-line arrays of quoted strings. Multi-line strings and inline tables are
# not parsed.
#
# Every function reads exactly ONE file, passed as $1. Layer precedence is the
# caller's business: render-model-catalog and delegate-resolve walk different
# layer stacks, and keeping merge policy out of here is what lets both share the
# leaf readers.

# toml_section_exists_in <file> <section> -> exit 0 when the header is present
toml_section_exists_in() {
  awk -v sec="[$2]" '{h=$0; gsub(/[[:space:]]/,"",h)} h==sec {found=1; exit} END{exit !found}' "$1"
}

# toml_key_exists_in <file> <section> <key> -> exit 0 when the key is defined.
# Separate from the value readers so a caller can tell "defined as empty" from
# "not defined", which is what makes per-key layer merging correct.
toml_key_exists_in() {
  awk -v sec="[$2]" -v key="$3" '
    { hn=$0; gsub(/[[:space:]]/,"",hn) }
    hn==sec {inx=1; next}
    /^[[:space:]]*\[/ {inx=0}
    inx && $0 ~ ("^[[:space:]]*" key "[[:space:]]*=") {found=1; exit}
    END {exit !found}
  ' "$1"
}

# toml_scalar_in <file> <section> <key> -> print the value, unquoted
toml_scalar_in() {
  awk -v sec="[$2]" -v key="$3" '
    { hn=$0; gsub(/[[:space:]]/,"",hn) }
    hn==sec {inx=1; next}
    /^[[:space:]]*\[/ {inx=0}
    inx && $0 ~ ("^[[:space:]]*" key "[[:space:]]*=") {
      rest=$0; sub(/^[^=]*=[[:space:]]*/,"",rest)
      if (rest ~ /^"/)      { sub(/^"/,"",rest); sub(/".*/,"",rest); print rest; exit }
      else if (rest ~ /^'\''/) { sub(/^'\''/,"",rest); sub(/'\''.*/,"",rest); print rest; exit }
      else { sub(/#.*/,"",rest); gsub(/[[:space:]]+$/,"",rest); print rest; exit }
    }
  ' "$1"
}

# toml_array_in <file> <section> <key> -> print one element per line
toml_array_in() {
  awk -v sec="[$2]" -v key="$3" '
    { hn=$0; gsub(/[[:space:]]/,"",hn) }
    hn==sec {inx=1; next}
    /^[[:space:]]*\[/ {inx=0}
    inx && $0 ~ ("^[[:space:]]*" key "[[:space:]]*=") {
      rest=$0; sub(/^[^=]*=/,"",rest)
      while (match(rest, /"[^"]*"/)) {
        print substr(rest, RSTART+1, RLENGTH-2)
        rest=substr(rest, RSTART+RLENGTH)
      }
      exit
    }
  ' "$1"
}

# toml_provider_names_in <file> -> print each [providers.X] name (tier subsections
# excluded, so [providers.codex.tiers] never reads as a provider called
# "codex.tiers")
toml_provider_names_in() {
  awk '{h=$0; gsub(/[[:space:]]/,"",h)} h ~ /^\[providers\.[A-Za-z0-9_-]+\]$/ {n=h; sub(/^\[providers\./,"",n); sub(/\]$/,"",n); print n}' "$1"
}
