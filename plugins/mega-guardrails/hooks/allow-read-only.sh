#!/usr/bin/env bash
# PreToolUse(Bash) hook, OPT-IN. Auto-approves a Bash command ONLY when the command
# STRING carries no write construct and names an inspection command that does not write,
# optionally behind one `cd PATH &&` prefix. It never denies and never asks. Its only
# power is to skip a prompt it can justify; everything else is passed through untouched
# and gets the normal permission flow, exactly as if this hook were not installed.
#
# That opening sentence used to read "only when it can prove the command is read-only",
# and it used to stop before the `cd PATH &&` clause. Both shortenings were false and both
# were caught by review, not by a test. Three sections below carry the evidence: WHY GIT IS
# NOT ON THE LIST, where the real binary wrote in an ordinary repository; WHAT APPROVAL
# PROVES, where the string cannot say which executable will run; and the flag tables, where
# two rounds of per-flag audit each turned up a flag that writes or execs. Read them before
# widening anything here. An overstated claim is worse than a narrow gap, because it
# invites reliance the check cannot support.
#
# WHY THIS IS A HOOK AND NOT A permissions.allow RULE.
# A `Bash(ls:*)` rule matches on the command PREFIX and cannot constrain what follows,
# so it also auto-approves `ls -la > ~/.bashrc`, `ls "$(sh -c 'touch owned')"`, and
# `ls x; rm -rf .`. There is no read-only prefix, which is why the settings template
# ships no allow rules at all and why deny-destructive's test suite locks that in. A hook
# is the only mechanism that gets to look at the WHOLE string before deciding.
#
# WHAT IT BUYS. Across 22,201 observed Bash calls the most common shape was `cd X && …`
# at 4,348 occurrences, and the transcripts carry roughly nineteen interruptions reading
# "This Bash command contains multiple operations. The following part requires approval:"
# where every named part was inspection (ls -la, git -C … status, wc -l). This removes
# the coreutils half of those. `git -C … status` is NOT removed and will not be; the
# evidence that cost it is under WHY GIT IS NOT ON THE LIST.
#
# HOW IT IS SAFE: one positive test, not a list of bypasses to chase.
# The command must consist ENTIRELY of bytes from this set:
#     A-Z a-z 0-9 space _ . / = , : + @ % -
# Nothing outside it is accepted, so every construct that could smuggle a write is
# rejected by the same line rather than by its own special case:
#   redirection > >> < <<< >| and fd duplication 2>&1   ('>' '<' '&' absent)
#   heredocs << and their bodies                         ('<' and newline absent)
#   command substitution $( ) and backticks              ('$' '(' '`' absent)
#   process substitution <( ) >( )                       ('<' '>' '(' absent)
#   variable expansion $VAR and ${VAR}                   ('$' '{' absent)
#   control operators ; && || & and newline              (';' '&' '|' newline absent)
#     one '&&' survives this row, and only one: see the cd exception below. The row is
#     true of the byte test and NOT true of the hook, which is why it says so here.
#   pipes |                                              ('|' absent)
#   globs * ? [ ] and brace expansion { }                (all absent)
#   quoting ' " that would have to be resolved           (both absent)
#   tilde expansion ~                                    ('~' absent)
# Every allowed byte is inert in a bash word: none of them starts an expansion, opens a
# quote, or separates a command. '=' is the only one with any grammar at all, and it only
# means assignment in word position 0, which must be an allowlisted command name.
#
# The single exception is `cd <path> && <command>`, matched as one anchored shape before
# anything else: exactly one leading cd, one path drawn from the byte set above minus the
# space, exactly one '&&', and then a remainder that goes through the full byte test,
# which forbids '&'. So `cd a && ls` is approvable and `cd a && ls && rm -rf .` is not,
# because its remainder still contains an '&'. That shape is handled because it is the
# most common command in the corpus, and it is handled this narrowly because a control
# operator is otherwise a flat rejection.
#
# COMMAND ALLOWLIST, each with a POSITIVE flag policy: an unknown flag is rejected rather
# than an unsafe flag being denied, so a flag added by a future coreutils release fails
# closed into the normal prompt.
#
# Audit this list one flag at a time, against the manual AND against strace, because three
# review rounds each found something already on it that reading the command NAME had
# missed: git, which writes .git/index and execs; then `file -z`, which execs; then
# `file -p`, which writes, and `stat --cached`, whose value can force a writeback. A flag
# earns its place only when its documented behavior can neither write nor exec, and
# "this command is obviously read-only" has now been wrong three rounds running.
#   ls    cannot write anything: it has no output-file flag and no way to run a program.
#   wc    counts bytes on stdin or named files.
#   stat  reports inode metadata. Its -c/--printf value is a format string, not a path.
#         --cached is REJECTED: --cached=never asks for AT_STATX_FORCE_SYNC, which the
#         statx(2) manual says may make a network filesystem write data back.
#   file  identifies a file from magic bytes. -C is REJECTED: it compiles a magic cache
#         to disk, which is a write. -m is rejected with it, since it only pairs with -C.
#         -z/--uncompress are REJECTED: they exec a decompressor named by the operand's
#         own content, so the file being inspected chooses the program.
#         -p/--preserve-date is REJECTED: it restores the access time through utimensat,
#         which writes the inode of the file being inspected.
#   head  prints the start of a file.
#   tail  prints the end of a file. -f/-F are REJECTED: follow mode never returns, and it
#         is a FLAG, which is the only thing this hook's policy reads.
# find is DELIBERATELY ABSENT even though the corpus shows it in the interrupted parts.
# Its primaries act: -delete removes files, -exec/-ok run arbitrary programs, and
# -fprint/-fls truncate a file named on the command line. A read-only `find` is a
# property of its primaries, not of the word find, so it does not belong in a list keyed
# on the command name.
#
# RUNTIME IS NOT PART OF THE PROOF. The -f rejection above is a rule about one flag, not
# a general promise that an approved command terminates, and it must not be read as one.
# This hook reads the command word and its flags; it never reads operands, so it cannot
# tell a bounded read from an unbounded one. `wc -c /dev/urandom`, `head -c 99999999
# /dev/zero` and `ls -R /` are all approved and none of them finishes in useful time.
# That is deliberate. Deciding termination from an operand means denylisting NAMES like
# /dev/*, which is trusting a name instead of proving a property, and it would reject
# `head -c 32 /dev/urandom`, which terminates and is useful. The property checked here is
# "does not write", and none of those three writes anything. A command that runs long is
# visible in the terminal and interruptible, so it costs a keystroke, not data.
#
# WHY GIT IS NOT ON THE LIST, and what removing it cost.
# git was on it: status, diff, log, show, ls-files, rev-parse. Every one of those is
# read-only in the sense a human means and none is read-only in the sense this hook
# claims. Measured on git 2.53.0 in a throwaway repository, with no attacker and no
# crafted payload:
#   `git status` and `git diff` REWRITE .git/index whenever the cached stat data is
#   stale, which is the ordinary state of a worktree somebody just edited. The file
#   changes on disk. That is a write, by the real binary, on a clean machine.
#   `git status`, `git diff` and `git ls-files` execute the program named by the
#   repository's own core.fsmonitor. `git log -p`, `git show` and `git diff` execute the
#   program named by diff.external or diff.<driver>.textconv. `git log`, `git show` and
#   `git diff` execute core.pager when stdout is a terminal.
# Rejecting the pre-subcommand `-c` flag stopped none of that, because none of it is set
# on the command line. It is set in .git/config and .gitattributes, files this hook never
# opens. Whether a git command writes is a property of the REPOSITORY, and this hook only
# ever sees the command string, so it is not a property this hook can decide. Approving
# it anyway is trusting a name in place of proving a property.
#
# `git rev-parse` survived every probe above and is still gone. Keeping it would mean
# maintaining a per-subcommand exception list against future git releases, and such a
# list fails OPEN: the flag policy rejects a flag it has not heard of, but nothing here
# can reject a git version that starts consulting fsmonitor somewhere new. Two approvals
# do not pay for a list that stops being true without saying so.
#
# THE COST IS REAL. `git -C <path> status` and `cd <path> && git status` are in the
# measured interruptions this hook was built to remove, and they prompt again now. That
# is the price of the decision string being true. A hook that approves `git status` while
# saying the command does not write is worse than one that prompts, because the sentence
# invites reliance the mechanism cannot support.
#
# WHAT APPROVAL PROVES, and what it does not. The proof is about the STRING: it carries
# no write construct, its first word after an optional `cd PATH &&` prefix is an
# allowlisted name, and its flags are on that name's list. It is NOT a proof about the
# process that will run. An `ls` earlier on PATH
# than /usr/bin, or an exported shell function named ls, passes the byte test and the
# flag test unchanged and can write anything. Both were reproduced.
#
# That gap is left open deliberately. This hook is a separate process reading a JSON
# payload; it never sees the shell that will execute, so it cannot resolve the name that
# shell will resolve, and a resolution done here would be stale before the command runs.
# More to the point, the prompt this hook replaces has the same hole: a human approving
# `ls -la` is not told which binary answers to ls either, so approving here makes nothing
# worse than the prompt it skips. A PATH entry or an exported function already carrying
# an attacker's code means the machine is already running that code and does not need
# `ls` to do it. So the answer is in the WORDS, and the decision reason below claims a
# property of the command string rather than of the running program.
#
# READ-ONLY MEANS READ, and a read can still be a read of something private. This hook
# approves `head -n 5 .env`, `wc -l /home/alice/.ssh/id_rsa`, and `ls -la /home/alice/.ssh`,
# because operands are inert bytes and it never looks at them. The permissions.deny rules
# in templates/settings.example.json do not cover that: they name the Read TOOL, not Bash,
# so those shapes never prompted because of a deny rule in the first place. What covers
# them is the sandbox's own credential path list, which blocks the open() whatever a
# permission decision said, and that is the layer to check before turning this hook on.
# What is genuinely given up is the prompt: a human who would have seen `head .env` go by
# no longer does. Leave the hook off if that prompt is the control you were relying on.
#
# OPT-IN. This hook is not registered in hooks.json, so installing the plugin does not
# turn it on and does not cost a process per Bash call. Turn it on by adding it to
# ~/.claude/settings.json (or a project .claude/settings.json):
#
#   "hooks": { "PreToolUse": [ { "matcher": "Bash", "hooks": [ { "type": "command",
#     "command": "~/.claude/plugins/megapowers/plugins/mega-guardrails/hooks/allow-read-only.sh"
#   } ] } ] }
#
# Adjust the path to wherever the marketplace is installed. Remove the entry to turn it
# off again; there is no other state.
#
# SPEED. It runs before every Bash call, so it pays for itself only if it is cheap. The
# byte test and every table lookup are bash builtins, no subprocesses. jq is spent only
# after a raw-payload prefilter shows the command could start with an allowlisted word,
# so an ordinary `npm test` or `go build` never forks it.
set -u
export LC_ALL=C LANG=C

