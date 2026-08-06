#!/usr/bin/env bash
# Dependency-light test for allow-read-only.sh, written as the attacker.
#
# The hook has exactly one power: emit permissionDecision "allow" and skip a prompt.
# So there are only two outcomes worth asserting, and the whole suite is the boundary
# between them:
#   ALLOW  the hook approved the command. What that establishes is a property of the
#          command STRING: it carries no write construct, its first word after an
#          optional `cd PATH &&` prefix is an allowlisted name, and every flag is on
#          that name's list. It is NOT a proof that the command is read-only, because
#          the string does not say which executable each name resolves to. This line
#          used to claim the proof, and it is worded this way now for the same reason
#          the decision reason is: an overstated contract invites reliance the check
#          cannot support, and a test suite that overstates it will pass a hook that
#          does the same.
#   PASS   the hook said nothing, and the command gets the normal permission flow.
# PASS is the safe outcome. Every smuggling attempt below must land there. The ALLOW
# fixtures are of two kinds and the sections say which: prompts the corpus says a human
# actually hit, and shapes pinned because approving them is a decision somebody argued for
# rather than an accident (a command that never terminates, an operand past a bare '--',
# a flag that survived the per-flag audit). This line used to claim all of them came from
# the corpus, which was never true of the /dev/urandom fixtures.
#
# Run: plugins/mega-guardrails/hooks/tests/allow-read-only.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../allow-read-only.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

