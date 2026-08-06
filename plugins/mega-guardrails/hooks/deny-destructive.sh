#!/usr/bin/env bash
# PreToolUse(Bash) guard — a SMALL, HIGH-CONFIDENCE accident tripwire.
#
# What it does:
#   - DENY a short list of catastrophic, unrecoverable commands (wipe root/home/system
#     dirs, format a disk, overwrite a block device, fork bomb, chmod 777 /). These are
#     never a legitimate agent action, so denying them costs no real capability.
#   - ASK (surface a confirmation instead of a flat refusal) for reversible-but-risky
#     LOCAL ops: destructive git (reset --hard, clean -f, branch -D, push --force, and
#     a whole-tree checkout/restore of '.', which discards uncommitted work), and a
#     remote download piped into a shell (curl … | bash). These are recoverable
#     (reflog) or a deliberate footgun, so the human decides in the moment. Remote
#     destructive ops (cloud deletes, terraform destroy, kubectl delete --all) are
#     NOT pattern-matched here: they are one-shot real-world effects, which is the
#     effect-broker skill's job, not a command-string grep's.
#   Note: this hook does NOT try to catch secret exfiltration — that is a security
#   concern better handled by the sandbox credential-block + permission denies in
#   templates/settings.example.json, not by grepping command strings (which false-positives
#   on legitimate API-key headers and is trivially bypassed).
#   - ALLOW everything else, including ordinary scoped cleanup (rm -rf ./dist,
#     rm -rf "$TMPDIR/x", git clean -fn dry-runs, curl with an API-key header).
#
# What it is NOT: a sandbox or a security boundary, and NOT the irreversibility layer.
# It matches a handful of high-signal patterns. It DOES segment the command string
# quote-aware first, but that is for precision, not for evasion resistance: without it
# `echo "rm -rf /"` and `grep -r "chmod 777 /" .` would be denied, and false denials on
# ordinary work are what get a guard switched off. Determined obfuscation (command
# substitution, stdin/heredoc-fed
# shells, aliases, escaped separators, wrapper option-values) will get past it — that is
# expected, and chasing every bypass with more regex is a losing game we do not play.
# Conversely it can occasionally over-flag: a heredoc/here-string BODY that literally
# contains a catastrophic-looking line is split like top-level shell, so a document that
# quotes `rm -rf /` may be denied — it errs safe, but it is a known false-positive edge.
#
# For REAL-WORLD irreversible actions (deploy, send, charge, drop a prod DB), the honest
# gate is not command-string parsing at all — it is declaration-based: see the portable
# `effect-broker` skill, which classifies an action and enforces simulate-then-commit.
# Real containment comes from the sandbox + permission system + the effect broker; this
# hook (Claude Code only) just catches the obvious LOCAL accident before it happens. It
# fails OPEN on anything it cannot read (missing jq, unparseable input) so it never wedges
# work; the one conservative case is a command too long to parse, which asks (see the cap).
#
# Commands wrapped in bash -c / sh -c / eval, and the argv of a find -exec/-ok primary,
# are re-scanned recursively (bounded depth) by the same rules a bare command gets. The
# reason is accident coverage, not evasion resistance: agents genuinely write
# `bash -c "cd $d && rm -rf $x"` and `find . -exec rm -rf $x \;`, and an unset variable
# there is as catastrophic nested as it is at the top level. Anyone deliberately hiding a
# command has easier routes (see the bypasses above); the depth cap exists to bound work,
# not to win that race. Each payload is strictly shorter than the command it came out of,
# so the recursion terminates on its own and the cap is only a work bound.
#
# SPEED: this hook runs before EVERY Bash call, so its cost is pure added latency.
# Two things keep it cheap. (1) LC_ALL=C: the parsers slice the command string, and
# under a UTF-8 locale every offset slice re-counts characters from the start, which
# makes them quadratic. Byte semantics are safe here because we only compare ASCII
# shell metacharacters, and UTF-8 continuation bytes (>= 0x80) can never collide with
# them, so multibyte data is copied through intact. (2) The parsers consume whole runs
# of ordinary characters per iteration instead of one character at a time, so the loop
# runs once per quote/separator rather than once per byte.
set -u
export LC_ALL=C LANG=C
command -v jq >/dev/null 2>&1 || exit 0
input="$(cat 2>/dev/null || true)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"
[ -n "$cmd" ] || exit 0

DECISION=""   # "deny" or "ask"
REASON=""
_PAYLOADS=()

emit() {
  jq -n --arg d "$1" --arg r "$2" \
    '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:$d,permissionDecisionReason:$r}}'
  exit 0
}

