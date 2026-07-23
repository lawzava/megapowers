# Install-smoke study

The out-of-box test the other studies don't cover: can a fresh environment
install megapowers by following the repo's own docs, and does the very first
task actually reach an installed skill? Structural validation (`validate.sh`)
proves manifests are well-formed; this proves the delivery path (discovery,
install, and first-task loading) on real harnesses.

## Protocol

For each harness, in a fresh config home (only credentials copied in; no user
config, no other plugins, no CLAUDE.md/AGENTS.md):

1. **Install per docs/setup.md, non-interactively.**
   - Claude Code: `claude plugin marketplace add <checkout>` +
     `claude plugin install megapowers@megapowers` (fresh `CLAUDE_CONFIG_DIR`).
   - Codex: `codex plugin marketplace add <checkout>` +
     `codex plugin add megapowers@megapowers` (fresh `CODEX_HOME`).
   - OpenCode / Antigravity: the documented manual path, a symlink of the
     canonical skill directory into the project (`.claude/skills/` for
     OpenCode's Claude-compatible loading, `.agents/skills/` for Antigravity).
2. **Assert the install registered** (`… plugin list` shows the plugin).
3. **First task**: in an empty project, ask the agent to load the
   test-driven-development skill and quote its core-principle sentence
   verbatim. The fresh home contains no other copy of that text, so a correct
   quote is strong evidence the installed skill was discovered and read. Oracle:
   a fixed-string, case-sensitive match on the skill's whole core-principle
   clause, not a five-word substring, so generic TDD phrasing does not pass.
   Caveat: this is a strong load-signal, not an unguessable nonce. The sentence
   descends from the public upstream this suite forked from (obra/superpowers,
   MIT), so a model could in principle reproduce it from training; an
   install-time random token would be needed for an unguessable proof.

Verdicts are `PASS` / `FAIL` / `SKIP(reason)` per assertion. Local diagnostic
mode permits a SKIP only when at least one assertion passes; an all-SKIP run
fails. Exact-ref remote release mode is strict: any SKIP or FAIL fails the run.

## Run

```bash
evals/studies/install-smoke/run-smoke.sh --out /tmp/install-smoke
# subset: --harnesses claude,codex

# post-publish release gate: fetch and test the exact public tag
evals/studies/install-smoke/run-smoke.sh \
  --out /tmp/install-smoke-v0.5.0 \
  --source lawzava/megapowers --ref v0.5.0 --version 0.5.0 \
  --harnesses claude,codex
```

Requires real credentials (run outside any credential-blocking sandbox).
Artifacts (install logs, task transcripts) land in `--out` for auditing.

## Scope and limits

- Local diagnostic mode uses a checkout path. The release gate clones the
  declared remote tag, verifies HEAD is exactly that tag, verifies every
  Claude/Codex plugin manifest has the expected version, records the commit in
  `source.json`, then installs from that immutable checkout into fresh homes.
- The first-task probe is an explicit skill request (deterministic oracle).
  Whether skills trigger organically on a matching task is a behavior
  question; that axis belongs to the process-behavior study's methodology,
  not an install check.
- OpenCode/Antigravity assertions follow the documented manual install; if
  your local CLI versions load skills from different paths, the study reports
  FAIL with artifacts rather than guessing.