pass=0; fail=0
decide() {
  local out dec
  out="$(jq -nc --arg c "$1" '{tool_name:"Bash",tool_input:{command:$c}}' | bash "$HOOK" 2>/dev/null)"
  if [ -z "$out" ]; then printf 'PASS'; return; fi
  dec="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.permissionDecision // "MALFORMED"' 2>/dev/null)"
  printf '%s' "$(printf '%s' "$dec" | tr 'a-z' 'A-Z')"
}
check() { # want cmd
  local got; got="$(decide "$2")"
  if [ "$got" = "$1" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%-5s got=%-5s :: %s\n' "$1" "$got" "$2"; fi
}
# The reason as the hook actually emits it, empty when nothing was approved. Separate from
# decide() because the reason is asserted as behavior, not read by eye.
reason_for() { # cmd
  jq -nc --arg c "$1" '{tool_name:"Bash",tool_input:{command:$c}}' | bash "$HOOK" 2>/dev/null |
    jq -r '.hookSpecificOutput.permissionDecisionReason // ""' 2>/dev/null
}

echo "== allow-read-only tests =="

echo "-- ALLOW: the prompts this hook exists to remove --"
# The measured interruption shapes: ls -la, git -C … status, wc -l, and the cd prefix
# that carried 4,348 of 22,201 observed Bash calls.
check ALLOW 'ls'
check ALLOW 'ls -la'
check ALLOW 'ls -lah'
check ALLOW 'ls -ltr'
check ALLOW 'ls -la /home/alice/Code'
check ALLOW 'ls --color=auto -la'
check ALLOW 'ls -la --group-directories-first'
check ALLOW 'wc -l'
check ALLOW 'wc -l README.md'
check ALLOW 'wc -lwc a.txt b.txt'
check ALLOW 'wc --lines README.md'
check ALLOW 'stat -c %s README.md'
check ALLOW 'stat --format=%n README.md'
check ALLOW 'file README.md'
check ALLOW 'file -b -i README.md'
check ALLOW 'file --mime-type README.md'
# The flags that SURVIVED the per-flag audit, pinned so a later over-correction that strips
# the list down to nothing is as visible as a widening. Each was read in the command's own
# manual and then run under strace: the only write any of them issues is write(1, ...) to
# stdout, and none issues a utime call, a creat, a rename, an unlink, or an execve past the
# one that started the process.
check ALLOW 'file -k -b README.md'
check ALLOW 'stat -L -t README.md'
check ALLOW 'stat --printf=%s README.md'
check ALLOW 'head -n 20 README.md'
check ALLOW 'head -5 README.md'
check ALLOW 'head -c 200 README.md'
check ALLOW 'tail -n 20 README.md'
check ALLOW 'tail -20 README.md'
check ALLOW 'cd /home/alice/Code/repo && ls -la'
check ALLOW 'cd plugins && wc -l README.md'
check ALLOW 'cd .. && head -n 5 README.md'
check ALLOW 'cd - && ls'
# Operands are unchecked because the byte set leaves them inert, so a flag-shaped path
# after '--' is fine and so is one that merely looks alarming.
check ALLOW 'ls -la -- /etc'
check ALLOW 'wc -l /etc/passwd'

echo "-- PASS: redirection in every form --"
# Each of these would make a read-only command WRITE. None needs its own rule: '>', '<'
# and '&' are simply not in the accepted byte set.
check PASS 'ls -la > /home/alice/.bashrc'
check PASS 'ls > out.txt'
check PASS 'ls -la >> /home/alice/.bashrc'
check PASS 'ls >| clobber.txt'
check PASS 'ls 2> err.txt'
check PASS 'ls 2>&1'
check PASS 'ls &> all.txt'
check PASS 'ls 1>&2'
check PASS 'wc -l < README.md'
check PASS 'wc -l <<< text'
check PASS 'head -n 1 f >&2'

echo "-- PASS: heredocs --"
check PASS 'wc -l <<EOF'
check PASS 'wc -l <<-EOF'
check PASS "$(printf 'wc -l <<EOF\nowned\nEOF\n')"

echo "-- PASS: command substitution in every syntax --"
check PASS 'ls $(sh -c touch)'
check PASS 'ls $(rm -r .)'
check PASS 'ls ${HOME}'
check PASS 'ls $HOME'
check PASS 'ls `id`'
check PASS 'ls "`sh -c whoami`"'
check PASS 'stat -c %s $(ls)'

echo "-- PASS: process substitution --"
check PASS 'wc -l <(rm -r .)'
check PASS 'ls >(tee owned)'

echo "-- PASS: control operators and newlines --"
check PASS 'ls ; rm -r .'
check PASS 'ls; rm -r .'
check PASS 'ls && rm -r .'
check PASS 'ls || rm -r .'
check PASS 'ls & rm -r .'
check PASS "$(printf 'ls\nrm -r .')"
check PASS "$(printf 'ls -la\nrm -rf /')"

echo "-- PASS: pipes --"
check PASS 'ls | sh'
check PASS 'wc -l | bash'
check PASS 'ls -la | tee /home/alice/.bashrc'

echo "-- PASS: globs the shell would expand into arguments nobody vetted --"
check PASS 'ls *.sh'
check PASS 'ls ?.txt'
check PASS 'ls [ab]*'
check PASS 'wc -l hooks/*.sh'
check PASS 'ls {a,b}'

echo "-- PASS: quoting that would have to be resolved --"
check PASS 'ls "my file"'
check PASS 'wc -l my\ file'
check PASS 'ls \; rm -r .'

echo "-- PASS: expansions that are not command substitution but still expand --"
check PASS 'ls ~'
check PASS 'ls ~/x'
check PASS 'ls !!'
check PASS 'ls # comment'
check PASS "$(printf 'ls\t-la')"

echo "-- PASS: the cd special case cannot be widened into a chain --"
# The remainder of `cd X && …` goes through the same byte test, which forbids '&'. So a
# second operator in the tail is a flat rejection, and this is the payload that proves it:
# every flag in it is one ls would accept, so only the '&' stands between it and approval.
check PASS 'cd a && ls && rm -r .'
check PASS 'cd a && ls ; rm -r .'
check PASS 'cd a && ls | sh'
check PASS 'cd a && rm -r .'
check PASS 'cd a && git push --force'
check PASS 'cd a && git checkout .'
check PASS 'cd $(pwd) && ls'
check PASS 'cd "a b" && ls'
check PASS 'cd ~ && ls'
check PASS 'cd a; ls'
check PASS 'cd a & ls'
check PASS 'cd a && ls > owned'
# A parent escape in the cd path is NOT a rejection here, and that is deliberate. Where a
# cd lands only matters when the tail can act on it; deny-destructive treats '..' as
# dangerous because `rm -rf ..` deletes an unnamed directory. No command-and-flag pair this
# hook approves writes anything, so `cd ../.. && ls` is a listing of somewhere else and
# stays approvable. Scoped to the pair on purpose: a PATH-shadowed `ls` writes wherever it
# likes and the cd would aim it, which is the gap under WHAT APPROVAL PROVES and not a
# claim this fixture gets to wave away.
check ALLOW 'cd a/../.. && ls'
check ALLOW 'cd ../.. && wc -l README.md'

echo "-- PASS: commands that are not on the allowlist --"
check PASS 'rm -rf /'
check PASS 'cat README.md'
check PASS 'echo hello'
check PASS 'npm test'
check PASS 'find . -name x'
check PASS 'find . -delete'
check PASS 'sh -c ls'
check PASS 'lsof -i'
check PASS 'statx foo'
check PASS 'gitk'
check PASS '/bin/ls -la'
check PASS 'ls-files'
check PASS 'sudo ls -la'
check PASS 'env ls'
check PASS 'LC_ALL=C ls'

echo "-- PASS: a flag the command's policy does not know --"
# An unknown flag is REJECTED rather than an unsafe flag being denied, so a flag added by
# a future release fails closed into the normal prompt instead of being approved.
check PASS 'ls --output=owned'
check PASS 'ls -laz'
check PASS 'wc --files0-from=list'
check PASS 'head --output=owned f'
check PASS 'stat --xyzzy f'
check PASS 'file -C -m ./magic'      # -C compiles a magic cache to disk: a write
check PASS 'file -C ./magic'
# -z picks a decompressor from the OPERAND's first bytes and execs it through PATH. On
# file 5.46, `file -z a.lz` runs lzip and `file -z a.zst` runs zstd. Every other flag on
# this allowlist execs nothing at all, so the file being inspected must not get to name a
# program. This is the one rejection found by probing the list rather than reading a man
# page, which is why it is pinned with the evidence attached.
check PASS 'file -z archive.lz'
check PASS 'file --uncompress archive.zst'
check PASS 'file -bz archive.xz'
# -p is the plainest violation this list ever carried, and it survived the round that found
# -z, one line above it in the same table. That is the argument for auditing a flag at a
# time instead of a command at a time. It WRITES, on every operand, unconditionally:
# file(1) restores the access time to "pretend that file never read them", and on file 5.46
# that restore is
#   utimensat(AT_FDCWD, "s.txt", [{tv_sec=1786028758, tv_nsec=0}, {...}], 0) = 0
# while plain `file s.txt` issues no utime call at all. So the syscall is the flag, not the
# read. It is not even faithful: whole seconds went in, so mtime fell from .419860996 to
# .000000000 and ctime moved. --preserve-date was never on the long list, which is why the
# single short character was the entire hole and why the cluster form is pinned beside it.
check PASS 'file -p README.md'
check PASS 'file -bp README.md'
check PASS 'file -pk README.md'
check PASS 'file --preserve-date README.md'
check PASS 'cd plugins && file -p README.md'
# --cached=never asks the kernel for AT_STATX_FORCE_SYNC, and statx(2) says that flag may
# require a network filesystem to write data back. Confirmed on GNU coreutils 9.7, where
# the default is AT_STATX_SYNC_AS_STAT. A long flag matches by name with its =value
# stripped, so the whole name goes rather than one mode: the hook has no way to allow
# --cached=always and reject --cached=never, and no mode of it appears in the corpus.
check PASS 'stat --cached=never README.md'
check PASS 'stat --cached=always README.md'
check PASS 'stat --cached=default README.md'
check PASS 'stat --cached never README.md'
check PASS 'cd plugins && stat --cached=never README.md'
check PASS 'tail -f app.log'         # never returns
check PASS 'tail -F app.log'
check PASS 'tail --follow app.log'
check PASS 'tail -n 5 -f app.log'

echo "-- ALLOW: an approved command may still run forever --"
# The -f rejection above is a rule about one FLAG, not a promise that approved commands
# terminate. The hook never reads operands, so these are approved and none finishes in
# useful time. They are pinned here so the behavior is a documented decision rather than
# an oversight a later reader silently "fixes": rejecting them means denylisting names
# like /dev/*, which would also reject the bounded `head -c 32 /dev/urandom`. Nothing here
# writes, and a long-running command is visible and interruptible.
check ALLOW 'wc -c /dev/urandom'
check ALLOW 'head -c 99999999 /dev/zero'
check ALLOW 'ls -R /'
check ALLOW 'head -c 32 /dev/urandom'
check ALLOW 'wc -l /dev/random'

echo "-- PASS: git is off the allowlist entirely --"
# git USED to be here with status, diff, log, show, ls-files and rev-parse approved, and
# every one of those was wrong. Measured on git 2.53.0 in a throwaway repository, with no
# attacker and nothing crafted:
#   `git status` and `git diff` rewrite .git/index when the cached stat data is stale.
#   `git status`, `git diff` and `git ls-files` run the program in core.fsmonitor.
#   `git log -p`, `git show` and `git diff` run diff.external or a textconv filter.
#   `git log`, `git show` and `git diff` run core.pager against a terminal.
# All of that is configured in .git/config and .gitattributes, which this hook never
# reads, so rejecting the pre-subcommand `-c` flag prevented none of it. Whether a git
# command writes is a property of the repository, not of the command string, and the
# string is the only thing this hook sees.
#
# These are the fixtures the hook used to APPROVE. They are here, in PASS, so that
# restoring the git arm fails the suite rather than quietly restoring a false claim.
check PASS 'git status'
check PASS 'git status -sb'
check PASS 'git status --porcelain'
check PASS 'git diff'
check PASS 'git diff --stat'
check PASS 'git diff --cached --name-only'
check PASS 'git log --oneline -5'
check PASS 'git log -n 20 --graph --decorate'
check PASS 'git log -p --name-status'
check PASS 'git show --stat HEAD'
check PASS 'git show HEAD'
check PASS 'git ls-files'
check PASS 'git ls-files -m --others --exclude-standard'
check PASS 'git --no-pager log --oneline'
check PASS 'git -C /home/alice/Code/repo status'
check PASS 'git -C /home/alice/Code/repo log --oneline -5'
# The cost, stated as a test rather than left in prose: these two are in the measured
# interruptions this hook was built to remove, and they prompt again. A later reader who
# wants them back has to answer the index write first.
check PASS 'cd /home/alice/Code/repo && git status'
check PASS 'cd .. && git log --oneline -5'
# rev-parse survived every write and exec probe and is still gone, because keeping it
# means a per-subcommand exception list that fails OPEN against a future git release.
check PASS 'git rev-parse --show-toplevel'
check PASS 'git rev-parse --abbrev-ref HEAD'
# Shapes that were already rejected. They stay pinned so the removal is not credited with
# work the byte test and the flag policy were doing.
check PASS 'git diff --output=owned'
check PASS 'git log -p --output=owned'
check PASS 'git show --output=owned HEAD'
check PASS 'git diff --ext-diff'
check PASS 'git log --textconv'
check PASS 'git show --show-signature HEAD'
check PASS 'git -c core.pager=sh log'
check PASS 'git -c diff.external=sh diff'
check PASS 'git --exec-path=/tmp/evil status'
check PASS 'git --git-dir=/tmp/evil status'
check PASS 'git --work-tree=/ status'
check PASS 'git --git-dir /tmp/evil status'
check PASS 'git checkout .'
check PASS 'git push --force origin main'
check PASS 'git clean -fd'
check PASS 'git reset --hard'
check PASS 'git commit -am wip'
check PASS 'git rm -r src'
check PASS 'git stash'
check PASS 'git status -x'
check PASS 'git'
check PASS 'git -C'
check PASS 'git -C /repo'
check PASS 'git -C /tmp -c core.pager=sh log'
check PASS 'git -C -c status'
check PASS 'git -C --git-dir=/evil status'
check PASS 'git status -- --output=owned'

echo "-- ALLOW: after a bare -- every word is an operand --"
# Operands need no check because no command reached here can write to one, so a
# flag-shaped operand is just a path: `tail -- -f` opens a file named '-f' and returns.
check ALLOW 'tail -- -f'
check ALLOW 'ls -- --output=owned'

echo "-- PASS: oversized input --"
long="ls -la"
while [ "${#long}" -lt 2100 ]; do long="$long /some/path/component"; done
check PASS "$long"

echo "== the hook never denies and never asks =="
# Its whole contract is that it can only skip a prompt. Replay every fixture in this file
# and assert that the decision is either absent or exactly "allow": a "deny" or "ask"
# would mean the hook had grown the power to block work, which is a different tool.
replayed=0
while IFS= read -r c; do
  [ -n "$c" ] || continue
  replayed=$((replayed + 1))
  got="$(decide "$c")"
  case "$got" in
    PASS|ALLOW) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL hook emitted %s (it may only allow) :: %s\n' "$got" "$c" ;;
  esac