# --- cheap prefilter: skip the parser entirely on ordinary commands ----------------
# split_segments/shell_words/strip_quoted still re-slice the command per quote and
# separator, so their cost grows faster than linearly on very long input. Most commands
# cannot be destructive at all, and this grep is O(n) and runs once, so it answers them
# without paying for the parser. PREFILTER_TOKENS is the
# union of every command word the deny/ask checks anchor on (the scan_level case labels:
# rm/find/chmod/dd/mkfs/wipefs/blkdiscard/shred/git/the shell wrappers/eval) plus the
# raw-string anchors (curl/wget/fetch for remote_pipe_to_shell, /dev/ for the
# block-device redirect in raw_catastrophic, and the :() fork bomb). curl/wget/fetch are
# OUTSIDE the \b(...)\b group and unanchored on purpose: remote_pipe_to_shell() matches
# them as plain substrings too (so `prefetch ... | python3` or `xcurl ... | node` still
# ASK), and the prefilter must hit everything the parser would, so it stays unanchored
# here in lockstep. Keep it in lockstep with those tables; the main test replays every
# DENY/ASK fixture through it and fails if any stops hitting. Correctness rule: the
# prefilter may only fast-ALLOW on a no-hit. It never denies, and an oversized hit
# degrades to ASK, never a plain allow.
PREFILTER_TOKENS='\b(rm|find|chmod|dd|mkfs|wipefs|blkdiscard|shred|git|bash|sh|zsh|dash|ash|ksh|eval)\b|curl|wget|fetch|/dev/|:\(\)'
rc=0
printf '%s' "$cmd" | grep -Eq "$PREFILTER_TOKENS" || rc=$?
if [ "$rc" -eq 1 ]; then
  exit 0                                     # (a) no trigger token: allow, at any size
elif [ "$rc" -ge 2 ]; then
  # grep itself errored on this host/pattern (rc>=2, e.g. a non-GNU grep rejecting \b).
  # A no-hit must mean "confirmed no token", never "grep failed to check": an error
  # here is NOT a no-hit, so fall through to the full parser instead of fast-allowing.
  :
fi
# (c) trigger token present but the command is too long to parse cheaply. A token means
# it COULD be destructive, so we must not fast-allow: degrade conservatively to ASK. The
# cap is a latency guard, not a correctness one. Measured on real agent traffic the parser
# handles a 5000-char command in ~40ms, and 16000 chars of quote-dense worst-case input in
# ~0.5s; the p100 command in 1660 observed Bash calls was 5.4k, so this asks on effectively
# nothing. Keep it high: a cap set near normal command lengths turns the hook into a
# confirmation prompt on ordinary long heredocs, which is how a guard gets switched off.
if ((${#cmd} > 16000)); then
  emit ask "command exceeds the safe parse length and contains a potentially destructive token. Review it before running (shorten it for a precise check)."
fi
# (b) trigger token present and short enough: fall through to the exact existing parser.

# --- quote-aware segment splitter (reads global $cmd) ------------------------------
# Splits on ; & | and newlines at the top level; content inside '...', "...", `...`
# is never treated as a separator, so quoted data is not re-split. NUL-delimited so a
# quoted newline inside a segment survives. Readers use `read -r -d ''`.
# Chunked, not per-character: each iteration consumes a whole run of ordinary
# characters with one ${var%%pattern} strip, so the loop runs once per quote or
# separator rather than once per byte (see the SPEED note above).
split_segments() {
  local rest="$cmd" chunk ch next inner segment specials
  specials=$'[\'"`\n;&|]'
  segment=
  while [ -n "$rest" ]; do
    chunk="${rest%%$specials*}"
    if [ "$chunk" = "$rest" ]; then segment+="$rest"; break; fi
    segment+="$chunk"; rest="${rest:${#chunk}}"
    ch="${rest:0:1}"; rest="${rest:1}"
    case "$ch" in
      "'" | '"' | '`')
        # copy the quoted run verbatim, delimiters included
        segment+="$ch"
        inner="${rest%%"$ch"*}"
        if [ "$inner" = "$rest" ]; then segment+="$rest"; rest=      # unterminated
        else segment+="$inner$ch"; rest="${rest:$((${#inner} + 1))}"; fi
        ;;
      *)
        printf '%s\0' "$segment"; segment=
        next="${rest:0:1}"
        { [ "$ch" = '&' ] || [ "$ch" = '|' ]; } && [ "$next" = "$ch" ] && rest="${rest:1}"
        ;;
    esac
  done
  printf '%s\0' "$segment"
}

# Split a segment into shell words, honoring '...' and "..." quoting and dropping the
# quote characters (so "/" becomes /, and "rm -rf /" as an echo ARG stays an arg).
# Fills the global array WORDS. Returns 1 on an unterminated quote.
WORDS=()
shell_words() {
  local rest="$1" chunk ch inner token in_token specials
  specials=$'[[:space:]\'"]'
  WORDS=(); token=; in_token=0
  while [ -n "$rest" ]; do
    chunk="${rest%%$specials*}"
    if [ "$chunk" = "$rest" ]; then token+="$rest"; in_token=1; break; fi
    if [ -n "$chunk" ]; then token+="$chunk"; in_token=1; rest="${rest:${#chunk}}"; fi
    ch="${rest:0:1}"; rest="${rest:1}"
    case "$ch" in
      "'" | '"')
        in_token=1
        inner="${rest%%"$ch"*}"
        [ "$inner" = "$rest" ] && return 1                          # unterminated quote
        token+="$inner"; rest="${rest:$((${#inner} + 1))}"
        ;;
      *) [ "$in_token" -eq 1 ] && { WORDS+=("$token"); token=; in_token=0; } ;;
    esac
  done
  [ "$in_token" -eq 1 ] && WORDS+=("$token")
  return 0
}

