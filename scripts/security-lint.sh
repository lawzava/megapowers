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

# Real fetch targets a skill may legitimately hit. A fetch to any host NOT on
# this list, in executable context, is flagged. Hosts are matched as a suffix
# (docs.anthropic.com matches, evil-anthropic.com.attacker.tld does not).
DOC_HOSTS='raw.githubusercontent.com github.com docs.github.com docs.anthropic.com anthropic.com platform.openai.com openai.com agentskills.io'

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
  find "$ROOT/plugins" -name 'SKILL.md' -type f 2>/dev/null
  find "$ROOT/plugins" -path '*/hooks/*' -type f 2>/dev/null
  find "$ROOT/templates" -type f 2>/dev/null
}

list_args_scope() {
  local p
  for p in "$@"; do
    if [ -d "$p" ]; then
      find "$p" -type f 2>/dev/null
    elif [ -f "$p" ]; then
      printf '%s\n' "$p"
    fi
  done
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

# host is allowlisted iff it equals or is a dot-suffix of a DOC_HOSTS entry
host_allowed() {
  local host="$1" d
  for d in $DOC_HOSTS; do
    [ "$host" = "$d" ] && return 0
    case "$host" in *".$d") return 0 ;; esac
  done
  return 1
}

check_fetch() {
  # a fetch command (curl/wget/fetch) plus an http(s) URL to a non-doc host
  local rel="$1" stream="$2" ln rest host bad
  while IFS=$'\t' read -r ln rest; do
    printf '%s' "$rest" | grep -qEi '(^|[^[:alnum:]_])(curl|wget|fetch)([^[:alnum:]_]|$)' || continue
    printf '%s' "$rest" | grep -qEi 'https?://' || continue
    bad=""
    while IFS= read -r host; do
      [ -n "$host" ] || continue
      host_allowed "$host" || bad="$host"
    done < <(printf '%s' "$rest" | grep -oEi 'https?://[a-zA-Z0-9.-]+' | sed -E 's#^https?://##I')
    [ -n "$bad" ] && emit "$rel" "$ln" "fetch of remote content in executable context (host: $bad)"
  done < <(printf '%s\n' "$stream")
}

check_regex() {
  # emit a hit for every logical line matching an extended regex
  local rel="$1" stream="$2" pat="$3" msg="$4" ln rest
  while IFS=$'\t' read -r ln rest; do
    emit "$rel" "$ln" "$msg"
  done < <(printf '%s\n' "$stream" | grep -iE -- "$pat" || true)
}

check_unicode() {
  # bidi / direction-override code points (Trojan Source)
  local rel="$1" stream="$2" ln rest
  while IFS=$'\t' read -r ln rest; do
    emit "$rel" "$ln" "unicode direction-override / bidi control character"
  done < <(printf '%s\n' "$stream" \
    | grep -nP '[\x{202a}-\x{202e}\x{2066}-\x{2069}\x{200e}\x{200f}\x{061c}]' 2>/dev/null \
    | sed -E 's/^[0-9]+://' || true)
}

B64_PAT='base64([[:space:]]+[^|]*)?(-d|-di|--decode)[^|]*\|[[:space:]]*(env[[:space:]]+)?([^[:space:]]*/)?(sh|bash|zsh|dash|ksh|python[0-9.]*|node|perl|ruby)([[:space:]]|$)'
EVAL_PAT='eval[^#]*(\$\(|`)[^)]*(curl|wget|fetch)'
INJECT_PAT='ignore (all |the )?(previous|prior) (instruction|message|context)|disregard (all |the )?(previous|prior|the above)|disable (the )?(sandbox|safety|guardrail|security)|bypass (the )?permission|bypass permissions|turn off (the )?(sandbox|safety)'

# --- scan ------------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  files="$(list_args_scope "$@")"
else
  files="$(list_default_scope)"
fi
files="$(printf '%s\n' "$files" | sort -u)"

while IFS= read -r f; do
  [ -n "$f" ] || continue
  [ -f "$f" ] || continue
  grep -Iq . "$f" 2>/dev/null || continue   # skip binary files
  rel="$(relpath "$f")"
  [ -n "${ALLOW[$rel]:-}" ] && continue
  stream="$(logical_lines "$f")"
  check_fetch "$rel" "$stream"
  check_regex "$rel" "$stream" "$B64_PAT" "base64-decoded blob piped into a shell"
  check_regex "$rel" "$stream" "$EVAL_PAT" "eval of fetched remote content"
  check_regex "$rel" "$stream" "$INJECT_PAT" "instruction to disable a safety mechanism"
  check_unicode "$rel" "$stream"
done < <(printf '%s\n' "$files")

if [ "$HITS" -gt 0 ]; then
  printf 'security-lint: %d finding(s)\n' "$HITS" >&2
  exit 1
fi
printf 'security-lint: clean\n' >&2
exit 0
