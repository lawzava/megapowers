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
  inherit only the smallest recent context a worker genuinely needs.
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

## Writer ownership discipline

There is exactly one writer to each owned path.

- Outside recursive coordinator mode, delegates write only inside dedicated
  worktrees or return patches.
- The lead reviews the joined diff and performs any authorized Git action after
  its direct children return.
- Re-run the tests yourself before believing a task is done. Never trust a
  self-reported pass.

For explicitly selected recursive coordinator mode, native subagents may write
concurrently only to disjoint owned paths in the shared checkout. Do not create
worktrees for this mode. A coordinator may subdivide only its inherited ownership.
Overlapping paths, shared interfaces, and dependencies remain sequential. Each
coordinator waits for its direct children, verifies their combined edits, and
returns one synthesized subtree result to its parent. The lead joins only its
direct children. Children must not perform Git index or ref operations.

## Hook backstops

The installed megapowers, mega-orchestration, and mega-guardrails manifests
dispatch to Codex-specific SessionStart, Stop, and PreToolUse behavior when
`PLUGIN_ROOT` is present. Each runs only after a `/hooks` trust decision against
its current hash; an update requires review again. The destructive guard maps
only catastrophic `deny` decisions because Codex does not support the guard's
reversible-risk `ask` tier. It is an accident backstop, not a sandbox; think
before deletes, resets, and force pushes.

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

## Git and style

- Conventional commits (`feat:` / `fix:` / `refactor:` / `test:` / `chore:`),
  atomic and focused; commit at the human's direction, not as a side effect of
  finishing a task.
- No attribution or session-link trailers in commits or PR bodies.
- Keep changes surgical: touch only what the task requires, match the existing
  style, minimum code that solves the problem.