is_var_assignment() { [[ "$1" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; }

# Resolve a segment's leading command past sudo/doas/env/nice/VAR=/redirect preamble.
# Sets RC_NAME (basename) and RC_TAIL (words after it, as a string). Returns 1 if none.
RC_NAME=""; RC_TAIL=""
resolve_command() {
  local text word wrapper
  # Work on the original string (not WORDS) so RC_TAIL keeps the raw, unsplit tail.
  text="$1"
  text="${text#"${text%%[![:space:]]*}"}"
  while :; do
    [[ "$text" =~ ^([^[:space:]]+) ]] || return 1
    word="${BASH_REMATCH[1]}"
    # skip a leading redirection like 2>/dev/null, >out, >>log, 2>&1, &>x, <in
    if [[ "$word" =~ ^([0-9]*(\&?>>?|<{1,3}>?)|[0-9]*[<>]\&[0-9-]*) ]]; then
      text="${text:${#word}}"; text="${text#"${text%%[![:space:]]*}"}"; continue
    fi
    if is_var_assignment "$word"; then
      text="${text:${#word}}"; text="${text#"${text%%[![:space:]]*}"}"; continue
    fi
    if [[ "$word" =~ ^(sudo|doas|command|env|nice|ionice|time|exec|nohup|setsid|stdbuf)$ ]]; then
      wrapper="$word"; text="${text:${#word}}"; text="${text#"${text%%[![:space:]]*}"}"
      # skip this wrapper's flags and VAR= assignments; conservatively skip one arg for
      # the common option-takes-value flags so we still reach the real command.
      while [[ "$text" =~ ^([^[:space:]]+) ]]; do
        word="${BASH_REMATCH[1]}"
        if [[ "$word" = -* ]] || is_var_assignment "$word"; then
          text="${text:${#word}}"; text="${text#"${text%%[![:space:]]*}"}"
          case "$wrapper:$word" in
            sudo:-u|sudo:-g|sudo:-U|sudo:-C|sudo:-p|sudo:-r|sudo:-t|sudo:-h|env:-u|env:-C|env:--unset|env:--chdir|nice:-n|ionice:-n|ionice:-c|ionice:-p|doas:-u|exec:-a|stdbuf:-i|stdbuf:-o|stdbuf:-e)
              [[ "$text" =~ ^([^[:space:]]+) ]] && { text="${text:${#BASH_REMATCH[1]}}"; text="${text#"${text%%[![:space:]]*}"}"; } ;;
          esac
          continue
        fi
        break
      done
      continue
    fi
    break
  done
  [[ "$text" =~ ^([^[:space:]]+) ]] || return 1
  word="${BASH_REMATCH[1]}"
  RC_TAIL="${text:${#word}}"
  while [[ "$word" = \\* ]]; do word="${word:1}"; done   # \rm (alias bypass) -> rm
  RC_NAME="${word##*/}"
  return 0
}

# --- lexical path normalization ----------------------------------------------------
# Equivalent spellings of one path must classify the same, or the test below is a
# spelling check rather than a target check: without this `rm -rf /home/alice//`,
# `rm -rf /home/alice/.`, and `rm -rf .././..` all name the home or parent directory
# while reading as ordinary scoped paths, and run with no guard at all.
# Collapses repeated separators, drops "." components, and folds ".." into the component
# before it. Purely textual on purpose: the guard must decide from the command string
# alone, so it never stats the path (the target frequently does not exist yet, and a
# guard that needs the filesystem answers differently on every host) and never resolves a
# symlink (that would let the disk, not the command, pick the verdict).
#
# A "${VAR:-/x}" word is split on its inner slash too. That is safe because the joins put
# the separator back and the patterns below key on the "${HOME:" prefix and the closing
# brace, which both survive; special-casing braces would be code no input can exercise.
NORM=""
normalize_path() {
  local rest="$1" abs=0 seg prev out="" i n=0
  local -a parts=()
  case "$rest" in /*) abs=1 ;; esac
  while [ -n "$rest" ]; do
    seg="${rest%%/*}"
    if [ "$seg" = "$rest" ]; then rest=; else rest="${rest:${#seg}+1}"; fi
    case "$seg" in
      "" | ".") continue ;;                      # repeated separator or a no-op component
      "..")
        prev=""; [ "$n" -gt 0 ] && prev="${parts[$((n - 1))]}"
        case "$prev" in
          "" | ".." | *'$'* | *'*'* | "~")
            # Nothing to fold into, or a component whose value the command string does
            # not state. Folding there would invent a path ($HOME/.. is not "."), so the
            # ".." stays literal and the classifier sees the same shape it sees today.
            [ "$abs" -eq 1 ] && [ "$n" -eq 0 ] && continue   # "/.." is "/"
            parts[$n]=".."; n=$((n + 1)) ;;
          *) unset "parts[$((n - 1))]"; n=$((n - 1)) ;;
        esac
        ;;
      *) parts[$n]="$seg"; n=$((n + 1)) ;;
    esac
  done
  for ((i = 0; i < n; i++)); do out="${out:+$out/}${parts[$i]}"; done
  [ "$abs" -eq 1 ] && out="/$out"
  [ -n "$out" ] || out="."                       # "", ".", "./" and "x/.." all mean the cwd
  NORM="$out"
}

# --- catastrophic target test (precise; NOT "any absolute/glob/var") ---------------
# Matches only root, a home directory, top-level system directories, and a parent-
# relative escape. Those are the paths whose recursive deletion is unrecoverable, or
# whose real location the command string does not pin down. A specific subdir ("$TMPDIR/x",
# "./dist", "~/.cache", "/tmp/app", "/home/alice/Code") is NOT catastrophic and passes.
is_catastrophic_target() {
  local w="$1" root base
  while [[ "$w" = \\* ]]; do w="${w:1}"; done   # strip leading escapes
  w="${w%%\\}"                                   # strip a trailing escape artifact
  [ -z "$w" ] && return 1
  # every comparison below is against a normalized path, so trailing and doubled
  # slashes, "." components, and ".." folding are handled once here instead of being
  # re-spelled into each pattern (which is how "/home/alice//" got through).
  normalize_path "$w"; w="$NORM"
  # the "~" patterns match a LITERAL tilde the agent passed unexpanded; $HOME is
  # matched by the patterns below.
  # shellcheck disable=SC2088

  case "$w" in
    "/" | "/*" | "/." | "/.*") return 0 ;;
    "~" | "~/*") return 0 ;;
    # $HOME family: bare, braced, and modifier forms (${HOME:?}, ${HOME:-/}), each
    # optionally with a /* suffix (whole-home wipe). A specific subdir like $HOME/x is safe.
    '$HOME' | '${HOME}' | '$HOME/*' | '${HOME}/*') return 0 ;;
    '${HOME:'*'}' | '${HOME:'*'}/*') return 0 ;;
  esac
  # A top-level system/root dir — either the dir ITSELF or a wildcard of all its
  # immediate contents (/etc, /etc/*). A deeper path (/var/lib/app, /opt/app/*) is
  # scoped and NOT catastrophic. Linux + macOS roots.
  for root in bin boot dev etc lib lib32 lib64 libx32 opt proc root run sbin srv sys usr var home \
              Users Applications System Library Volumes private Network cores; do
    [ "$w" = "/$root" ] && return 0
    [ "$w" = "/$root/*" ] && return 0
  done
  # A home directory named LITERALLY: /home/alice, /Users/alice, or the expanded value
  # of $HOME. The patterns above only catch the unexpanded '$HOME'/'~' spellings and the
  # bare '/home' parent, so an agent that writes out the real path (the shape that
  # actually turns up in traffic) walked past the guard and wiped its own home. 'base'
  # drops a trailing '/*' first because deleting a home dir's contents is the same loss
  # as deleting the dir. One level deep only: /home/alice/Code stays scoped.
  base="${w%/\*}"
  case "$base" in
    /home/?* | /Users/?*) [ "${base#/*/}" = "${base##*/}" ] && return 0 ;;
  esac
  [ -n "${HOME:-}" ] && [ "$HOME" != "/" ] && [ "$base" = "${HOME%/}" ] && return 0
  # A parent-relative escape ('..', '../..', deeper, and their '/*' contents form).
  # Where it lands depends on a cwd the command string does not state, so a recursive
  # delete rooted there is not a scoped delete. '.' and '../sibling' stay scoped.
  [[ "$w" =~ ^\.\.(/\.\.)*(/\*)?$ ]] && return 0
  return 1
}

has_recursive_flag() {
  local w
  for w in "${WORDS[@]}"; do
    [ "$w" = "--recursive" ] && return 0
    [[ "$w" = --* ]] && continue
    [[ "$w" = -?* && "$w" = *[rR]* ]] && return 0
  done
  return 1
}

# rm -rf <catastrophic>  -> deny
rm_is_catastrophic() {
  shell_words "$1" || return 1
  has_recursive_flag || return 1
  local w end=0
  for w in "${WORDS[@]}"; do
    [ "$end" -eq 0 ] && [ "$w" = "--" ] && { end=1; continue; }
    [ "$end" -eq 0 ] && [[ "$w" = -* ]] && continue
    is_catastrophic_target "$w" && return 0
  done
  return 1
}

# --- find: a catastrophic start path plus a primary that ACTS ----------------------
# The TIER rule below is a conjunction with is_catastrophic_target, so no tier can fire
# on `find . -name x ...`. That is what keeps the widened primary set cheap: the shapes
# it newly reaches are all rooted at /, a home directory, a top-level system directory,
# or a parent-relative escape.
#
# The exec argv re-scan is the one thing here that is NOT gated on the start path, and it
# has to be: `find .` in a harmless directory otherwise launders `rm -rf /`. See
# _queue_exec_payload for why that costs no extra noise.
#
# Tier the exec target instead of comparing it to one name. The old rule was "the word
# after -exec is literally rm", which trusted a NAME rather than proving a PROPERTY:
# `-exec sh -c ...` runs anything at all, and so does a target this guard has never
# heard of. So the first word after the primary decides.
#   2 (deny)  an unrecoverable destroyer, run once per matched file.
#   0 (silent) a command that cannot write. `find . -exec grep -l pat {} +` is an
#             everyday idiom and its rooted cousin `find / -exec grep ...` is a real
#             search; prompting on those is the false positive that gets a guard
#             switched off, so proven-read-only targets produce no signal at all.
#   1 (ask)   everything else: a shell, an interpreter, a wrapper like sudo or xargs,
#             or a program the guard does not recognize. An unknown target ASKS rather
#             than allows because the command string does not say what it does, and
#             running it over every file under / or a home directory is the accident
#             this rule exists for. It does not DENY, because unproven is not the same
#             as unrecoverable and this tier has to stay cheap to be wrong about.
# The read-only set is deliberately short and every member is a pure reader. `file` is
# absent on purpose: its -C flag compiles a magic cache to disk.
#
# The tier answers ONE question: is running this target over every file under a
# catastrophic start path worth a prompt. It says nothing about the target's own
# arguments, which is why the exec argv is ALSO queued for a full recursive re-scan (see
# _queue_exec_payload). The two are independent: the tier needs a dangerous start path,
# the re-scan does not.
_find_exec_tier() {
  local t="$1"
  while [[ "$t" = \\* ]]; do t="${t:1}"; done   # \rm (alias bypass) -> rm
  t="${t##*/}"
  case "$t" in
    rm|unlink|shred) return 2 ;;
    grep|egrep|fgrep|cat|head|tail|wc|ls|stat|du|readlink|realpath|basename|dirname|echo|printf|true|false|cmp|diff|md5sum|sha1sum|sha256sum|sha512sum) return 0 ;;
    *) return 1 ;;
  esac
}

