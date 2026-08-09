# The risky-logic review gate: what it is and why it reads what it reads

Design record for `hooks/delegate-nudge.sh` and the `rules.risky-logic-review`
block in `enforcement.toml`. The config file carries what an editor needs at
the point of edit; the reasoning lives here so a narrowing is not re-litigated
or, worse, quietly re-introduced.

## It is an accident backstop, not a security boundary

It catches the ordinary case: someone changes auth, billing, or concurrency
logic and ships it without a second pair of eyes. That case is common, it is
expensive, and a keyword scan over the pending diff catches it reliably.

It cannot become a security boundary. Five rounds of adversarial review against
the hook found the same class of defect every round: a narrowing that trusted a
NAME instead of proving a PROPERTY. The pending tree, a bounded byte sniff, a
file extension, a comment marker, a directory name, a path in the index rather
than the worktree. Each was real, each was fixed, and the next round found
another. That is what a heuristic over attacker-controlled input does, because
whoever writes the diff chooses the bytes the scanner reads.

So do not build anything on top of this that assumes evasion is hard. If a
change must not ship unreviewed, the control is the review itself, in CI, on a
branch nobody can push past. This gate makes forgetting expensive. It does not
make lying impossible.

For maintenance: a false NEGATIVE is worth fixing when it is reachable by
accident, and worth writing down rather than chasing when it needs a crafted
payload. A false POSITIVE is what actually kills a gate, because one that fires
on work it should not is one sessions learn to route around.

## Promote on measurement

The 2026-08-05 audit over 1324 Claude transcripts found the enforced gate
honored ~100% of the time at a cost of 141 blocks, 47 of them in a
documentation repository. It found the advisory router honored 100% of the time
across 8 deliveries. Neither mechanism has a compliance problem: the gate has a
precision problem and the router has a recall problem, which is why the work
went into scoping the gate and broadening the router rather than making either
louder. Those 141 blocks, and the 47, are how this rule nearly died.

## Why each scope key is narrowed the way it is

**`added_lines_only`.** Context lines are the tree as it already stood and
removed lines are risk going away. Scanning all three made every edit near auth
code read as an auth change.

**`skip_comments`.** A comment cannot authenticate a user or charge a card.
This is what stops prose ABOUT the gate from tripping the gate: the audit's
single largest false-positive cluster was this repository's own hook source and
docs citing the keyword list, 47 of 141 blocks.

The marker set is chosen per file extension, because `#`, `--`, and `;` open a
comment in some languages and execute in others (`#define AUTHZ_DISABLED 1`,
`--paymentAttempts`, `; paymentProcessor.charge()`). An unknown extension scans
every line, because the safe default when the language is unknown is to read
the line rather than trust a guess about its syntax.

`*` is in no marker set at all. It opens nothing: it only CONTINUES a block
comment, which a scanner reading one line at a time cannot know it is inside.
Meanwhile it dereferences a pointer, multiplies, and starts a generator method
in the very languages that set covers, so `*paymentTotal = 0` scanned clean.
One false positive on a continuation line beats a missed dereference.

**`exclude_globs`.** The test every entry has to pass: is this format PROVEN
not to execute? Not "is this usually documentation", and never "is this path
important". An exclusion bought with a path or a convenient extension is a
blind spot with a reassuring name.

Four exclusions that look obvious are deliberately absent, each removed after it
was shown to hide executable content:

| Rejected | Why |
|---|---|
| the test tree | A test can carry real auth logic, and a security scan that skips it has a known hole. |
| `*.txt` | Reads like prose and is not. `requirements.txt` is a dependency manifest, and a line adding an auth or payment library there is exactly what this gate exists to catch. |
| `docs/*` | A path, not a format. `docs/auth.ts`, a shell script, or a generated site's JavaScript all execute. |
| `*.mdx` | MDX embeds JSX and JavaScript. A source format wearing a documentation extension. |

The hook checks the claim per file rather than trusting the glob, against the
bytes that would actually ship. An excluded path is scanned anyway when it
carries the executable mode bit, when its first line is a `#!` shebang, or when
it cannot be read at all, because a file the kernel may run is not prose
whatever it is called. Where the worktree copy cannot answer, because it is a
fifo, a device, or a directory, the same two questions go to the STAGED copy:
the index mode and the staged first two bytes are what a plain `git commit`
ships. A worktree copy that cannot be read has proven nothing, and treating "I
could not look" as "it is prose" is how an executable payload rode an excluded
extension.

**`self_exclude`.** The gate never scans the files that DEFINE it. Editing the
matcher is not shipping risky logic, and a rule whose own text matches itself
blocks forever: the review request cites its own warning as the risky change.

