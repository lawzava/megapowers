#!/usr/bin/env bash
# Stop hook: the answer contract's one MECHANICAL rule, enforced instead of
# restated.
#
# The rule "no em or en dashes" has shipped in templates/CLAUDE.md since
# 2026-07-07 and did not move the number. Measured over 107 Claude sessions and
# 161 Codex sessions from 2026-08-08 to 2026-08-11, restricted to multi-turn
# interactive sessions so task mix cannot explain it:
#
#   Claude  24.8% of assistant turns carried a dash  (305 of 1231)
#   Codex    0.1%                                    (1 of 907)
#
# Same rule, same wording, two orders of magnitude apart. The variable is the
# enforcement mechanism, not the prose, which is the same conclusion the
# skill-router audit reached about process skills. So this is a gate, and it is
# the only style rule that gets one: a dash is decidable by looking, and the
# rest of the contract (four lines, no preamble, lead with the conclusion) is
# not. A gate that needs judgment is a gate that fires wrongly.
#
# WHAT IT DOES NOT TOUCH. Quoting is not writing. A dash inside a fenced code
# block, an indented code block, or a blockquote came from a file, a command, or
# somebody else's words, and rewriting those would corrupt the quote. Only prose
# the assistant wrote in its own voice is held to the rule.
#
# Fails OPEN everywhere: no jq, no transcript, an unreadable or malformed
# payload, a transcript with no assistant turn. A style rule must never be able
# to wedge a session.
set -u

command -v jq >/dev/null 2>&1 || exit 0

payload="$(cat)"
[ -n "$payload" ] || exit 0

# One re-entry and no more. A Stop hook that blocks is re-invoked after the
# model answers again, and a rule that cannot be satisfied would loop forever.
# The model gets exactly one corrective pass; if the second answer still carries
# a dash, the turn is allowed through rather than held hostage to a style rule.
case "$(printf '%s' "$payload" | jq -r 'if type == "object" then (.stop_hook_active // false) else true end' 2>/dev/null)" in
  true) exit 0 ;;
esac

transcript="$(printf '%s' "$payload" | jq -r 'if type == "object" then (.transcript_path // "") else "" end' 2>/dev/null)"
[ -n "$transcript" ] || exit 0
[ -f "$transcript" ] || exit 0
[ -r "$transcript" ] || exit 0

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib-json.sh
. "$here/lib-json.sh"
# shellcheck source=lib-toml.sh
. "$here/lib-toml.sh"

# The off switch every other rule gets. Layers read project, then user, then
# shipped, first hit wins. Anything that is not this consumer's own state is
# off, so a typo disables the rule rather than half-enabling it.
rule_state=""
for _layer in ".megapowers/enforcement.toml" \
              "${XDG_CONFIG_HOME:-$HOME/.config}/megapowers/enforcement.toml" \
              "$here/../enforcement.toml"; do
  [ -f "$_layer" ] || continue
  rule_state="$(toml_scalar_in "$_layer" "rules.answer-style" state 2>/dev/null)"
  [ -n "$rule_state" ] && break
done
case "$rule_state" in
  ""|enforced) : ;;
  *) exit 0 ;;
esac