# Read stdin with the builtin, not `$(cat)`. A command substitution forks a subshell and
# execs cat, which measured 5.0ms per call on its own: more than everything else this
# hook does put together, paid on every Bash call whether or not anything is approved.
# `-d ''` reads to EOF (JSON carries no NUL byte), IFS= keeps the payload untrimmed, and
# read returns nonzero at EOF with the data already in the variable.
input=""
IFS= read -r -d '' input || true
[ -n "$input" ] || exit 0

# Cheap bail before jq. This reads the RAW payload, so it is only ever used to give up:
# a miss means "do nothing", which is this hook's safe direction, and a hit still has to
# survive every check below. Whitespace-tolerant so a reformatted payload loses the
# optimization rather than the feature.
PREFILTER_RE='"command"[[:space:]]*:[[:space:]]*"(cd|ls|wc|stat|file|head|tail)[ "]'
[[ "$input" =~ $PREFILTER_RE ]] || exit 0

command -v jq >/dev/null 2>&1 || exit 0
cmd="$(printf '%s' "$input" | jq -r 'if (.tool_name // "Bash") == "Bash" then (.tool_input.command // empty) else empty end' 2>/dev/null || true)"
[ -n "$cmd" ] || exit 0

# A read-only inspection command is short. The cap bounds the work and costs nothing
# real: nothing this hook can approve comes close to it.
[ "${#cmd}" -le 2000 ] || exit 0