Each path is ANCHORED, not suffix matched, and the anchor is proven in both
shapes. Under a prefix, the prefix must hold this plugin: the running
installation when it lies inside the repository under review, or a prefix where
the repository declares this plugin in COMMITTED content as a
`.claude-plugin/plugin.json` naming it. With no prefix left, the repository
itself must be this plugin's: the running installation IS the repository root,
or the root carries a committed `.claude-plugin/plugin.json` naming this
plugin, or a committed root `.claude-plugin/marketplace.json` declares this
plugin at a source prefix that proves out by that same manifest test. The last
one is how the plugin's own source repository keeps entries carrying no plugin
prefix, such as `scripts/tests/enforcement.test.sh`, and it costs two committed
facts rather than a name.

A directory merely NAMED `mega-orchestration` anchors nothing, because a name is
something anyone can spell. A root file merely named `enforcement.toml` or
`hooks/delegate-nudge.sh` anchors nothing for the same reason. A manifest or
marketplace the pending tree ADDS anchors nothing either, because a change must
not mint its own exemption. An entry matching neither shape excludes nothing.

The list is longer than "the hook" because a gate needs fixtures, and a fixture
for a keyword scan necessarily contains the keywords. Any repository whose
subject matter IS this gate carries its own paths here; in an ordinary
repository the first two entries are the whole list. Adding a path costs a real
blind spot, so add one only when the file's purpose is to define or exercise the
gate, never because a hit was inconvenient.

**`max_rounds`.** The round ledger already counted consecutive dispatches of one
role on one branch; nothing capped them. The audit found a real branch that
reached round 11 and a repository branch that reached round 22. A reviewer that
has not converged in three passes is not converging: the next pass costs another
full review and returns another `needs_attention`.

## Unscannable content is announced, not blocked

A binary patch scanning as empty was a real hole. It is closed by saying so out
loud rather than by calling every unreadable file risky: for one round the gate
treated them all as hits, and adding an image, an archive, an object file, a
test fixture, or a generated report over the classification bound then cost a
full cross-vendor review with no risky word and no risky path anywhere in the
tree. That is the false positive that kills a gate.

The path is the one signal such a file still leaves, so an unscannable path
matching `keywords` blocks like any other finding. Everything else rides the
block preamble when something else fires, and goes out as a non-blocking notice
when nothing does. Not scanned is a fact; risky is a claim, and a claim needs
evidence.

A file the gate cannot OPEN takes the same route for the same reason. Unread and
unreadable-as-text are different reasons the scan stopped, not different rules:
the unreadable leg blocked outright for a round after the binary leg was
corrected, so a mode-000 `notes.md` cost a full review with no risky word and no
risky path. The readability test still runs ahead of the exclusion, because an
exclusion is a claim about content the gate CAN read; what that position settles
is whether the bytes were read, not whether they are risky.

## Why the declaration and the checker are separate

`scripts/check-enforcement.sh` requires `contract_test` to exist and to name the
rule. It deliberately does not try to prove the behavior itself.

That division was learned the hard way. The checker used to assert the guarantee
directly by grepping the hook for this file's name. A comment satisfied it.
Stripping comments first, an unused assignment satisfied it. Both times CI
reported the guarantee as met while nothing honored the value, because a textual
test cannot establish a behavioral property and text can always be shaped to
pass. A behavioral test can, and one already exists per hook, so the checker's
honest job is to insist the linkage is there rather than to reimplement the
proof badly.

## The comment-marker table

`skip_comments` needs to know which run of characters opens a comment, and the
answer is per language. Three common markers are executable syntax somewhere:
`#` opens a C preprocessor directive, `--` decrements in JavaScript, `;` opens a
statement in JavaScript and C. One marker set across every language reads all
three as comments and scans them clean, which is a hole chosen by the language
the change is written in. So the table picks the set from the file extension,
and an unknown extension scans EVERY line: an unrecognized language must cost a
false positive, never a silent pass. It is a table and not a parser because a
parser per language is a far larger surface than the false positives it removes.

**Every entry owes a proof that its marker cannot be executable syntax in that
language, and an extension naming more than one language proves nothing.** Seven
removals cost exactly that:

