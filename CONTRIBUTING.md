# Contributing

Contributions are welcome. The bar is the one the repo holds itself to: a
claim of effect needs a run behind it.

## Before you open a PR

1. Run the gates locally; both must be green:

   ```bash
   scripts/validate.sh      # structural: manifests, frontmatter, cross-refs, docs consistency, hooks
   bash evals/run-all.sh    # behavioral: deterministic scenarios with the mock agent
   ```

2. Match the evidence to the kind of change:

   | Change | What it needs |
   |---|---|
   | **Adding** behavioral guidance (a rule, prohibition, recipe, conditional) | Baseline the failure first, then write the guidance. Follow `plugins/megapowers/skills/writing-skills`. |
   | **Removing or compressing** guidance | The gates green, plus a sentence on why the model no longer needs it. No pressure test, unless the wording carries a published effect size in `evals/RESULTS.md`. |
   | Editorial (typos, links, meaning-preserving rewording) | Nothing beyond the gates. |

   Trims are welcome and do not need to clear the bar that additions do.
   Guidance written for an older model generation degrades output on a newer
   one, so a PR that deletes a rule the model now follows by default is a fix,
   not a regression. `plugins/megapowers/skills/writing-skills/de-prescription-rubric.md`
   is the standard for what comes out and what stays.

3. If you add an eval oracle, mutation-test it: feed it a deliberately broken
   artifact and confirm it fails. An oracle that cannot fail is a no-op, and
   review will ask for the evidence.

4. If you add or change a hook, add or extend its test under
   `plugins/*/hooks/tests/*.test.sh` (dependency-free bash, see the existing
   suites), and keep it fail-open: any error or uncertainty must allow.

5. Keep changes portable. Skills must work as plain `SKILL.md` on Claude Code,
   Codex, OpenCode, and Antigravity; harness-specific enforcement (hooks) is
   labeled by harness and fails open by absence elsewhere. Plugin manifests ship
   the supported Claude Code and Codex lifecycle hooks; the other harnesses are
   skills-only.

## Conventions

- Conventional commits (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`).
- One concern per commit. The subject carries the change; add a one sentence
  body only when the why is not readable from the diff.
- Cross-plugin skill references are soft: guard them with "if installed".
- No unsourced statistics in skills. See `evals/RESULTS.md` for the format a
  claim of effect needs.

## Releases

Write the `## X.Y.Z - ` CHANGELOG.md entry, then run `scripts/release.sh X.Y.Z`.
It stamps every plugin manifest and the public install pins in README.md,
docs/agent-install.md, and docs/setup.md; `scripts/validate.sh` checks the
result against the changelog. After the signed tag is public, run the strict
fresh-install gate against that exact remote ref:

```bash
evals/studies/install-smoke/run-smoke.sh \
  --out "${TMPDIR:-/tmp}/megapowers-install-X.Y.Z" \
  --source lawzava/megapowers --ref vX.Y.Z --version X.Y.Z \
  --harnesses claude,codex
```

This post-publish gate must have no FAIL or SKIP result. It records the fetched
commit in `source.json`; do not substitute a local checkout for release
certification.

## What gets merged

Small, verifiable improvements land fast. Large reworks should start as an
issue describing the failure you observed (ideally with a baseline transcript
or eval scenario) before the rewrite.

Shipped guidance describes how the ecosystem works now. It is not a changelog:
migration notes, superseded behavior, war stories that motivated a rule, and
measurement provenance belong in `CHANGELOG.md` or `evals/RESULTS.md`, not in a
skill body an agent loads. A PR that moves one of those out is welcome.
