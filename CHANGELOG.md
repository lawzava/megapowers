# Changelog

All plugins version together; the version in each Claude and Codex plugin
manifest (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`) matches
the repo release. The minimal Antigravity root manifests carry no version
field by design (their schema allows only name and description). Format:
[Keep a Changelog](https://keepachangelog.com), semver.

## 0.1.1 - 2026-07-03

### Fixed

- Full shellcheck pass now green in CI (14 scripts): quoting and `local`
  declaration cleanups, two justified suppressions for intentional patterns,
  and one real bug in the new head-to-head runner (`local a="$1" b="$a"`
  expands `$a` before the assignment lands, so the per-arm template path was
  built from an empty arm name).

### Removed

- The one-time star note in the `megapowers` SessionStart hook (shipped
  briefly after 0.1.0, never in a tagged release). An agent relaying a star
  request to its user reads as manipulative regardless of how gently it is
  worded; the README can ask instead.

### Added

- `evals/studies/head-to-head/` — committed protocol for a three-arm
  comparison (no suite / megapowers / upstream Superpowers) on the gauntlet
  task with organic triggering; no published numbers yet, awaits a keyed run.
- Scheduled freshness check (`scripts/check-freshness.sh` + monthly CI
  workflow): fails when a dated opinion (`docs/tool-support.md`,
  `delegates.toml`, eval results) has not been re-reviewed in 90 days, so
  staleness surfaces instead of rotting silently.
- README "see it work" section: a captured, reproducible hook transcript.
- Universal install channel documented: `npx skills add lawzava/megapowers`
  (the skills.sh CLI reads the marketplace manifest and discovers every
  plugin's skills — verified against the CLI's source; skills only, hooks
  and agents still ship via the native marketplaces). Plus a "Fleet" section
  in `docs/setup.md`: declarative multi-device sync via
  `extraKnownMarketplaces`/`enabledPlugins` (Claude Code) and
  `skills-lock.json` (everything else).

### Changed

- `docs/tool-support.md` now states the Windows support status explicitly
  (hooks are bash, CI runs on Linux; Windows untested).

## 0.1.0 - 2026-07-03

First versioned release. Everything before this shipped as 0.0.1 without a
changelog.

### Added

- `mega-orchestration/orchestrating` — the decision-root skill: routes a
  task's shape to the right structure (inline, parallel subagents, delegation,
  best-of-n, council, autonomous run) with spend-by-stakes effort defaults and
  a per-harness primitives reference (subagents / teams / workflows / effort).
- `run-loop.sh` Stop hook (Claude Code only): keeps an active autonomous run's
  loop turning instead of letting the session stop mid-run; exits only through
  honest journal state. 11-case test suite.
- `delegates.toml` roles `verify`, `judge`, `council_member` — the routes the
  swarm skills instruct through now resolve instead of exiting unknown-role.
- Senior-engineer communication register in `using-megapowers`, referenced by
  every artifact-writing skill (plans, briefs, specs, journals, reports).
- Autonomous-run ↔ spec-pipeline bridge: charters source their done-when from
  brainstormed specs, milestones execute via subagent-driven-development, and
  the three execution-path gates (writing-plans' execution-choice question,
  subagent-driven-development's pre-flight batch, executing-plans' concern and
  stop-and-ask checks) are now conditional on the run's autonomy level.
- `CONTRIBUTING.md`, `SECURITY.md`, issue/PR templates, this changelog.
- validate.sh docs-consistency checks (marketplace counts, plugin mentions,
  README skill lists) — the drift class that produced stale counts now fails CI.

### Changed

- Grug diet on the five heaviest skills (writing-skills −17%, brainstorming
  −16%, test-driven-development −7%, plus subagent-driven-development and
  systematic-debugging): duplicated rules stated once, phantom upstream skill
  references removed, unsourced statistics deleted. No discipline wording lost.
- Unified the subagent-driven-development vs executing-plans criterion in all
  three skills that state it.
- Doc accuracy: marketplace entry counts, plugin install lists, and per-plugin
  skill lists corrected and now CI-guarded.

### Migration notes

- If you installed before 0.1.0, reinstall/update each plugin (see "Updating"
  in `docs/setup.md`). No file formats changed; `.megapowers/` run and ledger
  state remains compatible.
