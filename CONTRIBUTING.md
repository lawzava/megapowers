# Contributing

Contributions are welcome. The bar is the one the repo holds itself to: a
claim of effect needs a run behind it.

## Before you open a PR

1. Run the gates locally; both must be green:

   ```bash
   scripts/validate.sh      # structural: manifests, frontmatter, cross-refs, docs consistency, hooks
   bash evals/run-all.sh    # behavioral: deterministic scenarios with the mock agent
   ```

2. If you change behavioral guidance in a skill (a rule, prohibition,
   recipe, or conditional meant to shape agent behavior), follow
   `plugins/megapowers/skills/writing-skills`: baseline the failure first,
   then write the guidance. Editorial changes (typos, links, rewording that
   preserves meaning) need no pressure test.

3. If you add an eval oracle, mutation-test it: feed it a deliberately broken
   artifact and confirm it fails. An oracle that cannot fail is a no-op, and
   review will ask for the evidence.

4. If you add or change a hook, add or extend its test under
   `plugins/*/hooks/tests/*.test.sh` (dependency-free bash, see the existing
   suites), and keep it fail-open: any error or uncertainty must allow.

5. Keep changes portable. Skills must work as plain `SKILL.md` on Claude Code,
   Codex, OpenCode, and Antigravity; anything Claude-only (hooks) is labeled
   Claude-only and fails open by absence elsewhere.

## Conventions

- Conventional commits (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`).
- One concern per commit; explain the why in the body.
- Cross-plugin skill references are soft: guard them with "if installed".
- No unsourced statistics in skills. See `evals/RESULTS.md` for the format a
  claim of effect needs.

## What gets merged

Small, verifiable improvements land fast. Large reworks should start as an
issue describing the failure you observed (ideally with a baseline transcript
or eval scenario) before the rewrite.