# Queue a find -exec target's OWN argv for the same recursive scan a `bash -c` or an
# `eval` payload gets. Without this the guard classified only find's START path, so
# `find . -exec rm -rf / \;` and `find . -exec sh -c 'rm -rf /' \;` ran silently from a
# harmless directory while the bare `rm -rf /` and `sh -c 'rm -rf /'` were denied. A find
# over a scoped tree launders the catastrophe, and that is a one-typo accident, not a
# crafted bypass.
#
# The words are re-quoted because shell_words already stripped the original quoting, and
# `sh -c rm -rf /` re-lexes into a different command than `sh -c 'rm -rf /'`: the payload
# collector would take only `rm` as the program. A word that re-lexes unchanged stays
# bare, because the command NAME must reach resolve_command without quotes around it.
#
# This adds no new noise on its own. The payload is judged by exactly the rules a bare
# command is judged by, so `find . -exec mytool {} \;` and `find . -exec chmod 644 {} \;`
# stay silent for the same reason bare `mytool` and bare `chmod 644 x` do.
_queue_exec_payload() {
  local w out="" specials
  specials=$'*[[:space:]\'"`$;&|()<>]*'
  for w in "$@"; do
    # shellcheck disable=SC2053   # $specials is a glob on purpose
    if [[ "$w" == $specials ]]; then
      if [[ "$w" != *\'* ]]; then w="'$w'"
      elif [[ "$w" != *\"* ]]; then w="\"$w\""
      fi
      # A word carrying BOTH quote characters is left bare. This lexer has no escapes, so
      # there is no spelling that survives, and passing it through is the silence that
      # stood here before rather than a new hole.
    fi
    out="${out:+$out }$w"
  done
  [ -n "$out" ] || return 0
  _PAYLOADS+=("$out")
}

# Sets FIND_TIER to "deny" or "ask" and returns 0; returns 1 when the command carries
# no signal. Deny outranks ask within one find, so `find / -delete -exec grep ...`
# still denies. Queues every -exec argv into _PAYLOADS regardless of the verdict, because
# a scoped start path silences the tier but must not silence the target itself.
FIND_TIER=""
find_is_risky() {
  shell_words "$1" || return 1
  local w in_starts=1 danger_start=0 deny=0 ask=0 in_exec=0 first_exec=0
  local -a execv=()
  FIND_TIER=""
  for w in "${WORDS[@]}"; do
    if [ "$in_exec" -eq 1 ]; then
      # An exec argv ends at ';' or '+'. A lone backslash counts because the top-level
      # splitter already consumed the ';' of a '\;' terminator, leaving the escape behind;
      # without this the argv would swallow every primary that follows it.
      case "$w" in
        ';' | '+' | '\' | '\;' | '\+')
          _queue_exec_payload ${execv+"${execv[@]}"}; execv=(); in_exec=0; continue ;;
      esac
      if [ "$first_exec" -eq 1 ]; then
        first_exec=0
        _find_exec_tier "$w"
        case "$?" in 2) deny=1 ;; 1) ask=1 ;; esac
      fi
      execv+=("$w")
      continue
    fi
    if [ "$in_starts" -eq 1 ]; then
      case "$w" in
        -H|-L|-P|-O*|-D) continue ;;
        -*|'('|'!') in_starts=0 ;;
        *) is_catastrophic_target "$w" && danger_start=1; continue ;;
      esac
    fi
    case "$w" in
      -delete) deny=1 ;;
      # -ok/-okdir are -exec with a confirmation prompt. They are still action-bearing,
      # and a hook must not assume the prompt saves anyone: an agent's find runs with no
      # terminal, so whether the prompt is even answerable depends on the harness.
      -exec|-execdir|-ok|-okdir) in_exec=1; first_exec=1 ;;
      # These do not touch what they match: they TRUNCATE the file named on the command
      # line. One clobbered file is a bounded, recoverable loss, so it asks instead of
      # denying, but `find / -fls ~/.bashrc` reads as a listing and destroys a dotfile,
      # which is exactly the accident worth one confirmation.
      -fprint|-fprint0|-fprintf|-fls) ask=1 ;;
    esac
  done
  # An unterminated argv is the COMMON case, not an edge: the splitter eats the ';' of
  # '\;', so the exec target usually runs to the end of the segment.
  [ "$in_exec" -eq 1 ] && _queue_exec_payload ${execv+"${execv[@]}"}
  [ "$danger_start" -eq 1 ] || return 1
  [ "$deny" -eq 1 ] && { FIND_TIER="deny"; return 0; }
  [ "$ask" -eq 1 ] && { FIND_TIER="ask"; return 0; }
  return 1
}

