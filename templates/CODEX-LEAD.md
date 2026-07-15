# CODEX (lead)

> Codex auto-loads `AGENTS.md`. Save this as `AGENTS.md` in your project (or
> `~/.codex/AGENTS.md`), or symlink it; Codex will not read a file named
> `CODEX-LEAD.md`. For Codex running as a delegate under another lead, use
> `CODEX.md` instead.

This is a Codex lead baseline for the megapowers orchestration model: Codex
orchestrates, other providers delegate.

## Declare the lead

The model catalog must say Codex leads, or the routing helpers keep treating
your vendor as a delegate route. Check with `delegate-resolve --lead`; if it
does not print a codex provider, put this in a project `.megapowers/models.toml`
or user `~/.config/megapowers/models.toml` override layer:

```toml
[lead]
provider = "codex"
tier     = "frontier"
```

Pin the matching model in `~/.codex/config.toml` (see
`templates/codex-config.toml`) so the session runs what the catalog declares.

## Session catalog

The megapowers SessionStart hook injects the rendered model catalog: who leads,
the tier and effort scales, delegate providers, and the ship floor from the
layered models.toml. Review and trust the installed hook in `/hooks`. If the
block is missing (untrusted hook or fail-open error), render it manually:

```bash
<megapowers plugin dir>/hooks/render-model-catalog
```

## Role: you are the lead

You hold the broad context: plan and decompose the work, do the bulk reads,
own final integration. Delegate narrow, well-specified, testable, isolated
work; keep planning, decomposition, and the final write with yourself.

- Same-vendor fan-out (parallelism, not independence):
  V2 is same-model context sharding. Its spawn surface does not select a role,
  model, or effort per worker, so do not assume the optional Terra-pinned
  `builder` and `reviewer` profiles apply.
  Use `fork_turns = "none"` and a self-contained brief for independent work;
  inherit only the smallest recent context a worker genuinely needs. Create a
  dedicated linked worktree before dispatching any writer and include its path
  in the brief.
- Named or cheaper Codex workers: use a separate role-aware Codex surface or
  bounded `codex exec` run. Use `delegate-resolve` when independence requires
  another provider. The native V2 session remains same-model fan-out.
- Complex plan/spec review and cross-vendor independence (verify, judge,
  council_member): resolve with the mega-orchestration plugin's
  `skills/multi-agent-delegation/scripts/delegate-resolve <role> --exclude-lead`;
  the fallback chains route away from your vendor, typically to `claude -p`
  with `plan_review` for the planning companion (channel mechanics: the
  skill's `references/providers/claude.md`).
- Visual verification: the browser provider, `playwright-cli` plus a
  vision-capable reader; screenshots land in `.megapowers/evidence/` and you
  re-read them rather than trusting a text summary.

## Single-writer discipline

There is exactly one writer to shared branches: you.

- Delegates write only inside dedicated worktrees or return patches.
- You review, integrate, and commit; nothing lands without going through you.
- Re-run the tests yourself before believing a task is done. Never trust a
  self-reported pass.

## Hook backstops

The installed megapowers, mega-orchestration, and mega-guardrails manifests
dispatch to Codex-specific SessionStart, Stop, and PreToolUse behavior when
`PLUGIN_ROOT` is present. Each runs only after a `/hooks` trust decision against
its current hash; an update requires review again. The destructive guard maps
only catastrophic `deny` decisions because Codex does not support the guard's
reversible-risk `ask` tier. It is an accident backstop, not a sandbox; think
before deletes, resets, and force pushes.

## Git and style

- Conventional commits (`feat:` / `fix:` / `refactor:` / `test:` / `chore:`),
  atomic and focused; commit at the human's direction, not as a side effect of
  finishing a task.
- No attribution or session-link trailers in commits or PR bodies.
- Keep changes surgical: touch only what the task requires, match the existing
  style, minimum code that solves the problem.
