#!/usr/bin/env bash
# security-lint.sh — flag the documented malicious-skill markers in this repo's
# executable-instruction surface: skill bodies, hook scripts, and templates.
#
# It is a lightweight, deterministic grep lint, not a sandbox or a proof of
# safety. It scans for the patterns a compromised or careless skill/hook would
# carry: a fetch of remote content in an executable context, a base64 blob
# decoded straight into a shell, `eval` of fetched content, unicode
# direction-override characters (the Trojan-Source trick), and instructions
# that tell the agent to turn its own safety off. It exists so this marketplace
# can enforce, in its own tree, the scan it would want applied to any
# third-party skill before install.
#
#   scripts/security-lint.sh                 scan the repo's default scope
#   scripts/security-lint.sh PATH [PATH...]  scan the given files/dirs instead
#
# Exit 0 clean, 1 on any hit (each printed as file:line: reason), 2 on a usage
# or environment error. Files that legitimately contain a pattern (test
# fixtures, an opt-in notifier) are listed in scripts/security-lint.allowlist.
# Wiring into validate.sh / CI is owned separately; this script stands alone.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST_FILE="$ROOT/scripts/security-lint.allowlist"

# --- allowlist -------------------------------------------------------------
declare -A ALLOW=()
if [ -r "$ALLOWLIST_FILE" ]; then
  while IFS= read -r line; do
    line="${line%%#*}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [ -n "$line" ] && ALLOW["$line"]=1
  done < "$ALLOWLIST_FILE"
fi

# A shipped, agent-facing skill (plugins/*/skills/*/SKILL.md) may NEVER be
# allowlisted: silencing the lint on an installed skill would let a malicious marker
# ship in exactly the surface this lint exists to protect. Fix the skill, not the
# allowlist. Refuse such an entry outright (exit nonzero, naming it); legitimate
# entries (test fixtures, the opt-in notifier template) are unaffected.
if [ "${#ALLOW[@]}" -gt 0 ]; then
  for entry in "${!ALLOW[@]}"; do
    case "$entry" in
      plugins/*/skills/*/SKILL.md)
        printf 'security-lint: disallowed allowlist entry: %s (a shipped skill may never be allowlisted; fix the skill, not the allowlist).\n' "$entry" >&2
        exit 1 ;;
    esac
  done
fi

# --- file discovery --------------------------------------------------------
list_default_scope() {
  find "$ROOT/plugins" -name 'SKILL.md' -type f -print0 2>/dev/null || return 2
  find "$ROOT/plugins" -path '*/hooks/*' -type f -print0 2>/dev/null || return 2
  find "$ROOT/templates" -type f -print0 2>/dev/null || return 2
}

list_args_scope() {
  local p
  for p in "$@"; do
    if [ -d "$p" ]; then
      find "$p" -type f -print0 2>/dev/null || return 2
    elif [ -f "$p" ]; then
      printf '%s\0' "$p"
    else
      printf 'security-lint: input is missing or unsupported: %s\n' "$p" >&2
      return 2
    fi
  done
}

path_sorts_after() {
  local LC_ALL=C
  [[ "$1" > "$2" ]]
}