| Removed | Why it could not stay |
|---|---|
| `.asm` | Names no assembler. GNU as reads `;` as a statement separator, so `; call payment_processor` assembles. `.nasm` names one assembler and keeps `;`. |
| `.cl` | Common Lisp and OpenCL C, where `; chargeCard();` is a statement. |
| `.r` | R, REBOL, and Rez. Rez runs a C preprocessor, so `#define AUTHZ_DISABLED 1` is a directive. |
| `.m` | Objective-C, MATLAB, Mercury, Wolfram. `//` is the postfix application operator, so `// chargePayment` is a call. |
| `.pl` | Perl and Prolog. Prolog comments with `%`, so `#` is not comment syntax at all. `.pm` is a Perl module and keeps `#`. |
| `.cfg`, `.conf` | Name a PURPOSE, not a language. Anything can be spelled with them, so nothing about their syntax is known. |

Two entries were audited and KEPT, because removing them trades a narrow miss for
a broad false positive:

- `.yml` and `.yaml` keep `#`. Inside a literal block scalar a `#` line is
  document DATA rather than YAML syntax, so a keyword there can reach whatever
  consumes the document. Reaching it needs that consumer to read `#` as code, and
  dropping the entry costs every comment in every pipeline file. The miss is
  named and left.
- `.css` keeps the C-family set for `/* */`, which really is CSS comment syntax.
  The `//` half matches nothing CSS executes, since a `//` line is a parse error
  rather than a statement, so honoring it costs no safety.

**A marker is not a proof either: some comment-shaped lines execute.** A shebang
picks the interpreter, `//go:build` picks what compiles, an Emacs file-local
`eval:` runs on open, a server-side include runs on request. Each is written in
the file's comment syntax, so the marker set said "comment" and the scanner
skipped the line carrying the semantics. They are excepted in `is_comment`.
`.sql` is split off the dash set from the other direction: MySQL and PostgreSQL
disagree about whether `--payment` is a comment at all, so one rule for both was
a rule for neither.

**A marker that closes on the same line proves nothing about the rest of it.**
`/* note */ chargeCard()` and `<!-- note --> <script>go()</script>` both start
with an opener and both execute, so a block opener counts only when the line does
not also close it. `-->` is out of the table entirely for the same reason `*` is:
it CLOSES a comment, and everything after it on the line is live. Haskell ends
the dash run at the first symbol character, so `-->` is an operator there and a
continuation line may legally begin with one.

## The audit rounds, in order

Each round fixed the previous round's overcorrection. The hook comments state
the resulting invariant; the sequence is here so the swings are legible.

1. **A binary patch scanned as empty and passed in silence.** Staging a compiled
   artifact bought a clean bill of health.
2. **Fix: every unscannable file became a hit.** That overcorrected. Adding a
   favicon, a tarball, an object file, a test fixture blob, or a generated report
   past the classification bound forced a full cross-vendor review with no risky
   keyword and no risky path anywhere in the tree.
3. **Split the two halves.** NOT SCANNED is a fact and rides the non-blocking
   notice; RISKY is a claim needing evidence, and the only evidence left when the
   bytes cannot be read is the path. A changed `payment_processor.bin` is a
   different proposition from a changed `favicon.png`. Silence is never the
   outcome either way, which is what keeps round 1 closed.
4. **The unreadable leg kept round 2's behavior one round longer**, because it
   arrives through a different door: a mode-000 `notes.md` still demanded a full
   review. It now takes the same route as the binary leg.
5. **A bounded prefix is not a classification.** Reading one 8192-character
   window and calling everything without a NUL text let a file whose first NUL
   sat at byte 8193 pass as source in both legs, since git sniffs only its own
   first 8000 bytes and bash drops NUL from captured strings.

The recurring shape, and the thing to check in any new narrowing: the gate
answered "I could not look" with "it is prose". Whenever a narrowing's safe
default is to skip, it is the defect this file keeps being audited for.

## Where the worktree copy is not the answer

Several legs re-ask their question against the STAGED bytes. The reason is
always the same: `git commit` ships the index, and the worktree copy can differ
from it or fail to exist.

- A tracked path whose worktree copy is a fifo, character device, socket, or
  directory is dropped from the worktree hop by `skipspec`, so it is named ONCE
  and would be classified from a copy that cannot answer. The sandbox makes
  exactly this shape by bind mounting `/dev/null` over deny-listed paths.
- Stage an executable shebang payload as `billing.md`, then restore the worktree
  copy to inert prose, and the exclusion claim held over a file that had nothing
  to do with what would ship.
- Stage a late-NUL binary and then delete it from the worktree: it renders as
  ordinary added lines, sniffs nothing on disk because there is nothing on disk,
  and passes as text.

A deletion classifies nothing in every one of these, and that is deliberate:
removed content executes nowhere, and reading the base copy back would fire on
an ordinary `rm` of a document.