# A mode that grants write broadly: numeric 777, or a symbolic mode that grants write
# to "other" or "all" (o/a scope, or an empty scope which means all). u+w / g+w alone
# are not broad. Removing perms (op '-') is never the risk.
_is_broad_write_mode() {
  local m="$1" scope op rest
  case "$m" in 777|0777|1777) return 0 ;; esac
  [[ "$m" =~ ^([ugoa]*)([-+=])([rwxXst]*)$ ]] || return 1
  scope="${BASH_REMATCH[1]}"; op="${BASH_REMATCH[2]}"; rest="${BASH_REMATCH[3]}"
  [ "$op" = "-" ] && return 1
  [[ "$rest" = *w* ]] || return 1
  [ -z "$scope" ] && return 0                       # bare +w/=rwx means all
  [[ "$scope" = *a* || "$scope" = *o* ]] && return 0
  return 1
}
chmod_is_catastrophic() {
  shell_words "$1" || return 1
  local w badmode=0 hasroot=0
  for w in "${WORDS[@]}"; do
    case "$w" in
      -*) : ;;
      *) _is_broad_write_mode "$w" && badmode=1; is_catastrophic_target "$w" && hasroot=1 ;;
    esac
  done
  [ "$badmode" -eq 1 ] && [ "$hasroot" -eq 1 ]
}

