# Changelog

All plugins version together; the version in each Claude and Codex plugin
manifest (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`) matches
the repo release. The minimal Antigravity root manifests carry no version
field by design (their schema allows only name and description). Format:
[Keep a Changelog](https://keepachangelog.com), semver.

## Unreleased

### Changed

- Wave 1 de-prescription: six skills rewritten for frontier models per the
  de-prescription rubric (writing-skills, subagent-driven-development,
  systematic-debugging, test-driven-development, autonomous-run, and the
  always-injected using-megapowers payload), cutting prescriptive scaffolding
  while keeping the rationale; skill descriptions are byte-identical to v0.1.4
  (guarded by `scripts/check-description-freeze.sh`). Keyed gate re-measure
  (2026-07-07) PASSED: every discipline effect held on both gate arms
  (claude-fable-5, gpt-5.5), the pre-trim systematic-debugging wording's
  flaky-test regression on claude-fable-5 improved from 25% to 75% clean, and
  a documented claude-haiku-4-5 cost is recorded; full tables and protocol in
  `evals/RESULTS.md` §6.

## 0.1.4 - 2026-07-07

### Fixed

- `templates/agent-notify`: notifications gate on interactive sessions and real
  completions (entrypoint/CLAUDECODE checks, a minimum-turn threshold, and no
  "done" ping while background tasks are still running).
- Weekly accuracy sweep: harness-primitives agent-teams entry (GA and
  resumable, was described as experimental and non-resumable) and workflows
  entry (saved workflows, script API, per-agent overrides); megapowers README
  SessionStart injection size (291 words, was claimed 390); setup.md tag
  signing claim (v0.1.3+ tags are GPG-signed) and pin list; stripe-go module
  path gains its `/v86` suffix; `/ultrareview` marked as deprecated alias;
  writing-skills per-field frontmatter limits; delegate-nudge retired
  `gemini -p` example; mega-guardrails auto-format file-type list; mega-go
  root manifest description.

## 0.1.3 - 2026-07-04

### Security

- `scripts/security-lint.sh` scans skills, hooks, and templates for
  prompt-injection markers (external-URL fetches in executable context,
  base64-piped-to-shell, remote `eval`, unicode direction overrides,
  disable-safety instructions). It runs in CI through `validate.sh`, and refuses
  to let an allowlist entry silence a shipped `SKILL.md`. CI also gains a
  SHA-pinned gitleaks secret scan and a `claude plugin validate --strict` job.
- `SECURITY.md` gains an indirect-prompt-injection threat model, a
  before-you-install per-plugin capability disclosure (every hook is
  `network: none`), GitHub Private Vulnerability Reporting as the default
  reporting channel, and signed release tags.
- `deny-destructive` gains a prefilter that fast-allows only commands matching
  no destructive pattern (verified against every deny and ask fixture); an
  oversized command carrying a trigger token now degrades to ASK rather than the
  old 20000-char fail-open. A pilot Codex port of the guard ships for manual
  wiring (`hooks/codex-deny-destructive.sh`), and its adapter never emits the
  `ask` decision Codex does not support.

### Changed

- `docs/tool-support.md` renamed to `docs/harness-support.md`: the docs
  standardized on "harness" for the host program, and the filename now
  matches. All in-repo references updated (validate.sh required-files list,
  the freshness check, links).
- The 0.1.1 README "see it work" hook transcripts (dropped in a later README
  rewrite without a changelog note) now live in the plugin READMEs:
  deny-destructive in mega-guardrails, run-loop in mega-orchestration, both
  re-captured from the current hooks.
- Eval oracles hardened against the audit's demonstrated blind spots, each with
  a runnable `--selftest` mutation suite: install-smoke now requires the skill's
  core-principle sentence verbatim (fixed-string, case-sensitive), the
  process-behavior flaky branch rejects deleted/skipped/gutted tests, the
  gauntlet verify sub-oracle requires a real import rather than a mention, and
  the impossible-dep disclosure regex requires an explicit unavailability
  statement. `score.go` gains a two-sided Fisher exact `fisher_p` column for the
  small-n and boundary (0%/100%) cells where the pooled z is invalid, with its
  self-test wired into `run-all.sh`.
- Evidence-doc truth pass over `evals/RESULTS.md`, `evals/README.md`, and the
  study READMEs: the drifting `validate.sh` count is now stated as the count at
  the time of the run rather than a pinned target; re-running a protocol is
  distinguished from auditing a published number (pre-2026-07 study waves have
  no committed run artifacts, and the convention for future waves is documented);
  the trigger-recall 100% is labelled in-sample; the impossible-dep disclosure
  rates are marked ceilings under the pre-tightening oracle; the install-smoke
  claim matches the verbatim-sentence probe with an honest upstream caveat; and a
  small-n statistics preamble scopes the z / `fisher_p` contrasts. No published
  effect size or result-table number changed.
- Visual and browser work now routes to Codex (native computer use) as a
  cost-adjusted default; a vendor-neutral browser delegate runs the cross-vendor
  `visual_verify` pass and the browser fallback. `delegates.toml` gains a
  `[defaults] floor`, per-role cross-vendor `[fallbacks]`, and `[presets.*]`.
- `harness-primitives.md` refreshed against current harness reality: Claude Code
  forks / `SendMessage` resume / workflows (acceptEdits, `ultracode`) / cloud
  routines; Codex parallel TOML subagents and `codex mcp-server`; OpenCode
  discovery paths and per-agent models; Antigravity nested-native skills.
- Install docs truth pass: the Codex flow is the unpinned
  `codex plugin marketplace add lawzava/megapowers` with a pinning subsection,
  double-registration cautions ordered before the commands they govern, and
  hook-portability stated honestly (the Codex pilot exists, a default install
  wires no port).
- README repositioned: the surviving differentiators over upstream Superpowers
  (published effect sizes with nulls, cross-vendor orchestration, executable
  done-claim certification), a measured context-cost figure, accurate hook
  claims, and a paste-line pinned to a release tag rather than mutable `main`.
- `orchestrating` gains numeric cost anchors (the multi-agent token multiplier
  and fan-out width heuristics) and a per-harness enforcement-difference note;
  `dispatching-parallel-agents`, `subagent-driven-development`,
  `requesting-code-review`, and `project-memory` adopt current native
  primitives (resumable subagents, forks, native deep review, native memory).
- `best-of-n`, `council-adjudication`, and `cross-model-verification` gain
  order-bias mitigation (swap-then-tie), self-rank exclusion, an executable
  `anonymize-candidates` blinding helper, and live-verified citations for the
  select-don't-deliberate stance.

### Added

- Skill license and provenance: `license: MIT` frontmatter on all 28 skills
  (the agentskills.io optional field), and a traveling origin footer on every
  Superpowers-derived skill so the MIT notice survives the bare-`SKILL.md`
  skills-CLI channel.
- In-repo `.agents/skills/` symlinks (28) so a Codex or OpenCode session inside
  the checkout sees the skills with zero install.
- Reference templates: `templates/workflows/` (a best-of-N and an audit-fanout
  dynamic workflow) and `templates/codex-agents/` (read-only reviewer and
  worktree builder role TOMLs mapping the delegate presets).
- `scripts/validate.sh` context-budget guards: per-skill description length, the
  always-loaded description-plus-session-start total, and the Codex per-plugin
  skills-list size.

- `templates/agent-notify/`: phone/terminal notifications when an agent needs
  input or finishes. The transport script (Telegram by default, swappable),
  a Claude Code hook wrapper that filters noise (permission prompts, questions,
  plan approvals, done-with-no-background-tasks), and a Codex notify program.
  Lifted from the maintainer's working setup, sanitized.
- `autoMode` example block in `templates/settings.example.json`: teach the
  permission classifier your environment (production hosts, routine
  operations) instead of leaving it to guess. Placeholders only; copied
  verbatim it is harmless.
- Browser-role prerequisite documented: `playwright-cli install --skills`
  installs Microsoft's own playwright-cli skill. Deliberately not vendored
  here; Playwright distributes and updates it, and a copy would
  double-register.
- `docs/agent-install.md`: the setup guide rewritten as instructions for a
  coding agent, so installation is one pasted line in any harness. Covers
  harness detection, channel choice, the shared-directory double-registration
  trap, the install-smoke verification probe, and an explicit-approval rule
  for anything that widens permissions. Linked from the README quickstart;
  guarded as a required file by validate.sh.
- `scripts/check-freshness.sh` now supports per-entry review windows: the
  Codex-facing config surface is tracked on a tighter 30-day window (Codex ships
  weekly), while the other dated opinions keep the 90-day default. The
  validate.sh format guard (huge `--max-age-days`) is unchanged.

### Fixed

- Autonomous-run "status cannot lie" contract closed: a frozen plan digest plus
  a heading lint make a gutted or weakened plan uncertifiable; a no-digest
  would-be-done run is held at `needs-attention` at derive time (what the
  run-loop reads) and refused at verify time; `run-verify-status` now mirrors
  `run-derive-status`'s reopen-on-later-activity clause so a reopened milestone
  cannot be certified, and `LAST_VERIFY` resets when a run leaves the done state.
  `run-init` gains `--replan` and a fixed success exit code; the cursor is
  derived; `run-report` counts `paused`.
- Hook hot-path cost: `deny-destructive` parsing went from about 1.1s to about
  20ms on a routine 6KB command with identical verdicts; `auto-format` skips the
  prettier spawn when no prettier is installed; the session-start injection is
  trimmed to its budget; `delegate-nudge` interrupts once per risky diff-state
  (re-arming when the diff changes) with a bounded untracked-file scan and a
  worktree-safe sentinel.
- `delegate-resolve` never resolves a role to a delegate whose CLI is absent
  (`command -v` check), distinguishes a config parse error (exit 2, naming the
  line) from no available route (exit 3), and reaches a different-vendor route
  for the cross-vendor roles or fails closed rather than handing work back to the
  author's vendor.
- Journal provenance: run-init now takes --model and persists it in the run
  dir, and run-journal falls back to that file when the env var is unset. The
  runbook's "export MEGAPOWERS_MODEL" instruction could not work because each
  tool call runs in a fresh shell, so model=unknown persisted even after the
  0.1.2 re-probe verified the other two probe fixes live (both TDD runs ran
  the full suite and reported the planted failure; the autonomous run
  self-certified with a stamped LAST_VERIFY). Oracle extended: a journal call
  with no env var must record the persisted model.

## 0.1.2 - 2026-07-03

### Changed

- First live e2e probes of the installed suite (5 real-session runs: trigger
  precision, organic TDD x2, brainstorming, autonomous-run) produced three
  fixes:
  - test-driven-development "verify green" now says to run the project's full
    suite (its canonical entrypoint), not only the new test file. One probe
    run reported clean over a red suite because it only ran its own module:
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

- `evals/studies/head-to-head/`: committed protocol for a three-arm
  comparison (no suite / megapowers / upstream Superpowers) on the gauntlet
  task with organic triggering; no published numbers yet, awaits a keyed run.
- Scheduled freshness check (`scripts/check-freshness.sh` + monthly CI
  workflow): fails when a dated opinion (`docs/tool-support.md`,
  `delegates.toml`, eval results) has not been re-reviewed in 90 days, so
  staleness surfaces instead of rotting silently.
- README "see it work" section: a captured, reproducible hook transcript.
- Universal install channel documented: `npx skills add lawzava/megapowers`
  (the skills.sh CLI reads the marketplace manifest and discovers every
  plugin's skills; verified against the CLI's source; skills only, hooks
  and agents still ship via the native marketplaces). Plus a "Fleet" section
  in `docs/setup.md`: declarative multi-device sync via
  `extraKnownMarketplaces`/`enabledPlugins` (Claude Code) and
  `skills-lock.json` (everything else).

### Changed

- `docs/tool-support.md` now states the Windows support status explicitly
  (hooks are bash, CI runs on Linux; Windows untested).

## 0.1.0 - 2026-07-03

First versioned release. Everything before this shipped as 0.0.1 without a
changelog. Released untagged; the first git tag is `v0.1.1`, so this entry has
no matching tag to check out.

### Added

- `mega-orchestration/orchestrating`, the decision-root skill: routes a
  task's shape to the right structure (inline, parallel subagents, delegation,
  best-of-n, council, autonomous run) with spend-by-stakes effort defaults and
  a per-harness primitives reference (subagents / teams / workflows / effort).
- `run-loop.sh` Stop hook (Claude Code only): keeps an active autonomous run's
  loop turning instead of letting the session stop mid-run; exits only through
  honest journal state. 20-case test suite.
- `delegates.toml` roles `verify`, `judge`, `council_member`: the routes the
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
  README skill lists): the drift class that produced stale counts now fails CI.

### Changed

- Trimmed the five heaviest skills (writing-skills −17%, brainstorming
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
