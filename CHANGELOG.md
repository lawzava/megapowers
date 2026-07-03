# Changelog

All plugins version together; the version in each Claude and Codex plugin
manifest (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`) matches
the repo release. The minimal Antigravity root manifests carry no version
field by design (their schema allows only name and description). Format:
[Keep a Changelog](https://keepachangelog.com), semver.

## Unreleased

### Added

- `docs/agent-install.md`: the setup guide rewritten as instructions for a
  coding agent, so installation is one pasted line in any harness. Covers
  harness detection, channel choice, the shared-directory double-registration
  trap, the install-smoke verification probe, and an explicit-approval rule
  for anything that widens permissions. Linked from the README quickstart;
  guarded as a required file by validate.sh.

### Fixed

- Journal provenance, mechanically this time. The 0.1.2 re-probe verified the
  other two probe fixes live (both TDD runs ran the full suite and reported
  the planted failure; the autonomous run self-certified with a stamped
  LAST_VERIFY) but model=unknown persisted: the runbook's "export
  MEGAPOWERS_MODEL" instruction cannot work, because each tool call runs in a
  fresh shell. run-init now takes --model and persists it in the run dir;
  run-journal falls back to that file when the env var is unset. Oracle
  extended: a journal call with no env var must record the persisted model.

## 0.1.2 - 2026-07-03

### Changed

- First live e2e probes of the installed suite (5 real-session runs: trigger
  precision, organic TDD x2, brainstorming, autonomous-run) produced three
  fixes:
  - test-driven-development "verify green" now says to run the project's full
    suite (its canonical entrypoint), not only the new test file. One probe
    run reported clean over a red suite because it only ran its own module —
    the scoped-true-claims decay mode the gauntlet study predicted. Gauntlet
    keyed re-run recommended before citing its numbers for this skill version.
  - run-verify-status now stamps LAST_VERIFY into the status file on pass, and
    run-derive-status says so when it derives STATE=done. A run that finishes
    without certification is now visible (done + LAST_VERIFY=none). Covered by
    the autonomous-run-contract oracle.
  - run-init's runbook template tells the agent to export MEGAPOWERS_MODEL
    once per session; without it every journal entry logs model=unknown
    (observed in the live probe).

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
