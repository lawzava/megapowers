#!/bin/bash
# PostToolUse(Write|Edit): format the touched file in place. Runs synchronously:
# it must finish before the tool result returns, or an async format can race a
# follow-up Edit and silently clobber it (or invalidate the model's old_string).
# It only rewrites the single file just written and is fast, so the cost is small.
f=$(jq -r '.tool_input.file_path // empty')
[ -f "$f" ] || exit 0

# Markdown only: enforce the register's no-dash-punctuation rule (see
# plugins/megapowers/skills/using-megapowers/SKILL.md, Communication) on prose, plus the
# arrow and section-sign shorthands, and straighten curly quotes. One jq pass, no new
# dependency. Mangled prose is worse than the characters, so it substitutes only where the
# replacement is unambiguous and leaves the rest for the author:
#   - Never rewrites anything but prose. Fenced blocks, indented blocks, inline code spans,
#     quotations (double-quoted spans, tracked across wrapped lines, and blockquotes), and
#     YAML frontmatter are copied through byte-identical. Fences and spans follow CommonMark
#     marker matching, so a nested fence and a multi-backtick span are read correctly.
#   - Quote characters are straightened before quotations are detected, so a curly-quoted
#     span becomes a quotation and then keeps its contents: the two rules agree instead of
#     leaving a half-converted pair. A quotation preserves its contents, not its quote marks.
#   - A line it cannot fully clear of dashes keeps every dash, and a line carrying more than
#     one dash is left whole, so a paired parenthetical (`the lead — Codex — owns git`) is
#     never half-rewritten nor dissolved into a comma list. Arrows get the same discipline.
#   - Only a single em dash, spaced on its left, between lowercase words becomes a comma:
#     an unspaced dash between letters is far more often an identifier (a link destination)
#     than punctuation. A dash against a digit, markup, or a line edge stays. Neither dash
#     rule ever substitutes an en dash: numeric ranges (`0–4`), compounds (`safety–usability`),
#     and word ranges (`monday – friday`, `monday – Friday`) are all legitimate, and a range
#     wants "to" rather than a comma or a sentence break. An en dash does still count toward
#     the lone-dash guard, so an em dash sharing its line with one is left to the author.
#
# On by default, like every other hook in this plugin: the rule was already stated in four
# places and violated anyway, so enforcement strength is the point. Set
# MEGAPOWERS_PROSE_REGISTER=off to turn the prose rewriting off; any other value, and unset,
# means on. It gates only this function, so Go and prettier formatting still run either way.
# The accepted cost is a downstream repository whose style guide wants em dash punctuation in
# prose getting it rewritten until someone finds the switch.
md_register() {
  local tmp
  [ "${MEGAPOWERS_PROSE_REGISTER:-on}" = "off" ] && return 0
  # Latency backstop. The pass is roughly linear but not free (98ms at 96KB, 231ms at 256KB,
  # 706ms at 764KB, 2.1s at 1.5MB) and it blocks the tool result, so above the threshold the
  # file is skipped whole: byte-identical beats a multi-second stall on a scraped or vendored
  # doc cache, which is what markdown this large is. The largest authored file here is 98KB.
  [ "$(wc -c < "$f" 2>/dev/null || echo 0)" -le 262144 ] || return 0
  # jq substitutes bytes that are not valid UTF-8, so a file that does not round-trip
  # through jq unchanged (a latin-1 document, say) is left alone rather than corrupted.
  jq -Rjs '.' < "$f" 2>/dev/null | cmp -s - "$f" || return 0
  tmp=$(mktemp) || return 0
  if jq -Rjs '
    # Two dashes in one segment are a paired parenthetical or appositive far more often than
    # two separate punctuation marks, and converting both dissolves the boundary into a comma
    # list ("registers — soft, directive — so" becomes an unreadable four-item list). So both
    # dash rules fire only on a lone dash; anything busier belongs to the author.
    def dashes:
      if ([match("[—–]"; "g")] | length) == 1
      then gsub("(?<=[a-z]) — (?=[A-Z])"; ". ")
           | gsub("(?<=[a-z]) — ?(?=[a-z])"; ", ")
      else . end;
    def arrows: gsub("(?<=\\S) *→ *(?=[A-Z0-9])"; " to ");
    # A digit must follow: `The § symbol` discusses the character rather than citing a
    # section, and "The section symbol" is a mangling, not a normalization. `§5c/§5f` is
    # deliberately collapsed to `section 5c/5f`, which is how the citation reads aloud.
    def sections:
      gsub("(?<=/)§ *(?=[0-9])"; "")
      | sub("^§ *(?=[0-9])"; "Section ")
      | gsub("§ *(?=[0-9])"; "section ");
    # 201c and 201d are the curly doubles, 2018 and 2019 the curly singles, 0027 the
    # straight apostrophe. Escaped, not literal: a literal apostrophe would close the shell
    # quoting, and shellcheck reads a literal curly quote in a script as a typo (SC1112). A
    # curly single is straightened whatever its role: an apostrophe and a closing single
    # quote both want that same byte, so telling the two apart decides nothing.
    def quotes:
      gsub("[\u201c\u201d]"; "\"")
      | gsub("[\u2018\u2019]"; "\u0027");
    # Apply f outside inline code spans. A span is a run of backticks closed by a run of the
    # same length (CommonMark), so ``a `` b`` and ```c``` are single spans; splitting on a
    # lone backtick would land their contents on an even index and rewrite them. An unclosed
    # run is literal text, and the search resumes at the run after it. walk emits a stream of
    # pieces rather than concatenating: `a + walk(...)` is right-nested, so it recopies the
    # tail at every span and turns a line with many spans quadratic in its own length.
    def code(f):
      . as $s
      | [match("`+"; "g")] as $r
      | ($r | length) as $rn
      | def walk($i; $pos):
          if $i >= $rn then ($s[$pos:] | f)
          else ($r[$i]) as $o
            | ($o.offset + $o.length) as $after
            | ([first(range($i + 1; $rn) | select($r[.].length == $o.length))][0]) as $j
            | if $j == null then (($s[$pos:$after] | f), walk($i + 1; $after))
              else (($r[$j].offset + $r[$j].length)) as $end
                | (($s[$pos:$o.offset] | f), $s[$o.offset:$end], walk($j + 1; $end))
              end
          end;
        [walk(0; 0)] | join("");
    # A fence marker line: its marker character, run length, and trailing text. Tracking all
    # three is what keeps a nested fence in sync; a bare toggle flips on the inner marker and
    # then reads real code as prose.
    def fmark:
      ([match("^ {0,3}(`{3,}|~{3,})(.*)$")][0]) as $m
      | if $m == null then null
        else {c: ($m.captures[0].string[0:1]), n: ($m.captures[0].string | length),
              rest: $m.captures[1].string} end;
    # Apply f to the prose of a line: outside inline code spans, outside quotations.
    def prose(f):
      code(split("\"") as $q
           | [range(0; $q | length) | if . % 2 == 0 then ($q[.] | f) else $q[.] end]
           | join("\""));
    # Each rule is gated on its own character being present at all. Walking the code spans of
    # a line costs real time and almost no line carries any of these, so the gate is what keeps
    # three passes cheaper than the two it replaces.
    def line:
      (if test("[—–]") then ((prose(dashes) | if test("[—–]") then null else . end) // .)
       else . end)
      | (if test("→") then ((prose(arrows) | if test("→") then null else . end) // .)
         else . end)
      | (if test("§") then prose(sections) else . end);
    # foreach, not reduce: an `.out += [$l]` accumulator inside a reduce rebuilds the array on
    # every line, which is superlinear (7.6s on 1.3MB) inside a synchronous hook. foreach
    # emits one line per input and keeps the carried state O(1).
    split("\n")
    | [ foreach .[] as $l ({n: 0, fence: null, fm: false, q: false, emit: null};
        ($l | fmark) as $m
        | .n += 1
        | if .n == 1 and ($l | test("^---$")) then .fm = true | .emit = $l
          elif .fm then (if $l | test("^(---|\\.\\.\\.)$") then .fm = false else . end) | .emit = $l
          # Inside a fence, the only closer is a run of the same character as the opener, at
          # least as long as the opener, with nothing after it.
          elif .fence != null then
            (if $m != null and $m.c == .fence.c and $m.n >= .fence.n
                and ($m.rest | test("^[ \t]*$")) then .fence = null else . end)
            | .q = false | .emit = $l
          # A backtick opener whose info string carries a backtick is not a fence: that is a
          # line-initial code span, and opening on it desyncs every fence after it.
          elif $m != null and ($m.c == "~" or ($m.rest | test("`") | not)) then
            .fence = {c: $m.c, n: $m.n} | .q = false | .emit = $l
          elif $l | test("^(    |\t| {0,3}>)") then .emit = $l
          elif $l == "" then .q = false | .emit = $l
          else ($l | if test("[\u201c\u201d\u2018\u2019]") then code(quotes) else . end) as $c
               | .emit = (if .q then $c else $c | line end)
               | .q = (if ($c | test("\"")) and ((($c | [match("\""; "g")] | length) % 2) == 1)
                       then (.q | not) else .q end)
          end;
        .emit) ]
    | join("\n")
  ' < "$f" > "$tmp" 2>/dev/null && [ -s "$tmp" ]; then
    # Copy into the original, not `mv` over it: `cat` keeps the inode, so the mode, the ACL,
    # any hard link, and a symlinked path all survive. That leaves one window, a kill between
    # truncate and the last write; a same-directory temp plus a mode copy would close it but
    # buys that with new failure modes (an unwritable directory, an incompletely copied mode)
    # on every single edit. Every other failure path is already guarded above.
    cmp -s "$tmp" "$f" || { cat "$tmp" > "$f"; } 2>/dev/null
  fi
  rm -f "$tmp"
}
case "$f" in
  *.go)
    if command -v goimports >/dev/null 2>&1; then goimports -w "$f"
    else gofmt -w "$f"; fi 2>/dev/null ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css|*.scss|*.md|*.yaml|*.yml)
    # Use a real prettier if one is resolvable, otherwise do nothing. Resolve it
    # ourselves and invoke it directly: never `npx`, whose node startup is ~0.6-2s even
    # when it resolves nothing, paid on every md/json/yaml edit in a non-JS project.
    prettier_bin=""
    d="$(cd "$(dirname "$f")" 2>/dev/null && pwd)"
    while [ -n "$d" ]; do
      if [ -x "$d/node_modules/.bin/prettier" ]; then prettier_bin="$d/node_modules/.bin/prettier"; break; fi
      [ "$d" = "/" ] && break
      d="$(dirname "$d")"
    done
    [ -z "$prettier_bin" ] && command -v prettier >/dev/null 2>&1 && prettier_bin="prettier"
    [ -n "$prettier_bin" ] && "$prettier_bin" --write "$f" >/dev/null 2>&1
    # After prettier, so formatting cannot undo it.
    case "$f" in *.md) md_register ;; esac ;;
esac
exit 0