done < <(sed -n "s/^check \(ALLOW\|PASS\) '\([^']*\)'.*/\2/p" "${BASH_SOURCE[0]}")
if [ "$replayed" -eq 0 ]; then fail=$((fail + 1)); echo "  FAIL no fixtures found to replay"; fi

echo "== payload handling =="
# A non-Bash tool that happens to carry a .tool_input.command must not be approved: this
# hook reasons about shell syntax and nothing else runs that grammar.
out="$(jq -nc '{tool_name:"Read",tool_input:{command:"ls -la"}}' | bash "$HOOK" 2>/dev/null)"
if [ -z "$out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "  FAIL approved a non-Bash tool call"; fi
# The pre-jq prefilter reads the raw payload, so a differently formatted one must still
# work. A miss here would silently turn the whole feature off rather than break loudly.
out="$(printf '{"tool_name": "Bash", "tool_input": { "command" : "ls -la" }}' | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | jq -e '.hookSpecificOutput.permissionDecision == "allow"' >/dev/null 2>&1; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL whitespace-formatted payload lost the approval"
fi
# Empty and junk stdin must be silent, not a crash and not an approval.
for junk in '' 'not json' '{}' '{"tool_input":{}}'; do
  out="$(printf '%s' "$junk" | bash "$HOOK" 2>/dev/null)"
  if [ -z "$out" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL non-silent on junk payload :: %s\n' "$junk"; fi
done
# The emitted object must be the shape Claude Code reads, or the approval is a no-op that
# looks like it works.
out="$(jq -nc '{tool_name:"Bash",tool_input:{command:"ls -la"}}' | bash "$HOOK" 2>/dev/null)"
if printf '%s' "$out" | jq -e '.hookSpecificOutput.hookEventName == "PreToolUse" and .hookSpecificOutput.permissionDecision == "allow" and (.hookSpecificOutput.permissionDecisionReason | length > 0)' >/dev/null 2>&1; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL emitted object is not a PreToolUse permission decision"
fi

echo "== the decision reason claims only what the check proves =="
# The reason is the only sentence a user reads about why a prompt was skipped, so it is
# asserted as behavior. It used to open with "proven read-only", which was false for every
# git subcommand the hook allowed and unprovable for the rest: no inspection of a command
# STRING says which executable a name resolves to, and a PATH-shadowed `ls` or an exported
# shell function named ls passes the byte test and the flag test unchanged. Both were
# reproduced. That gap is not closed in code, because the hook is a separate process that
# never sees the shell that will run, and because the prompt it replaces has the identical
# hole: nobody clicking approve on `ls -la` is told which binary answers. It is closed in
# the wording, and this test is what keeps the wording honest.
reason="$(jq -nc '{tool_name:"Bash",tool_input:{command:"ls -la"}}' | bash "$HOOK" 2>/dev/null | jq -r '.hookSpecificOutput.permissionDecisionReason // ""')"
case "$reason" in
  *read-only*) fail=$((fail + 1)); printf '  FAIL the reason still claims read-only :: %s\n' "$reason" ;;
  *) pass=$((pass + 1)) ;;
