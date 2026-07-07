#!/usr/bin/env bash
# PreToolUse(Bash) guard — a SMALL, HIGH-CONFIDENCE accident tripwire.
#
# What it does:
#   - DENY a short list of catastrophic, unrecoverable commands (wipe root/home/system
#     dirs, format a disk, overwrite a block device, fork bomb, chmod 777 /). These are
#     never a legitimate agent action, so denying them costs no real capability.
#   - ASK (surface a confirmation instead of a flat refusal) for reversible-but-risky
#     ops: destructive git (reset --hard, clean -f, branch -D, push --force, and a
#     whole-tree checkout/restore of '.', which discards uncommitted work), bulk cloud
#     deletes (aws s3 rm --recursive / rb --force), prune/destroy/delete-all
#     (docker prune -f, terraform destroy -auto-approve, kubectl delete --all), and a
#     remote download piped into a shell (curl … | bash). These are recoverable (reflog,
#     versioning) or a deliberate footgun, so the human decides in the moment.
#   Note: this hook does NOT try to catch secret exfiltration — that is a security
#   concern better handled by the sandbox credential-block + permission denies in
#   templates/settings.example.json, not by grepping command strings (which false-positives
#   on legitimate API-key headers and is trivially bypassed).
#   - ALLOW everything else, including ordinary scoped cleanup (rm -rf ./dist,
#     rm -rf "$TMPDIR/x", git clean -fn dry-runs, curl with an API-key header).
#
# What it is NOT: a sandbox or a security boundary, and NOT the irreversibility layer.
# It matches a handful of high-signal patterns and deliberately does not try to parse
# arbitrary shell. Determined obfuscation (command substitution, stdin/heredoc-fed
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
# fails OPEN: any parse error or oversized input exits 0 (allow) so it never wedges work.
#
# Commands wrapped in bash -c / sh -c / eval are re-scanned recursively (bounded depth)
# so the wrapper doesn't trivially defeat the checks.
set -u
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

