#!/usr/bin/env bash
# Regressions for auto-format.sh.
#
# 1. no_npx: it must not spawn npx (node startup is ~0.6-2s even when it resolves
#    nothing) on a doc/config edit when no prettier is installed. The guarded hook walks
#    up from the edited file for node_modules/.bin/prettier, else command -v prettier,
#    and only then invokes prettier directly. We put a fake `npx` earlier on PATH that
#    writes a sentinel; after the hook runs on a .md file with no prettier up-tree, the
#    sentinel must NOT exist and the hook must still exit 0.
#
# 2. normalizes_markdown_punctuation: the house register forbids dash punctuation
#    (plugins/megapowers/skills/using-megapowers/SKILL.md, Communication) and curly quotes.
#    Markdown gets normalized after prettier, only where the replacement is unambiguous. The
#    fake prettier appends a line carrying an em dash, so a normalized appended line also
#    proves the ordering. Quote characters are straightened before quotations are detected,
#    so a curly-quoted span then protects its own contents from the dash rules.
#
# 3. leaves_code_fences_untouched, commonmark_markers, and only_markdown: rewriting code is a
#    correctness bug, not a style fix. Fenced blocks, indented blocks, inline spans, and
#    non-markdown files must survive byte-identical. Markers follow CommonMark, so a nested
#    fence and a multi-backtick span do not desync the parse and expose code as prose.
#
# 4. register_switch: the register normalization ships on and MEGAPOWERS_PROSE_REGISTER=off
#    turns off that step alone, leaving Go and prettier formatting alone. Every on-state
#    assertion in this file runs under `env -u MEGAPOWERS_PROSE_REGISTER`, so an ambient value
#    in the caller's environment cannot make the suite pass by doing nothing.
#
# The fixtures and the needles that match them carry literal curly quotes on purpose: they
# are the input under test, not a mistyped shell quote.
# shellcheck disable=SC1112
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../auto-format.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
pass=0; fail=0

# $1 test name, $2 needle, $3 file
has() {
  if grep -qF -- "$2" "$3"; then pass=$((pass + 1))
  else fail=$((fail + 1)); printf '  FAIL %s: expected to find %s\n' "$1" "$2"; fi
}
lacks() {
  if grep -qF -- "$2" "$3"; then fail=$((fail + 1)); printf '  FAIL %s: expected to lose %s\n' "$1" "$2"
  else pass=$((pass + 1)); fi
}
# $1 file, $2 bin dir to put first on PATH. MEGAPOWERS_PROSE_REGISTER is explicitly unset,
# never merely assumed unset: an ambient `off` in the caller's environment would otherwise
# make every normalization assertion below pass vacuously.
run_hook() {
  printf '{"tool_input":{"file_path":"%s"}}' "$1" \
    | env -u MEGAPOWERS_PROSE_REGISTER PATH="$2:$PATH" bash "$HOOK" >/dev/null 2>&1
}

# --- 1. no_npx -----------------------------------------------------------------
mkdir -p "$work/nobin"
sentinel="$work/npx-was-called"
{
  printf '#!/usr/bin/env bash\n'
  printf 'touch "%s"\n' "$sentinel"
  printf 'exit 0\n'
} > "$work/nobin/npx"
chmod +x "$work/nobin/npx"

md="$work/doc.md"
printf '# title\n\ntext\n' > "$md"
rc=0
run_hook "$md" "$work/nobin" || rc=$?

