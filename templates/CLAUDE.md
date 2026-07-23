<!-- Recommended baseline; adapt to your project. Pairs with the megapowers marketplace. -->

# Project instructions

This is a starting baseline. Copy it to your project root as `CLAUDE.md`, or merge
the sections you want into `~/.claude/CLAUDE.md`, then edit to fit your stack. It
assumes the [megapowers](https://github.com/lawzava/megapowers) plugins are installed
and leans on them instead of restating their content.

## Workflow

Let the megapowers process skills lead; don't paraphrase or pre-empt them.

- Creating or changing behavior → start with **brainstorming**, then **writing-plans**.
- Implementing → **test-driven-development** (write the failing test first).
- Something's broken → **systematic-debugging** before proposing a fix.
- Wrapping up → **requesting-code-review**, **verification-before-completion**, then
  **finishing-a-development-branch**.
- Ordinary isolated or parallel work → **using-git-worktrees**,
  **subagent-driven-development**, or parallel fan-out per **orchestrating**.

When a skill applies, invoke it before answering. It owns the procedure; these notes
just say when to reach for which.

## Delegation

Route specialized work to the best model via the mega-orchestration plugin rather than
doing everything inline. The model catalog (lead, tiers, providers, floor) lives in
models.toml and the role routing in delegates.toml; both are layered, with a project
`.megapowers/<file>` or user `~/.config/megapowers/<file>` override winning per key
over the shipped copies. Every session sees a rendered catalog block at start (the
megapowers SessionStart hook runs hooks/render-model-catalog), so model and tier
choices need no skill invocation. Model updates go in an override layer, which
survives plugin updates. This file assumes Claude leads: if the shipped
catalog's `[lead]` declares another provider, set `[lead] provider = "claude"`
in an override layer. Independent reviews declare each artifact author's
actual vendor with `--author-vendor`; lead identity is not authorship. The
catalog declares who leads, the tier scale, each
provider's tier map, and the floor; delegates.toml maps roles to providers and
defines how each delegate runs. Resolve routes with `scripts/delegate-resolve
<role>`; the delegation skill and the delegate agents read the same tables.

For ordinary delegation, delegates write only inside worktrees or return patches. The
lead owns final review and authorized Git actions. Always run the tests yourself and
confirm the output; never trust a self-reported pass.

Recursive coordinator mode uses nested Agent calls, not agent teams. Teams cannot
nest. Use it only when coordinator subagents have access to `Agent`. Assign children
disjoint paths in the shared checkout, and keep overlapping work sequential. Do not
create worktrees for this mode. Each coordinator waits for its direct children,
verifies their combined edits, and returns one synthesized subtree result to its
parent. Children must not perform Git index or ref operations. The lead performs any
authorized Git action only after its direct children return.

For very large audits, migrations, or repeatable multi-agent research, prefer Claude
Code dynamic workflows (its built-in multi-agent workflow runner, invoked with the
`ultracode` keyword or a saved workflow) over hand-managed delegation. Use ordinary
skills and direct subagents for small or medium tasks.

## Git

- Branch per feature or fix. Never commit directly to `main`.
- Conventional commits (`feat:` / `fix:` / `refactor:` / `test:` / `chore:`), atomic:
  one logical change each.
- Commit at the human's direction, not as a side effect of a skill step.
- No attribution, co-author, or session-link trailers in commit messages or PR bodies.
- Stage explicit paths. Don't force-add ignored files or bypass hooks.

## Review & verification

- Get an independent review for risky logic: auth, billing, concurrency, anything
  with security or data-integrity stakes. This is what requesting-code-review and
  receiving-code-review are for; take review feedback with technical rigor, not
  reflexive agreement.
- Show evidence before claiming done. Run the command, read the output, then report:
  the discipline verification-before-completion enforces. Assertions without
  evidence don't count as complete.

## Safety

The mega-guardrails deny-destructive hook is an accident backstop: it blocks a handful
of obviously destructive commands. It is not a sandbox and not a security boundary.
Don't rely on it to contain untrusted input or risky operations; think before you run.

## Scratch storage

- Honor `$TMPDIR` and tool-specific temporary or cache variables. Do not
  hard-code `/tmp` for worktrees, build caches, browser profiles, model
  archives, or other potentially large artifacts.
- Before a large scratch job, confirm the selected directory exists, is
  writable in the current sandbox, and has enough capacity. Prefer disk-backed
  scratch when `/tmp` is memory-backed or constrained.
- If the configured scratch root is not writable, request scoped access or use
  an ignored workspace directory. Do not silently fall back to `/tmp` for
  large output.
- Keep `/tmp` for small, short-lived OS temporary files and IPC state.

## Style

- Terse. No filler, no preamble, no hedging. Status updates in one line.
- No em or en dashes. Scan final text for `—` and `–` and rewrite each hit
  with a period, comma, colon, or parentheses.
- User-facing text states what changed and the measured number. No sales
  punchlines ("this release is for you"), no sizzle adjectives; the facts do
  the selling.
- Write the minimum code that solves the problem. No speculative features, single-use
  abstractions, or premature configurability.
- Surgical changes: touch only what the request requires. No drive-by refactors or
  reformatting. Match the surrounding style. Clean up your own orphans; leave
  pre-existing dead code alone (but mention it).
- Run the tests after every meaningful change. If three attempts at one approach fail,
  stop and summarize what you tried, what failed, and the next idea.