# --- cheap prefilter: bound the O(n^2) parser cost --------------------------------
# split_segments/shell_words/strip_quoted scan the command with per-character bash
# loops; ${cmd:i:1} is O(offset) under a UTF-8 locale, so the parser is quadratic in
# command length. On a benign multi-KB heredoc that is over a second of dead wait added
# to EVERY Bash tool call. This grep is O(n) and runs once. PREFILTER_TOKENS is the
# union of every command word the deny/ask checks anchor on (the scan_level case labels:
# rm/find/chmod/dd/mkfs/wipefs/blkdiscard/shred/git/aws/docker/terraform/tofu/kubectl/
# the shell wrappers/eval) plus the raw-string anchors (curl/wget/fetch for
# remote_pipe_to_shell, /dev/ for the block-device redirect in raw_catastrophic, and the
# :() fork bomb). curl/wget/fetch are OUTSIDE the \b(...)\b group and unanchored on
# purpose: remote_pipe_to_shell() matches them as plain substrings too (so `prefetch ... |
# python3` or `xcurl ... | node` still ASK), and the prefilter must hit everything the
# parser would, so it stays unanchored here in lockstep. Keep it in lockstep with those
# tables; prefilter-coverage.test.sh replays every DENY/ASK fixture through it and fails
# if any stops hitting. Correctness rule: the prefilter may only fast-ALLOW on a no-hit.
# It never denies, and an oversized hit degrades to ASK, never a plain allow.
PREFILTER_TOKENS='\b(rm|find|chmod|dd|mkfs|wipefs|blkdiscard|shred|git|aws|docker|terraform|tofu|kubectl|bash|sh|zsh|dash|ash|ksh|eval)\b|curl|wget|fetch|/dev/|:\(\)'
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
# (c) trigger token present but the command is too long to parse cheaply. The parser is
# quadratic, so past ~4000 chars it becomes seconds of latency (the old 20000-char cap
# sat exactly where latency peaked at ~11s AND fail-OPEN-allowed a 20k command that could
# be deniable). A token means it COULD be destructive, so we must not fast-allow: degrade
# conservatively to ASK.
if ((${#cmd} > 4000)); then
  emit ask "command exceeds the safe parse length and contains a potentially destructive token. Review it before running (shorten it for a precise check)."
fi
# (b) trigger token present and short enough: fall through to the exact existing parser.

# --- quote-aware segment splitter (reads global $cmd) ------------------------------
# Splits on ; & | and newlines at the top level; content inside '...', "...", `...`
# is never treated as a separator, so quoted data is not re-split. NUL-delimited so a
# quoted newline inside a segment survives. Readers use `read -r -d ''`.
split_segments() {
  local i ch next quote segment
  quote=; segment=
  for ((i = 0; i < ${#cmd}; i++)); do
    ch="${cmd:i:1}"; next="${cmd:i+1:1}"
    if [ -n "$quote" ]; then
      segment+="$ch"; [ "$ch" = "$quote" ] && quote=; continue
    fi
    case "$ch" in
      "'" | '"' | '`') quote="$ch"; segment+="$ch" ;;
      $'\n' | ';' | '&' | '|')
        printf '%s\0' "$segment"; segment=
        { [ "$ch" = '&' ] || [ "$ch" = '|' ]; } && [ "$next" = "$ch" ] && i=$((i + 1))
        ;;
      *) segment+="$ch" ;;
    esac
  done
  printf '%s\0' "$segment"
}

# Split a segment into shell words, honoring '...' and "..." quoting and dropping the
# quote characters (so "/" becomes /, and "rm -rf /" as an echo ARG stays an arg).
# Fills the global array WORDS. Returns 1 on an unterminated quote.
WORDS=()
shell_words() {
  local text="$1" i ch quote token in_token
  WORDS=(); quote=; token=; in_token=0
  for ((i = 0; i < ${#text}; i++)); do
    ch="${text:i:1}"
    if [ "$quote" = "'" ]; then [ "$ch" = "'" ] && quote= || token+="$ch"; in_token=1; continue; fi
    if [ "$quote" = '"' ]; then [ "$ch" = '"' ] && quote= || token+="$ch"; in_token=1; continue; fi
    case "$ch" in
      [[:space:]]) [ "$in_token" -eq 1 ] && { WORDS+=("$token"); token=; in_token=0; } ;;
      "'") quote="'"; in_token=1 ;;
      '"') quote='"'; in_token=1 ;;
      *) token+="$ch"; in_token=1 ;;
    esac
  done
  [ -z "$quote" ] || return 1
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

# --- catastrophic target test (precise; NOT "any absolute/glob/var") ---------------
# Matches only root, home-root, and top-level system directories — the paths whose
# recursive deletion is unrecoverable. A specific subdir ("$TMPDIR/x", "./dist",
# "~/.cache", "/tmp/app") is NOT catastrophic and passes.
is_catastrophic_target() {
  local w="$1" root
  while [[ "$w" = \\* ]]; do w="${w:1}"; done   # strip leading escapes
  w="${w%%\\}"                                   # strip a trailing escape artifact
  [ "$w" != "/" ] && w="${w%/}"                  # drop one trailing slash, but keep bare /
  [ -z "$w" ] && return 1
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

# find <catastrophic-start> ... -delete|-exec rm  -> deny
find_is_catastrophic() {
  shell_words "$1" || return 1
  local w in_starts=1 danger_start=0 danger_action=0 expect_exec=0
  for w in "${WORDS[@]}"; do
    if [ "$expect_exec" -eq 1 ]; then [ "${w##*/}" = "rm" ] && danger_action=1; expect_exec=0; continue; fi
    if [ "$in_starts" -eq 1 ]; then
      case "$w" in
        -H|-L|-P|-O*|-D) continue ;;
        -*|'('|'!') in_starts=0 ;;
        *) is_catastrophic_target "$w" && danger_start=1; continue ;;
      esac
    fi
    case "$w" in -delete) danger_action=1 ;; -exec|-execdir) expect_exec=1 ;; esac
  done
  [ "$danger_start" -eq 1 ] && [ "$danger_action" -eq 1 ]
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

aws_is_risky() {
  shell_words "$1" || return 1
  local w s3=0 rm=0 rb=0 rec=0 force=0 dryrun=0
  for w in "${WORDS[@]}"; do
    case "$w" in
      s3|s3api) s3=1 ;; rm) rm=1 ;; rb) rb=1 ;;
      --recursive) rec=1 ;; --force) force=1 ;; --dryrun|--dry-run) dryrun=1 ;;
    esac
  done
  [ "$dryrun" -eq 1 ] && return 1
  { [ "$s3" -eq 1 ] && [ "$rm" -eq 1 ] && [ "$rec" -eq 1 ]; } && return 0
  { [ "$s3" -eq 1 ] && [ "$rb" -eq 1 ] && [ "$force" -eq 1 ]; } && return 0
  return 1
}