if [ ! -e "$sentinel" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL no_npx: npx was spawned (sentinel exists)\n'; fi
if [ "$rc" -eq 0 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL no_npx: hook exit code %s, want 0\n' "$rc"; fi

# --- fake prettier: a no-op formatter that leaves a dash for the normalizer ----
mkdir -p "$work/fmtbin"
{
  printf '#!/usr/bin/env bash\n'
  printf 'for a in "$@"; do [ -f "$a" ] && printf "postfmt — done.\\n" >> "$a"; done\n'
  printf 'exit 0\n'
} > "$work/fmtbin/prettier"
chmod +x "$work/fmtbin/prettier"

# --- 2. normalizes_markdown_punctuation ---------------------------------------
reg="$work/register.md"
cat > "$reg" <<'EOF'
# title

The hook is fast — it formats one file.

The gate is mechanical — Prettier runs first.

Composite is 0–4 across the runs, and 5a–5e held.

The window covers 1914—1918 in the table.

See §3 for the ladder and RED → GREEN for the loop.

The §5c/§5f numbers hold.

The probe prompt was "urgent — keep it quick" verbatim.

The journal said "done — added with
leftpad — adapter" and moved on.

The rule says “no dash punctuation” in prose.

The probe was “urgent — keep it quick” verbatim.

The note said “done — added with
leftpad — shim” and moved on.

The lead ‘owns’ git and it’s Anthropic’s call, so don’t bypass.

Use `TaskGroup` (3.11+) — not a sequential `await` loop — when they differ, per §8.

> command = git push origin —delete branch

#### The safety–usability trade-off at R1.

Ship it – then measure.

The window is monday – friday for the run.

The window is monday – Friday for the run.

See [notes](./my—notes.md) and ![alt](./a—b.png) here.

Auto <https://x.test/a—b> stays.

[ref]: ./x—y.md

Written in registers — soft, directive, emphatic — so the test can measure.

The § symbol is discussed here.

The flow is RED → GREEN and then green → refactor here.
EOF
run_hook "$reg" "$work/fmtbin"

# An accepted tradeoff, not a target: the comma leaves a splice ("fast, it formats"). The
# house register tolerates fragments, and a comma is still on-register where the dash is not.
has normalizes_markdown_punctuation 'fast, it formats one file.' "$reg"
has normalizes_markdown_punctuation 'mechanical. Prettier runs first.' "$reg"
has normalizes_markdown_punctuation '0–4 across the runs, and 5a–5e held.' "$reg"
has normalizes_markdown_punctuation '1914—1918 in the table.' "$reg"
has normalizes_markdown_punctuation 'See section 3 for the ladder and RED to GREEN' "$reg"
has normalizes_markdown_punctuation 'The section 5c/5f numbers hold.' "$reg"
has normalizes_markdown_punctuation 'was "urgent — keep it quick" verbatim.' "$reg"
has normalizes_markdown_punctuation 'said "done — added with' "$reg"
has normalizes_markdown_punctuation 'leftpad — adapter" and moved on.' "$reg"
# A line it cannot fully clear of dashes keeps every dash (a half-closed parenthetical is
# worse than the original), but the section sign on that line still normalizes.
has normalizes_markdown_punctuation 'Use `TaskGroup` (3.11+) — not a sequential `await` loop — when they differ, per section 8.' "$reg"
has normalizes_markdown_punctuation '> command = git push origin —delete branch' "$reg"
# An unspaced en dash between words is a compound (safety-usability), not punctuation.
has normalizes_markdown_punctuation '#### The safety–usability trade-off at R1.' "$reg"
# En dashes are never rewritten. A spaced en dash is as often a word range as punctuation, and
# a range wants "to": `monday, friday` inverts the meaning, and `monday. Friday` leaves two
# fragments. The capitalized form is how a date range is usually written, so it is covered too.
has normalizes_markdown_punctuation 'Ship it – then measure.' "$reg"
has normalizes_markdown_punctuation 'The window is monday – friday for the run.' "$reg"
has normalizes_markdown_punctuation 'The window is monday – Friday for the run.' "$reg"
# An unspaced em dash between letters is an identifier far more often than punctuation, so the
# comma rule requires a space on the left. Without that, every link destination breaks.
has normalizes_markdown_punctuation 'See [notes](./my—notes.md) and ![alt](./a—b.png) here.' "$reg"
has normalizes_markdown_punctuation 'Auto <https://x.test/a—b> stays.' "$reg"
has normalizes_markdown_punctuation '[ref]: ./x—y.md' "$reg"
# A fully converted dash pair dissolves an appositive into a comma list, which changes the
# meaning. Both dash rules fire only on a lone dash; a busier line goes back to the author.
has normalizes_markdown_punctuation 'Written in registers — soft, directive, emphatic — so the test can measure.' "$reg"
# Discussing the character, not citing a section: a digit must follow.
has normalizes_markdown_punctuation 'The § symbol is discussed here.' "$reg"
# Arrows get the dash discipline: one arrow on the line cannot convert, so neither does.
has normalizes_markdown_punctuation 'RED → GREEN and then green → refactor here.' "$reg"
lacks normalizes_markdown_punctuation '§3' "$reg"
lacks normalizes_markdown_punctuation '§5c' "$reg"
lacks normalizes_markdown_punctuation '§8' "$reg"
has normalizes_markdown_punctuation 'postfmt, done.' "$reg"

# Curly quotes are off-register too. The quote characters are straightened first, so a
# curly-quoted span then counts as a quotation and its contents keep their dashes: the
# two rules agree instead of leaving a half-converted pair.
has normalizes_markdown_punctuation 'The rule says "no dash punctuation" in prose.' "$reg"
has normalizes_markdown_punctuation 'The probe was "urgent — keep it quick" verbatim.' "$reg"
has normalizes_markdown_punctuation 'The note said "done — added with' "$reg"
has normalizes_markdown_punctuation 'leftpad — shim" and moved on.' "$reg"
# A curly single is straightened whatever its role: an apostrophe and a closing single
# quote both want the same ASCII byte, so the distinction has nothing to decide.
has normalizes_markdown_punctuation "The lead 'owns' git and it's Anthropic's call, so don't bypass." "$reg"
lacks normalizes_markdown_punctuation '“' "$reg"
lacks normalizes_markdown_punctuation '”' "$reg"
lacks normalizes_markdown_punctuation '‘' "$reg"
lacks normalizes_markdown_punctuation '’' "$reg"

# A second pass must be a byte-identical no-op: the hook fires on every edit, so a
# normalizer that keeps nudging the file (a trailing newline, say) corrupts it slowly.
idem="$work/idempotent.md"
printf '# title\n\nThe hook is fast — it formats one file.\n\nThe rule says “no dashes” and it’s fine.\n' > "$idem"
run_hook "$idem" "$work/nobin"
cp "$idem" "$work/idempotent.once"
run_hook "$idem" "$work/nobin"
if cmp -s "$idem" "$work/idempotent.once"; then pass=$((pass + 1))
else fail=$((fail + 1)); printf '  FAIL normalizes_markdown_punctuation: second pass changed the file\n'; diff -u "$work/idempotent.once" "$idem" | sed -n '3,10p'; fi

# --- 3. leaves_code_fences_untouched ------------------------------------------
code="$work/code.md"
cat > "$code" <<'EOF'
Prose has an em dash — it goes away.

Prose has a curly “quote” and it’s fine.

```go
// map key — value, arrow → here, section §4
// smart quotes are code here: “a” and it’s ‘b’
m := map[string]string{"a—b": "c"}
```

An indented block:

    x — y → z §5
    q “a” ‘b’ z

An inline `a — b → c §6` span stays.

An inline `q “a” ‘b’ c` span stays.
EOF
run_hook "$code" "$work/fmtbin"

has leaves_code_fences_untouched '// map key — value, arrow → here, section §4' "$code"
has leaves_code_fences_untouched '// smart quotes are code here: “a” and it’s ‘b’' "$code"
has leaves_code_fences_untouched 'm := map[string]string{"a—b": "c"}' "$code"
has leaves_code_fences_untouched '    x — y → z §5' "$code"
has leaves_code_fences_untouched '    q “a” ‘b’ z' "$code"
has leaves_code_fences_untouched 'An inline `a — b → c §6` span stays.' "$code"
has leaves_code_fences_untouched 'An inline `q “a” ‘b’ c` span stays.' "$code"
has leaves_code_fences_untouched 'dash, it goes away.' "$code"
has leaves_code_fences_untouched 'Prose has a curly "quote" and it'"'"'s fine.' "$code"

# --- 3a. commonmark_markers ----------------------------------------------------
# A fence is a marker character and a run length, not a toggle: a bare toggle flips on the
# inner marker of a nested fence and then reads the rest of the document inside out, rewriting
# real code. Same for an inline span, whose closer is a backtick run of the *same length*.
nest="$work/nested.md"
cat > "$nest" <<'EOF'
````md
```
y — z
```
````

```markdown
~~~
code — here
~~~
```

```code``` is inline, then a real fence:

```go
x — y
```

Span ``a — b`` here and ```c — d``` too.

Prose tail — converts.
EOF
run_hook "$nest" "$work/fmtbin"

# Inner fence of a 4-backtick block, and a tilde fence inside a backtick block.
has commonmark_markers 'y — z' "$nest"
has commonmark_markers 'code — here' "$nest"
# A backtick opener whose info string carries a backtick is a code span, not a fence, so the
# real fence after it still opens rather than closing a phantom one.
has commonmark_markers '```code``` is inline, then a real fence:' "$nest"
has commonmark_markers 'x — y' "$nest"
# Multi-backtick spans: the standard CommonMark way to show a literal backtick.
has commonmark_markers 'Span ``a — b`` here and ```c — d``` too.' "$nest"
# The one prose line outside all of it still normalizes, so this is not passing by doing nothing.
has commonmark_markers 'Prose tail, converts.' "$nest"

# --- 3b. byte_safety ----------------------------------------------------------
# jq substitutes bytes that are not valid UTF-8, which would corrupt a latin-1 document,
# and a read-only file must not produce shell noise on the hook's stderr.
bad="$work/latin1.md"
printf 'caf\xe9 is fast — it formats one file.\n' > "$bad"
cp "$bad" "$work/latin1.orig"
run_hook "$bad" "$work/nobin"
if cmp -s "$bad" "$work/latin1.orig"; then pass=$((pass + 1))
else fail=$((fail + 1)); printf '  FAIL byte_safety: rewrote a file that is not valid UTF-8\n'; fi

ro="$work/readonly.md"
printf 'The hook is fast — it formats one file.\n' > "$ro"
chmod 444 "$ro"
noise=$(printf '{"tool_input":{"file_path":"%s"}}' "$ro" \
  | env -u MEGAPOWERS_PROSE_REGISTER PATH="$work/nobin:$PATH" bash "$HOOK" 2>&1 >/dev/null)
chmod 644 "$ro"
if [ -z "$noise" ]; then pass=$((pass + 1))
else fail=$((fail + 1)); printf '  FAIL byte_safety: hook wrote to stderr: %s\n' "$noise"; fi

# --- 4. only_markdown ---------------------------------------------------------
go="$work/main.go"
cat > "$go" <<'EOF'
package main

import "fmt"

func main() {
	// curly “q” and it’s ‘b’
	s := "em — dash, arrow → and §7"
	fmt.Println(s)
}
EOF
run_hook "$go" "$work/fmtbin"

has only_markdown 's := "em — dash, arrow → and §7"' "$go"
has only_markdown '// curly “q” and it’s ‘b’' "$go"
lacks only_markdown 'section 7' "$go"

# --- 5. register_switch -------------------------------------------------------
# On by default, like the rest of this plugin. MEGAPOWERS_PROSE_REGISTER=off disables the
# prose rewriting and nothing else: Go and prettier formatting are not part of the switch.
# Only the exact value `off` disables it, so a stray `0`, `false`, or `no` fails safe to on.
off="$work/switch-off.md"
printf '# title\n\nThe hook is fast — it formats one file.\n\nThe rule says “no dashes” here.\n' > "$off"
cp "$off" "$work/switch-off.orig"
printf '{"tool_input":{"file_path":"%s"}}' "$off" \
  | MEGAPOWERS_PROSE_REGISTER=off PATH="$work/nobin:$PATH" bash "$HOOK" >/dev/null 2>&1
if cmp -s "$off" "$work/switch-off.orig"; then pass=$((pass + 1))
else fail=$((fail + 1)); printf '  FAIL register_switch: =off still rewrote the markdown\n'; diff -u "$work/switch-off.orig" "$off" | sed -n '3,10p'; fi

# The same document with the variable explicitly unset must normalize, so the check above
# cannot be passing because the normalizer is broken.
on="$work/switch-on.md"
cp "$work/switch-off.orig" "$on"
run_hook "$on" "$work/nobin"
has register_switch 'fast, it formats one file.' "$on"
has register_switch 'The rule says "no dashes" here.' "$on"

# Any value other than `off` is on.
other="$work/switch-other.md"
cp "$work/switch-off.orig" "$other"
printf '{"tool_input":{"file_path":"%s"}}' "$other" \
  | MEGAPOWERS_PROSE_REGISTER=false PATH="$work/nobin:$PATH" bash "$HOOK" >/dev/null 2>&1
has register_switch 'fast, it formats one file.' "$other"

# The switch gates the register only. A Go file is still formatted with it off.
if command -v gofmt >/dev/null 2>&1 || command -v goimports >/dev/null 2>&1; then
  goff="$work/switch-off.go"
  printf 'package main\n\nfunc main() {\nx := 1\n_ = x\n}\n' > "$goff"
  printf '{"tool_input":{"file_path":"%s"}}' "$goff" \
    | MEGAPOWERS_PROSE_REGISTER=off PATH="$work/nobin:$PATH" bash "$HOOK" >/dev/null 2>&1
  has register_switch '	x := 1' "$goff"
else
  printf '  SKIP register_switch: no gofmt or goimports on PATH\n'
fi

echo "== auto-format: $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