esac
case "$reason" in
  *executable*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL the reason does not disclaim executable resolution :: %s\n' "$reason" ;;
esac
# This suite's own header defines what an ALLOW means, and it used to define it as "the
# hook proved the command read-only": the same overstatement, in the file whose job is to
# catch it. A contract asserted in one place and denied in another is how the first one came
# back, so the header is checked too. It is a text test on text, which is all it claims to
# be, and it reads only the header block so the pattern below cannot match itself.
head_contract="$(sed -n '1,19p' "${BASH_SOURCE[0]}")"
if printf '%s' "$head_contract" | grep -qiE 'prove[sd]? the command read-only'; then
  fail=$((fail + 1)); echo "  FAIL this suite's header still claims an ALLOW proves the command read-only"
else
  pass=$((pass + 1))
fi
if printf '%s' "$head_contract" | grep -qF 'STRING' && printf '%s' "$head_contract" | grep -qF 'executable'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL this suite's header no longer scopes an ALLOW to the command string"
fi

echo "== the reason is not contradicted by the string it describes =="
# The second overstatement this hook shipped, and the reason it is now asserted mechanically
# instead of by eye. The list of absent constructs used to end at "or control operator",
# flatly, while the hook approves `cd PATH && COMMAND`, which contains '&&' and was 4,348 of
# 22,201 observed calls. Same failure as "proven read-only": a sentence the user can hold up
# against the string and find false. Reverting the wording turns this red.
cd_reason="$(reason_for 'cd /home/alice/Code/repo && ls -la')"
case "$cd_reason" in
  *'&&'*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL the reason denies control operators while approving && :: %s\n' "$cd_reason" ;;
