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
- Isolated or parallel work → **using-git-worktrees**, **subagent-driven-development**,
  **dispatching-parallel-agents**.

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
survives plugin updates. The catalog declares who leads (`[lead]`), the
vendor-neutral tier scale and each provider's tier map, which provider handles which
role, the floor for anything that ships, and how each delegate runs. Resolve routes
with `scripts/delegate-resolve <role>`; the delegation skill and the delegate agents
read the same table.

Single-writer rule: delegates write only inside worktrees or return patches. The lead
owns integration and commits. Always run the tests yourself and confirm the output;
never trust a self-reported pass.

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
