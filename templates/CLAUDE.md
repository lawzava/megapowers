<!-- Recommended baseline; adapt to your project. Pairs with the megapowers marketplace. -->

# Project instructions

Baseline for a project running the
[megapowers](https://github.com/lawzava/megapowers) plugins. Copy to
`CLAUDE.md`, or merge the sections you want into `~/.claude/CLAUDE.md`, then
edit to fit your stack. It leans on the plugins instead of restating them.

## Answers

Write for a senior engineer skimming. Compression beats grammar: drop articles,
subjects, and copulas where meaning survives. Fragments are fine. Slang is
fine. Padding is not.

- Answer in the first line. No preamble, no restating the question.
- Four lines of prose is the ceiling. Code, diffs, and command output are free.
- Cite `path:line`. Do not narrate where something lives.
- One line per finding or option. Three or more items go in a list or table.
- No recap of what you just did. No closing summary. No offers to help further.
- State a risk once, plainly, then stop. Do not stack hedges.
- Ask a question as one line, no framing.
- No em or en dashes. Use a period, comma, colon, or parentheses.

Length comes from content, never from manner. A real tradeoff, design, or
postmortem earns more; the same answer said slower does not.

## Workflow

Skills own their procedures. When one covers the task, follow it rather than
improvising a parallel process. Mechanical edits need no skill.

Unclear feature: brainstorming, then writing-plans. Implementing:
test-driven-development, failing test first. Something broken:
systematic-debugging before proposing a fix. Wrapping up:
requesting-code-review, verification-before-completion,
finishing-a-development-branch.

## Delegation

Route specialized work to the best model through mega-orchestration instead of
doing everything inline. The session catalog block renders the lead, tiers, and
floor at start, so model choices need no skill invocation.
`scripts/delegate-resolve <role>` resolves a route. Put model updates in a
project `.megapowers/models.toml` or user `~/.config/megapowers/models.toml`
override layer, which survives plugin updates.

Delegates write only inside worktrees or return patches. The lead owns review,
integration, and Git. Run the tests yourself; never trust a self-reported pass.
Independence is per artifact author: resolve with `--author-vendor <vendor>`,
not `--exclude-lead`, so a review routes away from whoever actually wrote it.

Recursive coordinator mode uses nested Agent calls, not agent teams. Children
get disjoint paths in the shared checkout; overlapping work stays sequential. Do
not create worktrees for this mode. Children must not perform Git index or ref
operations. Full contract: megapowers:subagent-driven-development.

For very large audits, migrations, or repeatable multi-agent research, prefer
the harness's own workflow runner over hand-managed delegation.

## Git

- Branch per feature or fix. Never commit directly to `main`.
- Conventional commits (`feat:` / `fix:` / `refactor:` / `test:` / `chore:`),
  atomic: one logical change each.
- The subject line carries the change. Add a body only when the why is not
  readable from the diff, and cap it at one sentence. No paragraphs, no bullet
  lists, no restating what the diff already shows.
- Commit at the human's direction, not as a side effect of a skill step.
- No attribution, co-author, or session-link trailers.
- Stage explicit paths. Do not force-add ignored files or bypass hooks.

## Review and verification

Independent review for risky logic: auth, billing, concurrency, security, data
integrity. Evaluate the feedback on the merits; agreement is not a response.

Run the command, read the output, then claim. Assertions without evidence do
not count as complete.

## Safety

The mega-guardrails deny-destructive hook is an accident backstop. It is not a
sandbox and not a security boundary. Think before you run.

## Scratch storage

Honor `$TMPDIR` and tool-specific temporary or cache variables. Do not hard-code `/tmp` for worktrees, build caches, browser profiles, or other large artifacts.
Before a large scratch job, confirm the directory exists, is writable in the current sandbox, and has enough capacity.
Do not silently fall back to `/tmp` for large output: request scoped access or use an ignored workspace directory. Keep `/tmp` for small, short-lived OS temporary files and IPC state.

## Code

- Write the minimum code that solves the problem. No speculative features,
  single-use abstractions, or premature configurability.
- Touch only what the request requires. Match the surrounding style. No
  drive-by refactors. Clean up your own orphans; leave pre-existing dead code
  alone but mention it.
- Run the tests after every meaningful change. If three attempts at one
  approach fail, stop and summarize what you tried, what failed, and the next
  idea.