esac
# Then the same check against every fixture this suite expects ALLOW, in both directions.
# Forward: an approved string carrying '&&' needs a reason that admits it. Backward: after
# one leading `cd PATH &&` is stripped, an approved string must carry NO operator at all,
# so the sentence stays true because the hook keeps it true rather than by being vague.
while IFS= read -r c; do
  [ -n "$c" ] || continue
  r="$(reason_for "$c")"
  [ -n "$r" ] || continue
  if [[ "$c" == *'&&'* ]] && [[ "$r" != *'&&'* ]]; then
    fail=$((fail + 1)); printf '  FAIL approved && without the reason admitting it :: %s\n' "$c"; continue
  fi
  rest="$c"
  [[ "$rest" =~ ^cd\ [^\ ]+\ \&\&\ (.*)$ ]] && rest="${BASH_REMATCH[1]}"
  case "$rest" in
    *';'*|*'|'*|*'&'*|*$'\n'*)
      fail=$((fail + 1)); printf '  FAIL approved an operator the reason says is absent :: %s\n' "$c" ;;
    *) pass=$((pass + 1)) ;;
  esac
done < <(sed -n "s/^check ALLOW '\([^']*\)'.*/\1/p" "${BASH_SOURCE[0]}")

# The hook has to keep the reasoning, or the next reader "fixes" a rejection back onto the
# list. The grep is on the MECHANISMS rather than on the section titles, because a title
# survives a deletion that takes the evidence with it. Each token is the syscall or the
# config key that cost one entry its place: the first three are why git is off the list,
# and the last two are what the flag tables gave up in later rounds, the utimensat that
# `file -p` issues and the forced attribute sync that `stat --cached=never` requests.
for token in '.git/index' 'core.fsmonitor' 'diff.external' 'PATH' 'utimensat' 'AT_STATX_FORCE_SYNC'; do
  if grep -qF "$token" "$HOOK"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL the hook no longer names %s as evidence for a rejection\n' "$token"
  fi