# The LAST assistant turn only. Earlier ones were already answered for, and a
# transcript that grows all session would otherwise re-block on text the model
# has no way to reach any more.
#
# Text blocks only: thinking is not shown to the user and tool input is not
# prose. `-s` would slurp a session-long transcript into memory, so the file is
# streamed and the last match wins.
#
# `jq -c` renders each turn as ONE line, a JSON string with its newlines
# escaped, so `tail -n 1` picks the last turn rather than the last line of some
# earlier turn's text. Decoding happens after the pick. Doing it the other way
# round, decoding first and then trimming by bytes, judged the whole session at
# once and re-blocked on a turn the model could no longer reach.
last="$(jq -c 'select(type == "object")
  | select(.type == "assistant")
  | select((.isSidechain // false) | not)
  | [ (.message.content // [])[]? | select(type == "object") | select(.type == "text") | (.text // "") ]
  | join("\n")
  | select(length > 0)' "$transcript" 2>/dev/null \
  | tail -n 1 | jq -r '.' 2>/dev/null | tail -c 60000)"
[ -n "$last" ] || exit 0

# Strip what the assistant is quoting rather than writing, then look. Order
# matters: fences are closed before anything inside them is judged.
prose="$(printf '%s\n' "$last" | awk '
  # A FENCE REMEMBERS ITS DELIMITER. Toggling on any run of backticks or tildes
  # let a shorter or opposite-kind marker INSIDE a fence close it early and hand
  # the rest of somebody else s quoted code to the gate. CommonMark closes a
  # fence only on the same character, at least as long as the opener, so that is
  # what is tracked. An unterminated fence still swallows the rest, which is the
  # safe direction: unjudged beats wrongly judged.
  {
    if (match($0, /^[[:space:]]*(`+|~+)/)) {
      run = $0
      sub(/^[[:space:]]*/, "", run)
      sub(/[^`~].*$/, "", run)
      if (length(run) >= 3) {
        if (!fence) { fence = 1; fchar = substr(run, 1, 1); flen = length(run); next }
        if (substr(run, 1, 1) == fchar && length(run) >= flen) { fence = 0; next }
      }
    }
  }
  fence { next }
  # Blockquote: somebody else s words, carried verbatim.
  /^[[:space:]]*>/ { next }
  # An indented code block, the four-space form.
  /^    / { next }
  # A LINE HOLDING A BACKTICK IS NOT JUDGED. This began as a regex for code
  # spans and grew delimiter-run matching, and review kept finding CommonMark
  # cases it still got wrong: unequal runs, an embedded delimiter, trailing
  # content on a fence marker. Each fix turned a style hook a little more into a
  # Markdown parser, and every miss blocked a mention the contract explicitly
  # permits (skills/using-megapowers/SKILL.md says to backtick a character you
  # discuss, precisely because tooling cannot tell use from mention).
  #
  # The bias here is the OPPOSITE of the risky-logic gate, deliberately. There a
  # missed line is a security hole and a false fire is an explainable nuisance.
  # Here a missed dash is one dash, and a false block costs the user a turn
  # rewriting correct text. So ambiguity allows.
  #
  # Costs: a dash sharing a line with any inline code goes unflagged. Buys: this
  # cannot block a permitted mention, and it needs no parser.
  /`/ { next }
  { print }
')"

# U+2014 EM DASH, U+2013 EN DASH. Spelled as bytes so this file can be read and
# edited by tooling that mangles the characters themselves, and so the rule can
# be stated in a file that must not contain what it forbids.
em=$'\xe2\x80\x94'
en=$'\xe2\x80\x93'
found=""
case "$prose" in
  *"$em"*) found="an em dash" ;;
esac
case "$prose" in
  *"$en"*) [ -z "$found" ] && found="an en dash" || found="$found and an en dash" ;;
esac
[ -n "$found" ] || exit 0

# NAME THE OFFENDING TEXT, for the same reason the risky-logic gate now names
# its keyword: a block that describes a category and not an instance leaves the
# reader guessing at which sentence to change.
sample="$(printf '%s\n' "$prose" | grep -m1 -F -e "$em" -e "$en" 2>/dev/null | cut -c1-160)"

msg="Answer contract: the reply you just wrote contains $found. The rule is in your project instructions and it is not stylistic taste: dashes are the single mechanical marker of the register this user does not want. Rewrite that reply now, replacing each one with a period, comma, colon, parentheses, or a new sentence. Change nothing else, and do not mention this correction in the rewrite."
[ -z "$sample" ] || msg="$msg The first one is in: $sample"

printf '{"decision":"block","reason":"%s"}\n' "$(escape_for_json "$msg")"
exit 0