# The `cd X && Y` shape, stripped to Y before the byte test sees the '&'. The path class
# is the safe byte set minus the space, so a path carrying a quote, a '$', a glob, or a
# second '&' does not match here and falls through to a flat rejection.
CD_RE='^cd ([A-Za-z0-9_./=,:+@%-]+) && (.+)$'
[[ "$cmd" =~ $CD_RE ]] && cmd="${BASH_REMATCH[2]}"

# The one positive test. See the byte table in the header for what each rejection covers.
SAFE_RE='^[A-Za-z0-9 _./=,:+@%-]+$'
[[ "$cmd" =~ $SAFE_RE ]] || exit 0

read -r -a WORDS <<< "$cmd"
[ "${#WORDS[@]}" -ge 1 ] || exit 0

# Every character of a short-flag word must be on that command's list, which checks
# clusters (-la, -lah, -ltr) without enumerating them. "$c" is quoted inside the glob so
# a character is compared as itself and never as a pattern.
_short_ok() { # $1 = allowed characters, $2 = flag word with its leading '-' removed
  local ok="$1" w="$2" i c
  [ -n "$w" ] || return 1
  for ((i = 0; i < ${#w}; i++)); do
    c="${w:i:1}"
    [[ "$ok" == *"$c"* ]] || return 1
  done
  return 0
}

# A long flag matches by NAME, with any =value stripped first. That is safe only while no
# name on a list has a VALUE that acts, so the value is part of what each table below has
# to clear, not something this function establishes. `stat --cached` is the counterexample
# that made the distinction real: the name reads as a caching hint and the value =never
# selects AT_STATX_FORCE_SYNC, so the flag was removed rather than the mode allowlisted.
# For every name that remains, the value is a format string, an enumerated word, a count,
# or an fnmatch pattern; none of them is a path the byte set could make executable and
# none of them names a program to run.
_long_ok() { # $1 = space-delimited allowed names, $2 = flag word
  local list=" $1 " w="$2"
  w="${w%%=*}"
  [[ "$list" == *" $w "* ]]
}

# Every character below was checked against `ls --help` on GNU coreutils 9.7 and is a
# display, sort, or field-selection flag; ls has no output-file flag and no flag that runs
# a program. The enumeration is what keeps ls safe, not a restatement of a property the
# command name already had. It used to say the latter, and that framing is exactly how a
# writing flag sat on file's list for two rounds. A flag it omits falls through to the
# normal prompt. Note that -p and -u mean something harmless HERE and something else
# elsewhere: ls -p appends a slash to directories and ls -u shows the access time, while
# file -p rewrites it, so a character is only ever safe against one command's manual.
LS_SHORT='1aAbcCdFghiklmnopqrRsStuUvxXZ'
LS_LONG='--all --almost-all --author --classify --color --directory --escape --format --full-time --group-directories-first --hide --human-readable --ignore --indicator-style --inode --literal --no-group --numeric-uid-gid --quoting-style --recursive --reverse --si --size --sort --time --time-style --width'
WC_SHORT='clmwL'
WC_LONG='--bytes --chars --lines --max-line-length --total --words'
STAT_SHORT='cfLt'
# --cached is absent because one of its modes writes. `stat --cached=never` asks the kernel
# for AT_STATX_FORCE_SYNC, and statx(2) says that flag "may require that a network
# filesystem perform a data writeback to get the timestamps correct". Measured on GNU
# coreutils 9.7: statx(AT_FDCWD, "f", AT_STATX_FORCE_SYNC|AT_SYMLINK_NOFOLLOW, ...), where
# the default is AT_STATX_SYNC_AS_STAT. A long flag matches by NAME with its =value
# stripped, so keeping --cached kept every MODE including that one, and no mode of it
# appears anywhere in the corpus. stat has no short spelling of it.
STAT_LONG='--dereference --file-system --format --printf --terse'
# 'C' and 'm' are absent on purpose: `file -C -m x` compiles a magic cache to disk.
# 'z' and --uncompress are absent for a different reason, found by adversarial probing of
# this list rather than by reading the manual. On file 5.46, `file -z a.lz` and
# `file -z a.zst` exec a helper named lzip or zstd through PATH. Which helper runs is
# decided by the OPERAND's first bytes, so a file the agent merely inspected picks the
# program name. Nothing else on this allowlist execs anything, and losing one flag nobody
# in the corpus typed is cheaper than keeping the only content-directed exec on it.
# 'p' and --preserve-date are absent because -p WRITES, unconditionally, on every operand.
# file(1) restores the access time so it can "pretend that file never read them", and the
# restore is a filesystem metadata write. Measured on file 5.46 against an ext4 file:
#   utimensat(AT_FDCWD, "s.txt", [{tv_sec=1786028758, tv_nsec=0},
#                                 {tv_sec=1786028758, tv_nsec=0}], 0) = 0
# Plain `file s.txt` issues no utime call at all, so the write is the FLAG and not the
# read. It is not even a faithful restore: the call carries whole seconds, so the file's
# mtime went from .419860996 to .000000000 and its ctime moved, which means -p destroys
# sub-second mtime and cannot hide that it ran. --preserve-date was never on the long
# list, so this one character was the whole hole, and `file -bp x` rode a cluster through.
FILE_SHORT='bhikLnNrsv'
FILE_LONG='--brief --dereference --keep-going --mime --mime-encoding --mime-type --no-dereference --print0 --separator --special-files'
HEAD_SHORT='0123456789cnqvz'
HEAD_LONG='--bytes --lines --quiet --silent --verbose --zero-terminated'
# 'f' and 'F' are absent on purpose: tail -f never returns.
TAIL_SHORT='0123456789cnqvz'
TAIL_LONG='--bytes --lines --quiet --silent --verbose --zero-terminated'

# Check every dash-word against one command's policy. Words after a bare '--' are
# operands, and operands need no check: they are paths drawn from an inert byte set, and
# no command reached here can write to one. That last clause is the whole load-bearing
# part, and it was FALSE while `file -p` was allowed, because -p writes the inode of every
# operand. It is what makes leaving operands unchecked defensible, so a flag that touches
# an operand breaks this comment, not just its own table entry. Confirmed after the
# removal: `file -- -p x` issues no utime syscall, because -p past a bare '--' is a
# filename file cannot open, and the flag itself can no longer be set at all.
_flags_ok() { # $1 = short chars, $2 = long names, $@ = words to check
  local short="$1" long="$2" w
  shift 2
  for w in "$@"; do
    case "$w" in
      --) return 0 ;;
      --*) _long_ok "$long" "$w" || return 1 ;;
      -*) _short_ok "$short" "${w#-}" || return 1 ;;
    esac
  done
  return 0
}