# Block-device targets (whole-disk overwrite). Excludes char devices like /dev/null,
# /dev/zero, /dev/random which are legitimate dd targets.
_is_block_device() {
  case "$1" in
    # /dev/loop[0-9]* (a numbered loop dev), NOT /dev/loop-control (a char control node)
    /dev/sd*|/dev/nvme*|/dev/vd*|/dev/xvd*|/dev/mmcblk*|/dev/disk*|/dev/rdisk*|/dev/mapper/*|/dev/dm-*|/dev/md[0-9]*|/dev/md/*|/dev/loop[0-9]*|/dev/mtdblock*|/dev/hd*|/dev/sr*|/dev/vblk*|/dev/rbd*|/dev/nbd*|/dev/drbd*|/dev/pmem*|/dev/zvol/*) return 0 ;;
  esac
  # Fallback: if the path exists and is actually a block device, treat it as one
  # (covers device families not in the static list above). Harmless when it doesn't
  # exist (the static list is the portable primary check).
  [ -b "$1" ] && return 0
  return 1
}
dd_is_catastrophic() {
  shell_words "$1" || return 1
  local w
  for w in "${WORDS[@]}"; do case "$w" in of=*) _is_block_device "${w#of=}" && return 0 ;; esac; done
  return 1
}
# A disk-format/wipe tool is catastrophic only when it targets a block DEVICE. The
# same tool against a plain file (e.g. `mkfs.ext4 disk.img` for a loopback image) is
# a normal, reversible operation and must NOT be denied.
format_is_catastrophic() {
  shell_words "$1" || return 1
  local w
  for w in "${WORDS[@]}"; do _is_block_device "$w" && return 0; done
  return 1
}

# --- ASK-tier tails (reversible / routine but worth a confirm) ---------------------
# A checkout/restore pathspec that resolves to the whole repo root or the whole
# cwd. Catches the plain forms (., ./, ./., bare * or **) by stripping leading
# ./ repeats, the ':/' root magic, and any ':(...)' long-form magic (top, glob,
# icase, in any order) whose path part is empty, '.', '*', or '**'. A scoped
# spec (./src, :(top)src, :/sub, *.md) is not whole-tree and stays allowed.
_is_whole_tree_pathspec() {
  local w="$1" p
  case "$w" in
    :/|':/.'|':/*'|':/**') return 0 ;;   # quoted patterns: literal :/* etc., not a glob
    ':('*')'*)
      p="${w#*)}"
      case "$p" in ''|.|'*'|'**') return 0 ;; esac
      return 1 ;;
  esac
  while [ "${w#./}" != "$w" ]; do w="${w#./}"; done
  case "$w" in ''|.|'*'|'**') return 0 ;; esac
  return 1
}
git_is_risky() {
  shell_words "$1" || return 1
  local w sub="" reset_hard=0 clean_force=0 clean_dry=0 branch_force=0 branch_del=0 pushforce=0 skip=0
  local co_dot=0 rst_dot=0 rst_staged=0 rst_worktree=0
  for w in "${WORDS[@]}"; do
    if [ -z "$sub" ]; then
      if [ "$skip" -eq 1 ]; then skip=0; continue; fi
      case "$w" in
        -C|-c|--git-dir|--work-tree|--namespace) skip=1; continue ;;
        --*=*|--no-pager|--paginate|--bare) continue ;;
        -*) continue ;;
        reset|clean|branch|push|checkout|restore) sub="$w"; continue ;;
        *) return 1 ;;
      esac
    fi
    case "$sub" in
      reset) [ "$w" = "--hard" ] && reset_hard=1 ;;
      clean) case "$w" in
          --force) clean_force=1 ;;
          --dry-run) clean_dry=1 ;;
          --*) : ;;
          -[A-Za-z]*) [[ "$w" = *f* ]] && clean_force=1; [[ "$w" = *n* ]] && clean_dry=1 ;;
        esac ;;
      branch) case "$w" in
          -D) branch_del=1; branch_force=1 ;;
          -d|--delete) branch_del=1 ;;
          -f|--force) branch_force=1 ;;
          -[A-Za-z]*) [[ "$w" = *D* ]] && { branch_del=1; branch_force=1; }; [[ "$w" = *d* ]] && branch_del=1; [[ "$w" = *f* ]] && branch_force=1 ;;
        esac ;;
      push) case "$w" in --force-with-lease*|--force-if-includes) : ;; --force|-f) pushforce=1 ;; +[!-]*) pushforce=1 ;; esac ;;   # bare --force is risky even if a lease flag is also present (last one wins); lease-only is safe
      # whole-tree discard: a pathspec that resolves to the repo root ('.', './',
      # './.', ':/', or ':(top)' magic with no scoping path) after checkout/restore
      # discards every uncommitted change with no reflog. A branch name or a
      # specific path stays allowed.
      checkout) case "$w" in -*) : ;; *) _is_whole_tree_pathspec "$w" && co_dot=1 ;; esac ;;
      restore) case "$w" in
          --staged) rst_staged=1 ;;
          --worktree) rst_worktree=1 ;;
          --*) : ;;
          -[A-Za-z]*) [[ "$w" = *S* ]] && rst_staged=1; [[ "$w" = *W* ]] && rst_worktree=1 ;;
          *) _is_whole_tree_pathspec "$w" && rst_dot=1 ;;
        esac ;;
    esac
  done
  [ "$reset_hard" -eq 1 ] && return 0
  [ "$clean_force" -eq 1 ] && [ "$clean_dry" -eq 0 ] && return 0
  { [ "$branch_del" -eq 1 ] && [ "$branch_force" -eq 1 ]; } && return 0
  [ "$pushforce" -eq 1 ] && return 0            # bare --force/-f present (a lease flag doesn't neutralize it)
  [ "$co_dot" -eq 1 ] && return 0               # git checkout . / checkout -- . (whole-tree discard)
  # restore of the worktree at '.': plain `git restore .` (worktree by default) or any
  # form with --worktree/-W; `git restore --staged .` alone only unstages and is safe.
  if [ "$rst_dot" -eq 1 ]; then
    { [ "$rst_worktree" -eq 1 ] || [ "$rst_staged" -eq 0 ]; } && return 0
  fi
  return 1
}

# --- raw-string tier: only catastrophic, quote-stripped so quoted data can't trip it -
# Rebuild the command with quoted regions removed, then match block-device redirect and
# fork bomb. (echo ':(){ :|:& };:' keeps the payload inside quotes -> stripped -> safe.)
raw_catastrophic() {
  local dequoted="$1"
  # device-family list must stay in sync with _is_block_device() above (glob there,
  # ERE here). Covers a redirect like `cat /dev/zero > /dev/rdisk3` or `: > /dev/sda`.
  if printf '%s' "$dequoted" | grep -Eq '(^|[^<])>[[:space:]]*/dev/(sd|nvme|vd|xvd|mmcblk|disk|rdisk|mapper/|dm-|md[0-9]|md/|loop[0-9]|mtdblock|hd|sr|vblk|rbd|nbd|drbd|pmem|zvol/)'; then
    REASON="redirect to a raw block device (would overwrite a disk)"; DECISION="deny"; return 0
  fi
  if printf '%s' "$dequoted" | grep -Eq ':\(\)[[:space:]]*\{[[:space:]]*:[[:space:]]*\|[[:space:]]*:'; then
    REASON="fork bomb"; DECISION="deny"; return 0
  fi
  return 1
}