# repo-relative path for allowlist lookup and reporting
relpath() {
  local p="$1"
  case "$p" in
    "$ROOT"/*) printf '%s' "${p#"$ROOT"/}" ;;
    *) printf '%s' "$p" ;;
  esac
}

# Join backslash-continued lines into one logical record, tab-prefixed with the
# physical line where the record starts, so a multi-line `curl ... \<newline>
# "https://..."` is scanned (and reported) as a single fetch.
logical_lines() {
  awk '
    { line = $0
      if (start == 0) start = FNR
      if (line ~ /\\$/) { sub(/\\$/, "", line); buf = buf line; next }
      buf = buf line
      printf "%d\t%s\n", start, buf
      buf = ""; start = 0
    }
    END { if (buf != "") printf "%d\t%s\n", start, buf }
  ' "$1"
}

HITS=0
emit() { printf '%s:%s: %s\n' "$1" "$2" "$3"; HITS=$((HITS + 1)); }

check_text_rules() {
  # One awk process checks every text rule for the file. The old implementation
  # forked grep once or twice per logical line, making clean files the slow path.
  local rel="$1" stream="$2" ln msg findings
  if ! findings="$(printf '%s\n' "$stream" | awk '
    {
      line=$0
      sub(/^[0-9]+\t/, "", line)
      start=$0
      sub(/\t.*/, "", start)
      lower=tolower(line)

      if (lower ~ /(^|[^[:alnum:]_])(curl|wget|fetch)([^[:alnum:]_]|$)/ &&
          lower ~ /https?:\/\//)
        fetch_hits=fetch_hits start "\tfetch of remote content in executable context\n"
      if (lower ~ /base64([[:space:]]+[^|]*)?(-d|-di|--decode)[^|]*\|[[:space:]]*(env[[:space:]]+)?([^[:space:]]*\/)?(sh|bash|zsh|dash|ksh|python[0-9.]*|node|perl|ruby)([[:space:]]|$)/)
        base64_hits=base64_hits start "\tbase64-decoded blob piped into a shell\n"
      if (lower ~ /eval[^#]*(\$\(|`)[^)]*(curl|wget|fetch)/)
        eval_hits=eval_hits start "\teval of fetched remote content\n"
      if (lower ~ /ignore (all |the )?(previous|prior) (instruction|message|context)|disregard (all |the )?(previous|prior|the above)|disable (the )?(sandbox|safety|guardrail|security)|bypass (the )?permission|bypass permissions|turn off (the )?(sandbox|safety)/)
        safety_hits=safety_hits start "\tinstruction to disable a safety mechanism\n"
    }
    END { printf "%s%s%s%s", fetch_hits, base64_hits, eval_hits, safety_hits }
  ')"; then
    printf 'security-lint: text-rule scan failed: %s\n' "$rel" >&2
    return 2
  fi
  [ -n "$findings" ] || return 0
  while IFS=$'\t' read -r ln msg; do
    emit "$rel" "$ln" "$msg"
  done <<< "$findings"
}

check_unicode() {
  # bidi / direction-override code points (Trojan Source)
  local rel="$1" stream="$2" ln rest
  while IFS=$'\t' read -r ln rest; do
    case "$rest" in
      *$'\xE2\x80\xAA'*|*$'\xE2\x80\xAB'*|*$'\xE2\x80\xAC'*|*$'\xE2\x80\xAD'*|*$'\xE2\x80\xAE'*|\
      *$'\xE2\x81\xA6'*|*$'\xE2\x81\xA7'*|*$'\xE2\x81\xA8'*|*$'\xE2\x81\xA9'*|\
      *$'\xE2\x80\x8E'*|*$'\xE2\x80\x8F'*|*$'\xD8\x9C'*)
        emit "$rel" "$ln" "unicode direction-override / bidi control character" ;;
    esac
  done <<< "$stream"
}

# --- scan ------------------------------------------------------------------
if ! discovery_file="$(mktemp)"; then
  printf 'security-lint: could not create discovery file\n' >&2
  exit 2
fi
trap 'rm -f -- "$discovery_file"' EXIT

if [ "$#" -gt 0 ]; then
  if ! list_args_scope "$@" > "$discovery_file"; then
    printf 'security-lint: input discovery failed\n' >&2
    exit 2
  fi
else
  if ! list_default_scope > "$discovery_file"; then
    printf 'security-lint: default-scope discovery failed\n' >&2
    exit 2
  fi
fi

declare -A SEEN_FILES=()
files=()
while IFS= read -r -d '' f; do
  [ -z "${SEEN_FILES[$f]+set}" ] || continue
  SEEN_FILES["$f"]=1
  files+=("$f")
done < "$discovery_file"

# Preserve the prior bytewise lexical reporting order without flattening paths
# through a newline-delimited external sort.
for ((i = 1; i < ${#files[@]}; i++)); do
  key="${files[i]}"
  j=$((i - 1))
  while ((j >= 0)) && path_sorts_after "${files[j]}" "$key"; do
    files[j + 1]="${files[j]}"
    j=$((j - 1))
  done
  files[j + 1]="$key"
done

for f in "${files[@]}"; do
  [ -f "$f" ] || continue
  rel="$(relpath "$f")"
  [ -n "${ALLOW[$rel]:-}" ] && continue
  if [ ! -r "$f" ]; then
    printf 'security-lint: unreadable input: %s\n' "$rel" >&2
    exit 2
  fi
  if grep -Iq . "$f" 2>/dev/null; then
    :
  else
    probe_rc=$?
    [ "$probe_rc" -eq 1 ] && continue       # skip binary or empty files
    printf 'security-lint: input probe failed: %s\n' "$rel" >&2
    exit 2
  fi
  if ! stream="$(logical_lines "$f")"; then
    printf 'security-lint: logical-line scan failed: %s\n' "$rel" >&2
    exit 2
  fi
  check_text_rules "$rel" "$stream" || exit 2
  check_unicode "$rel" "$stream"
done

if [ "$HITS" -gt 0 ]; then
  printf 'security-lint: %d finding(s)\n' "$HITS" >&2
  exit 1
fi
printf 'security-lint: clean\n' >&2
exit 0