case "${WORDS[0]}" in
  ls)   _flags_ok "$LS_SHORT"   "$LS_LONG"   "${WORDS[@]:1}" || exit 0 ;;
  wc)   _flags_ok "$WC_SHORT"   "$WC_LONG"   "${WORDS[@]:1}" || exit 0 ;;
  stat) _flags_ok "$STAT_SHORT" "$STAT_LONG" "${WORDS[@]:1}" || exit 0 ;;
  file) _flags_ok "$FILE_SHORT" "$FILE_LONG" "${WORDS[@]:1}" || exit 0 ;;
  head) _flags_ok "$HEAD_SHORT" "$HEAD_LONG" "${WORDS[@]:1}" || exit 0 ;;
  tail) _flags_ok "$TAIL_SHORT" "$TAIL_LONG" "${WORDS[@]:1}" || exit 0 ;;
  *) exit 0 ;;
esac

# Printed rather than built with jq: the reason is a fixed ASCII string this file owns,
# so the approve path costs no second process.
#
# The reason states a property of the STRING and stops there. It used to open with
# "proven read-only", which was false for every git subcommand this hook allowed and
# unprovable for the rest, since no inspection of a command string says which executable
# a name resolves to. An overstated reason is worse than a narrow gap, because it invites
# reliance the check cannot support.
#
# It also used to end the list at "or control operator", flatly, and that was false for the
# single most common thing this hook approves: `cd PATH && COMMAND` carries an '&&', and
# that shape was 4,348 of 22,201 observed calls. The exception is named here rather than
# dropped, because dropping the shape would cost the feature and softening the sentence to
# "generally" would cost the reader's ability to check it. Whoever reads this reason must
# be able to hold the string up against it and find every claim true, including this one.
printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"allowlisted inspection command with allowlisted flags, in a command string carrying no redirection, substitution, expansion, pipe, glob, or quoting, and no control operator other than the single && of an approved cd PATH && COMMAND prefix. Proven of the string only: it does not establish which executable each name resolves to."}}'
exit 0