# A remote download piped into an interpreter that EXECUTES it (curl … | bash). Quote-aware
# caller passes de-quoted text, so a mention inside a string doesn't trip it. This is a
# footgun worth a confirm, not an unrecoverable catastrophe -> ASK.
#
# The interpreter must be reading its program FROM the pipe. `curl … | python3 -c '<script>'`
# and `curl … | node parse.js` run their own program and treat the download as DATA — that is
# the ordinary way to read a JSON API from the shell and must not prompt. So a code-bearing
# argument (-c/-e/-m, --eval, or a script path) means allow, while a bare `| bash`, `| sh -x`,
# `| bash -s -- --yes`, or `| python3 -` reads stdin as its program and still asks. The
# download and the pipe must also sit in one command list, on one line (no ; & or newline
# between them), so a `curl` earlier in a script can't pair with an unrelated later pipe.
remote_pipe_to_shell() {
  local re w codepat
  local -a tailwords
  re=$'(curl|wget|fetch)[^|;&\n]*\\|[[:space:]]*((sudo|env|command|exec|nohup)[[:space:]]+)*([^[:space:]]*/)?(sh|bash|zsh|dash|ksh|python[0-9.]*|node|ruby|perl)([[:space:]]+[^|;&\n]*)?([|;&\n]|$)'
  [[ "$1" =~ $re ]] || return 1
  # which short flag carries the program depends on the interpreter: a shell takes only
  # -c, while python/node/ruby/perl also take -e/-m (and clusters like perl -ne). `sh -e`
  # is errexit, NOT a program, so it must stay in the asking set.
  case "${BASH_REMATCH[5]}" in
    sh | bash | zsh | dash | ksh) codepat='-*c*' ;;
    *) codepat='-*[cem]*' ;;
  esac
  read -ra tailwords <<< "${BASH_REMATCH[6]}"
  for w in ${tailwords+"${tailwords[@]}"}; do
    case "$w" in
      --) break ;;                                  # end of options; the rest are the piped program's args
      *'>'* | *'<'*) break ;;                       # a redirection tail (| bash > install.log), not a program
      --command* | --eval* | --module*) return 1 ;;
      --*) continue ;;                              # a long option that carries no program
      -*)
        # shellcheck disable=SC2053   # $codepat is a glob on purpose
        [[ "$w" == $codepat ]] && return 1
        continue ;;
      *) return 1 ;;                                # a script path: stdin is that script's data
    esac
  done
  return 0
}

