# Install-smoke study

The out-of-box test the other studies don't cover: **can a fresh environment
install this suite by following the repo's own docs, and does the very first
task actually reach an installed skill?** Structural validation (`validate.sh`)
proves manifests are well-formed; this proves the *delivery path* — discovery,
install, and first-task loading — on real harnesses.

## Protocol

For each harness, in a **fresh config home** (only credentials copied in — no
user config, no other plugins, no CLAUDE.md/AGENTS.md):

1. **Install per docs/setup.md, non-interactively.**
   - Claude Code: `claude plugin marketplace add <checkout>` +
     `claude plugin install megapowers@megapowers` (fresh `CLAUDE_CONFIG_DIR`).
   - Codex: `codex plugin marketplace add <checkout>` +
     `codex plugin add megapowers@megapowers` (fresh `CODEX_HOME`).
   - OpenCode / Antigravity: the documented manual path — symlink the canonical
     skill directory into the project (`.claude/skills/` for OpenCode's
     Claude-compatible loading, `.agents/skills/` for Antigravity).
2. **Assert the install registered** (`… plugin list` shows the plugin).
3. **First task**: in an empty project, ask the agent to load the
   test-driven-development skill and quote its core-principle sentence
   verbatim. The fresh home contains no other copy of that text, so a correct
   quote proves the installed skill was discovered and read. Oracle:
   case-insensitive match on "watch the test fail".

Verdicts are `PASS` / `FAIL` / `SKIP(reason)` per assertion; a harness with no
CLI or no working auth is SKIPPED, not silently passed. Exit code 1 on any FAIL.

## Run

```bash
evals/studies/install-smoke/run-smoke.sh --out /tmp/install-smoke
# subset: --harnesses claude,codex
```

Requires real credentials (run outside any credential-blocking sandbox).
Artifacts (install logs, task transcripts) land in `--out` for auditing.

## Honest scope

- The marketplace add uses the **local checkout path**, not the network form
  (`lawzava/megapowers`) — it validates the repo's marketplace/manifest
  wiring end to end, but not GitHub fetching.
- The first-task probe is an **explicit** skill request (deterministic oracle).
  Whether skills trigger *organically* on a matching task is a behavior
  question — that axis belongs to the process-behavior study's methodology,
  not an install check.
- OpenCode/Antigravity assertions follow the documented manual install; if
  your local CLI versions load skills from different paths, the study reports
  FAIL with artifacts rather than guessing.
