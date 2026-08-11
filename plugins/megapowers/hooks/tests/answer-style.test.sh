#!/usr/bin/env bash
# The answer contract's one mechanical rule. These tests hold it to firing on
# prose the assistant wrote and staying silent on everything else, because a
# style gate that fires wrongly is worse than the prose rule it replaces: the
# prose rule only fails to bind, a wrong gate costs a rewrite of correct text.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../answer-style.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

EM=$'\xe2\x80\x94'
EN=$'\xe2\x80\x93'

pass=0
fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }

# One assistant turn holding $1 as its text, then the hook's verdict.
verdict() {
  local text="$1" active="${2:-false}" tr="$TMP/t.jsonl"
  jq -nc --arg t "$text" '{type:"assistant",message:{content:[{type:"text",text:$t}]}}' > "$tr"
  printf '{"stop_hook_active":%s,"transcript_path":"%s"}' "$active" "$tr" \
    | bash "$HOOK" 2>/dev/null | jq -r '.decision // "allow"' 2>/dev/null
}
check() {
  local got
  got="$(verdict "$2" "${3:-false}")"
  [ -z "$got" ] && got=allow
  if [ "$1" = "$got" ]; then ok; else bad "want=$1 got=$got :: $4"; fi
}

echo "== answer style tests =="

check allow "Plain prose with no forbidden punctuation." false "clean prose allows"
check block "The gate fired ${EM} and that is the finding." false "an em dash blocks"
check block "Two options ${EN} pick one." false "an en dash blocks"
check allow "A hyphen-joined word and a range 1-2 are fine." false "hyphens are not dashes"

# Quoting is not writing. Rewriting these would corrupt the quote.
check allow "$(printf 'Here is the file:\n\n```\nconst x = 1; // set %s see docs\n```\n' "$EM")" false "a fenced block is exempt"
check allow "$(printf 'It replied:\n\n> we shipped it %s finally\n' "$EM")" false "a blockquote is exempt"
check allow "$(printf 'Output:\n\n    usage: run %s help\n' "$EM")" false "an indented code block is exempt"

# MENTION IS NOT USE, and backticking is the documented way to mention a
# character (skills/using-megapowers/SKILL.md). Blocking it made every reply
# that explained this rule unsendable, including the ones written while it was
# being built.
check allow "The rule bans \`${EM}\` and \`${EN}\` in prose." false "a backticked dash is a mention"
# THE COST OF THE BIAS, PINNED. A dash sharing a line with any inline code goes
# unflagged. That is the deliberate trade: this gate would otherwise need a
# CommonMark parser to tell a mention from a use, and every version that tried
# blocked mentions the contract permits. A missed dash is one dash; a false
# block costs a turn rewriting correct text.
check allow "The rule bans \`x\` and I also wrote ${EM} on this line." false "a dash sharing a line with code is not judged"
check allow "An odd \` backtick and a dash ${EM} after it." false "an ambiguous backtick line is not judged"
# It only exempts the LINE. A dash on a clean line still blocks, even when the
# same reply carries inline code elsewhere.
check block "$(printf 'Run \`make test\` first.\nThen the verdict %s unclear.\n' "$EM")" false "a clean line still blocks beside a code line"
# A double-backtick span is one valid span, not two empty ones with the content
# exposed between them. Matching single delimiters blocked this permitted mention.
check allow "The character is \`\`${EM}\`\` in CommonMark." false "a double-backtick span is a mention"

# A FENCE CLOSES ON ITS OWN DELIMITER. A shorter or opposite-kind marker inside
# one used to close it early and expose the rest of the quoted code.
check allow "$(printf 'File:\n\n````\n```\nnested %s fence\n```\n````\n' "$EM")" false "a shorter marker does not close a longer fence"
check allow "$(printf 'File:\n\n```\n~~~\nother kind %s inside\n~~~\n```\n' "$EM")" false "the opposite kind does not close a fence"

# Prose AROUND an exempt block is still prose.
check block "$(printf 'Summary %s the run failed.\n\n```\nclean\n```\n' "$EM")" false "prose beside a fenced block still blocks"

# An unterminated fence swallows the rest. Unjudged beats wrongly judged.
check allow "$(printf 'Reading:\n\n```\nstuff %s more\n' "$EM")" false "an unterminated fence is exempt"

# The loop guard. One corrective pass, then the turn goes through.
check allow "Still has a dash ${EM} here." true "stop_hook_active allows"

# The reason has to name the rule and quote the offending line, or the model is
# left guessing which sentence to change.
reason="$(jq -nc --arg t "The verdict ${EM} unclear." '{type:"assistant",message:{content:[{type:"text",text:$t}]}}' > "$TMP/r.jsonl"
  printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TMP/r.jsonl" | bash "$HOOK" 2>/dev/null | jq -r '.reason // ""')"
case "$reason" in
  *"em dash"*) ok ;;
  *) bad "the reason must name which dash was found" ;;
esac
case "$reason" in
  *"The verdict"*) ok ;;
  *) bad "the reason must quote the offending line" ;;
esac

# The LAST turn is what is judged. An earlier turn the model can no longer reach
# must not block forever.
{
  jq -nc --arg t "Old text ${EM} here." '{type:"assistant",message:{content:[{type:"text",text:$t}]}}'
  jq -nc --arg t "Clean now." '{type:"assistant",message:{content:[{type:"text",text:$t}]}}'
} > "$TMP/multi.jsonl"
got="$(printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TMP/multi.jsonl" | bash "$HOOK" 2>/dev/null | jq -r '.decision // "allow"' 2>/dev/null)"
[ -z "$got" ] && got=allow
if [ "$got" = "allow" ]; then ok; else bad "only the last assistant turn is judged"; fi

# Fails open on every malformed input. A style rule must not wedge a session.
for bad_payload in '' 'not json' '{}' '{"transcript_path":"/nonexistent/nope.jsonl"}' '[]'; do
  got="$(printf '%s' "$bad_payload" | bash "$HOOK" 2>/dev/null | jq -r '.decision // "allow"' 2>/dev/null)"
  [ -z "$got" ] && got=allow
  if [ "$got" = "allow" ]; then ok; else bad "malformed payload must fail open: [$bad_payload]"; fi
done

# A sidechain turn is a subagent's, not the answer to the user.
jq -nc --arg t "Subagent text ${EM} here." '{type:"assistant",isSidechain:true,message:{content:[{type:"text",text:$t}]}}' > "$TMP/side.jsonl"
got="$(printf '{"stop_hook_active":false,"transcript_path":"%s"}' "$TMP/side.jsonl" | bash "$HOOK" 2>/dev/null | jq -r '.decision // "allow"' 2>/dev/null)"
[ -z "$got" ] && got=allow
if [ "$got" = "allow" ]; then ok; else bad "a sidechain turn is not the answer to the user"; fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