# Remove quoted regions (keep the quotes' delimiters gone AND their content gone) so
# raw matching only sees UNQUOTED shell text.
strip_quoted() {
  local rest="$1" out="" chunk ch inner specials
  specials=$'[\'"`]'
  while [ -n "$rest" ]; do
    chunk="${rest%%$specials*}"
    if [ "$chunk" = "$rest" ]; then out+="$rest"; break; fi
    out+="$chunk"; rest="${rest:${#chunk}}"
    ch="${rest:0:1}"; rest="${rest:1}"
    inner="${rest%%"$ch"*}"
    if [ "$inner" = "$rest" ]; then rest=; else rest="${rest:$((${#inner} + 1))}"; fi
  done
  printf '%s' "$out"
}

# Scan every segment once. Sets DECISION/REASON on the first catastrophic (deny) hit and
# returns 0. If no deny hit but an ask-worthy op is seen, records it (ask) and keeps
# scanning for a deny (deny outranks ask). Collects bash -c/eval payloads in _PAYLOADS.
scan_level() {
  local seg name tail take pw ask_reason="" ew joined
  _PAYLOADS=()
  while IFS= read -r -d '' seg || [ -n "$seg" ]; do
    resolve_command "$seg" || continue
    name="$RC_NAME"; tail="$RC_TAIL"
    case "$name" in
      rm)     rm_is_catastrophic "$tail"     && { REASON="recursive rm of a root, home, or system directory. Delete a specific subdirectory instead (e.g. rm -rf ./dist)."; DECISION="deny"; return 0; } ;;
      find)
        if find_is_risky "$tail"; then
          [ "$FIND_TIER" = "deny" ] && { REASON="find deleting or shredding from a root/home/system start path. Use a specific relative start path."; DECISION="deny"; return 0; }
          ask_reason="find running an unvetted -exec/-ok target, or a -fprint/-fls file write, from a root/home/system start path. The command string does not say what that target does to every file it matches; confirm it, or use a specific relative start path."
        fi ;;
      chmod)  chmod_is_catastrophic "$tail"  && { REASON="chmod 777 on a root/system path."; DECISION="deny"; return 0; } ;;
      dd)     dd_is_catastrophic "$tail"     && { REASON="dd writing to a raw block device (would overwrite a disk)."; DECISION="deny"; return 0; } ;;
      mkfs|mkfs.*|wipefs|blkdiscard|shred)
        # device-gated: shred/wipefs/mkfs of a block DEVICE wipes a disk; against a
        # plain file (shred secret.txt, mkfs.ext4 disk.img) it is a normal operation.
        format_is_catastrophic "$tail" && { REASON="$name targeting a block device (would wipe a disk). Against a plain file this is allowed."; DECISION="deny"; return 0; } ;;
      git)      git_is_risky "$tail"     && ask_reason="destructive git (reset --hard / clean -f / branch -D / push --force / whole-tree checkout or restore). Uncommitted changes have no reflog; confirm it's intended, or target specific paths." ;;
      bash|sh|zsh|dash|ash|ksh)
        shell_words "$tail" || WORDS=()
        take=0
        for pw in "${WORDS[@]}"; do
          if [ "$take" = 1 ]; then [ -n "$pw" ] && _PAYLOADS+=("$pw"); take=0; continue; fi
          case "$pw" in -c|-[A-Za-z]*c) take=1 ;; esac
        done
        ;;
      eval)
        shell_words "$tail" || WORDS=()
        joined=""
        for ew in "${WORDS[@]}"; do [ -n "$ew" ] || continue; joined="${joined:+$joined }$ew"; done
        [ -n "$joined" ] && _PAYLOADS+=("$joined")
        ;;
    esac
    [ -n "$ask_reason" ] && [ -z "$REASON" ] && { REASON="$ask_reason"; DECISION="ask"; }
  done < <(split_segments)

  # raw patterns on the UNQUOTED command text (quote-stripped so quoted data can't trip)
  local dequoted; dequoted="$(strip_quoted "$cmd")"
  raw_catastrophic "$dequoted" && return 0
  if [ -z "$DECISION" ] && remote_pipe_to_shell "$dequoted"; then
    REASON="piping a remote download into a shell/interpreter. Download and inspect it first, or confirm."; DECISION="ask"
  fi
  [ "$DECISION" = "ask" ] && return 0
  return 1
}

# Scan a command and everything it wraps via -c/eval, up to a depth cap. A deny found at
# any depth wins; otherwise the outermost ask (if any) stands.
analyze() {
  local depth="$1" payload saved_cmd
  local -a payloads
  if scan_level; then
    [ "$DECISION" = "deny" ] && return 0
    # ask recorded; keep scanning payloads for a deny that would override it
  fi
  if [ "$depth" -lt 8 ] && [ "${#_PAYLOADS[@]}" -gt 0 ]; then
    payloads=("${_PAYLOADS[@]}")
    saved_cmd="$cmd"
    for payload in "${payloads[@]}"; do
      [ -n "$payload" ] || continue
      cmd="$payload"
      analyze "$((depth + 1))"
      [ "$DECISION" = "deny" ] && { cmd="$saved_cmd"; return 0; }
    done
    cmd="$saved_cmd"
  fi
  [ -n "$DECISION" ] && return 0
  return 1
}

if analyze 0; then
  emit "$DECISION" "$REASON"
fi
exit 0