# Only a real `docker [<group>] prune` op is risky — not an unrelated command that
# merely contains the word "prune" deep in its args (docker run ... npm prune -f).
docker_is_risky() {
  shell_words "$1" || return 1
  local w positional=0 is_prune=0 force=0 expect_prune=0
  for w in "${WORDS[@]}"; do
    if [[ "$w" = -* ]]; then
      case "$w" in --force|-f|--force=true) force=1 ;; -[A-Za-z]*) [[ "$w" = *f* ]] && force=1 ;; esac
      continue
    fi
    positional=$((positional + 1))
    if [ "$positional" -eq 1 ]; then
      case "$w" in
        prune) is_prune=1 ;;
        system|image|container|volume|network|builder|buildx) expect_prune=1 ;;
        *) return 1 ;;   # run / exec / compose / build / ... — not a prune op
      esac
    elif [ "$expect_prune" -eq 1 ]; then
      [ "$w" = "prune" ] && is_prune=1
      expect_prune=0
    fi
  done
  [ "$is_prune" -eq 1 ] && [ "$force" -eq 1 ]
}

tf_is_risky() {
  shell_words "$1" || return 1
  local w destroy=0 auto=0
  for w in "${WORDS[@]}"; do
    case "$w" in destroy|-destroy|--destroy) destroy=1 ;; -auto-approve|--auto-approve|-auto-approve=true) auto=1 ;; esac
  done
  [ "$destroy" -eq 1 ] && [ "$auto" -eq 1 ]
}

kubectl_is_risky() {
  shell_words "$1" || return 1
  local w del=0 all=0 dry=0
  for w in "${WORDS[@]}"; do
    case "$w" in delete) del=1 ;; --all|--all=true) all=1 ;; --dry-run|--dry-run=*) dry=1 ;; esac
  done
  [ "$dry" -eq 1 ] && return 1
  [ "$del" -eq 1 ] && [ "$all" -eq 1 ]
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

# A remote download piped into a shell/interpreter (curl … | bash). Quote-aware caller
# passes de-quoted text, so a mention inside a string doesn't trip it. This is a footgun
# worth a confirm, not an unrecoverable catastrophe -> ASK.
remote_pipe_to_shell() {
  printf '%s' "$1" | grep -Eq '(curl|wget|fetch)[^|]*\|[[:space:]]*((sudo|env|command|exec|nohup)[[:space:]]+)*([^[:space:]]*/)?(sh|bash|zsh|dash|ksh|python[0-9.]*|node|ruby|perl)([[:space:]]|$)'
}

# Remove quoted regions (keep the quotes' delimiters gone AND their content gone) so
# raw matching only sees UNQUOTED shell text.
strip_quoted() {
  local s="$1" out="" i ch quote
  quote=
  for ((i = 0; i < ${#s}; i++)); do
    ch="${s:i:1}"
    if [ -n "$quote" ]; then [ "$ch" = "$quote" ] && quote=; continue; fi
    case "$ch" in "'"|'"'|'`') quote="$ch" ;; *) out+="$ch" ;; esac
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
      find)   find_is_catastrophic "$tail"   && { REASON="find deleting from a root/home/system start path. Use a specific relative start path."; DECISION="deny"; return 0; } ;;
      chmod)  chmod_is_catastrophic "$tail"  && { REASON="chmod 777 on a root/system path."; DECISION="deny"; return 0; } ;;
      dd)     dd_is_catastrophic "$tail"     && { REASON="dd writing to a raw block device (would overwrite a disk)."; DECISION="deny"; return 0; } ;;
      mkfs|mkfs.*|wipefs|blkdiscard|shred)
        # device-gated: shred/wipefs/mkfs of a block DEVICE wipes a disk; against a
        # plain file (shred secret.txt, mkfs.ext4 disk.img) it is a normal operation.
        format_is_catastrophic "$tail" && { REASON="$name targeting a block device (would wipe a disk). Against a plain file this is allowed."; DECISION="deny"; return 0; } ;;
      git)      git_is_risky "$tail"     && ask_reason="destructive git (reset --hard / clean -f / branch -D / push --force / whole-tree checkout or restore). Uncommitted changes have no reflog; confirm it's intended, or target specific paths." ;;
      aws)      aws_is_risky "$tail"     && ask_reason="aws s3 rm --recursive / rb --force deletes bucket data. Confirm it's intended." ;;
      docker)   docker_is_risky "$tail"  && ask_reason="docker prune --force removes containers/images/volumes. Confirm it's intended." ;;
      terraform|tofu) tf_is_risky "$tail" && ask_reason="terraform/tofu destroy -auto-approve tears down infrastructure with no prompt. Confirm it's intended." ;;
      kubectl)  kubectl_is_risky "$tail" && ask_reason="kubectl delete --all removes every resource of a kind. Confirm it's intended, or target a specific resource." ;;
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