done

echo "== no git command is approvable =="
# A loop rather than fixtures alone: every subcommand that was ever on the list, crossed
# with the four spellings that used to reach it, including the `cd X && ...` prefix that
# strips itself before the byte test. A partial restore that brings back only some
# subcommands fails here without anyone having to remember to add a fixture for it.
for sub in status diff log show ls-files rev-parse; do
  for form in "git $sub" "git -C /repo $sub" "git --no-pager $sub" "cd /repo && git $sub"; do
    got="$(decide "$form")"
    if [ "$got" = "PASS" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=PASS got=%-5s :: %s\n' "$got" "$form"; fi
  done
done

echo "== the hook is opt-in =="
# Registering it by default would auto-approve on every installation without anyone
# choosing that, and would cost a process per Bash call for users who never wanted it.
# The opt-in is a hook entry the user adds to their own settings.json; the plugin manifest
# must stay clear of it.
HOOKS_JSON="$HERE/../hooks.json"
if [ ! -f "$HOOKS_JSON" ]; then
  fail=$((fail + 1)); printf '  FAIL %s not found\n' "$HOOKS_JSON"
else
  if jq -e '[.hooks[]?[]?.hooks[]?.command] | join(" ") | contains("allow-read-only") | not' "$HOOKS_JSON" >/dev/null 2>&1; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    printf '  FAIL %s registers allow-read-only.sh; it must ship unregistered.\n' "$HOOKS_JSON"
    printf '       Auto-approval is a decision the user makes, not one an install makes\n'
    printf '       for them, and an unwanted hook still costs a process on every Bash call.\n'
  fi
  # The guard that actually blocks things must still be registered next to it, or this
  # test would pass on a manifest that lost both.
  if jq -e '[.hooks[]?[]?.hooks[]?.command] | join(" ") | contains("deny-destructive")' "$HOOKS_JSON" >/dev/null 2>&1; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL %s no longer registers deny-destructive.sh\n' "$HOOKS_JSON"
  fi
fi
# The header has to carry the opt-in instructions, since an unregistered hook is
# undiscoverable otherwise.
if grep -q 'PreToolUse' "$HOOK" && grep -q 'settings.json' "$HOOK"; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); echo "  FAIL the hook header does not document how to turn it on"
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
