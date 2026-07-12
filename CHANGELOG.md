# Changelog

All plugins version together; the version in each Claude and Codex plugin
manifest (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`) matches
the repo release. The minimal Antigravity root manifests carry no version
field by design (their schema allows only name and description). Format:
[Keep a Changelog](https://keepachangelog.com), semver.

## Unreleased

## 0.3.4 - 2026-07-13

### Fixed

- `templates/codex-config.toml` no longer emits the removed
  `commit_attribution` key. Commit-trailer policy remains in `AGENTS.md` and
  repository Git hooks, where current Codex versions actually honor it.

## 0.3.3 - 2026-07-13

### Fixed

- Codex config guidance now matches the current CLI contract: named profiles
  are separate `$CODEX_HOME/<name>.config.toml` files. The Sol `ultra` example
  introduced in v0.3.2 moves out of the invalid `[profiles.complex]` table and
  into `templates/codex-complex.config.toml`.

## 0.3.2 - 2026-07-13

### Added

- Codex-native hook dispatch from the existing plugin manifests: megapowers
  injects the model catalog at SessionStart, mega-orchestration runs the
  independent-review nudge at Stop, and the newly published Codex
  mega-guardrails bundle runs the destructive-command adapter at PreToolUse.
  Claude-only run-loop and formatter payloads no-op under Codex.
- Terra-pinned Codex `builder` and `reviewer` role profiles, packaged inside
  mega-orchestration as installable assets as well as root templates. Builder
  refuses edits unless the lead dispatched it into a dedicated linked worktree.

### Changed

- Claude Fable 5 delegation now defaults to isolated, stateless one-shots with
  `--safe-mode --no-session-persistence`; read-only reviews add plan mode and
  an explicit read-only tool set. The catalog names Claude as the planning,
  verification, and judging companion at a deliberate `high` effort ceiling.
- Codex lead guidance now distinguishes Sol lead work, Terra native fan-out,
  and Fable plan/verification passes; documents an optional Sol `ultra`
  profile, conservative native-agent limits, app-server refresh checks, hook
  trust, duplicate cleanup, and v0.3.1 manual-hook migration.

### Fixed

- Claude permission-template secret denies use exact paths instead of wildcard
  forms the harness does not interpret, and `.firecrawl/` local research state
  is ignored.
- Validation now locks the Codex marketplace count, role model pins and plugin
  packaging, safe Claude channel flags, secret-deny syntax, and local research
  hygiene.
- The three legacy manual `codex-hooks.json` pilot manifests are removed now
  that the normal plugin manifests dispatch by harness, preventing accidental
  duplicate hook wiring after upgrade.

## 0.3.1 - 2026-07-12

### Changed

- The shipped catalog now declares codex as lead (gpt-5.6-sol, frontier) with
  claude as the cross-vendor delegate: plan_review/verify/judge/council_member
  route to claude, claude's dispatch effort is capped at high by policy, and
  the antigravity provider is removed. Claude Code leads declare themselves in
  an override layer (`[lead] provider = "claude"`); templates/CLAUDE.md says
  how.

### Added

- Review-role fallbacks: plan_review and code_review carry cross-vendor
  `[fallbacks]` chains, so `delegate-resolve <role> --exclude-lead` resolves
  reviews away from the lead's vendor under either lead. Lead-swap tests pin
  both directions.
- templates/CODEX-LEAD.md: a Codex-as-lead AGENTS.md charter (lead
  declaration, session catalog, delegation routes, single-writer, hook
  caveats). templates/codex-config.toml pins the catalog's frontier model.
- Codex hook pilot ports (manual wiring, trust-gated, fail-open; see
  docs/setup.md): a SessionStart adapter injecting the rendered model catalog
  (megapowers hooks/codex-session-catalog.sh) and a Stop manifest running
  delegate-nudge.sh, whose delegate detection now also matches Codex rollout
  transcripts (both observed `cmd` serializations) in the config-driven regex
  and the static fallback.

### Fixed

- delegate-resolve: exit 4 (provider disabled) is reserved for single-candidate
  routes and reports the actual sole candidate; a fully-disabled multi-candidate
  chain exits 3 (no available route).
- references/providers/claude.md documents pinning effort via `claude --effort`
  (the CLI speaks the catalog's low/medium/high/xhigh/max scale unmapped).

## 0.3.0 - 2026-07-12

### Changed

- models.toml refresh (verified against live sources 2026-07-12): Haiku pinned
  by alias (`claude-haiku-4-5`), the codex provider gains the full GPT-5.6 tier
  ladder (sol/terra/luna, GA 2026-07-09), and the cost hint trued to ~2x at
  current list prices.
- `[efforts]`: a second vendor-neutral scale (low/medium/high/xhigh/max) with
  per-effort purposes and per-provider `efforts` subsets. The floor's effort
  half now validates against it (exit 2 unknown; `--check` finding), and the
  session-start block renders the efforts ladder (block budget 900B).

- models.toml: the model catalog (lead, tiers with per-tier purposes, providers,
  floor) split out of delegates.toml, layered project > user > shipped, shipped
  as identical twins in both plugin roots (CI-asserted). delegates.toml keeps
  roles, requires, fallbacks, and presets; pre-0.3 override files with inline
  provider sections keep working and win over the catalog.
- Every session now starts with a rendered model-catalog block: megapowers
  session-start runs hooks/render-model-catalog (fail-open, <=600B), so tier and
  delegate choices need no skill invocation.
- delegate-resolve: --models flag and MODELS_TOML env pin the catalog stack;
  --where lists both stacks; --check validates across both. delegate-nudge reads
  detect markers from catalog layers too.

## 0.2.0 - 2026-07-12

### Changed

- delegates.toml is now the model-agnostic source of truth: `[lead]` declares the
  orchestrator, `[tiers]` defines a vendor-neutral scale (fast/strong/frontier),
  providers carry tier maps, capabilities, detect markers, and reference files.
- delegate-resolve: layered config (project and user overrides win per key over the
  shipped file), `--lead`, `--exclude-lead`, `--where`, `--check`, TIER output,
  capability and floor filtering; tested by scripts/tests/delegate-resolve.test.sh.
- Skills and agents de-branded: prose speaks roles and config keys;
  `model-delegate` replaces `codex-delegate`; per-provider channel and prompting
  guidance moved to `references/providers/`.
- delegate-nudge.sh derives its delegate-detection patterns from delegates.toml
  `detect` keys, with the old static regex as fail-open fallback.

## 0.1.10 - 2026-07-11

### Changed

- `scripts/validate.sh` mirrors CI's native plugin-validate job: it runs
  `claude plugin validate --strict` on the marketplace manifest and every
  plugin when the claude CLI is installed (skipped with a pointer to the CI
  job otherwise). Closes the local-pass/CI-fail gap that forced the v0.1.7
  re-tag.

## 0.1.9 - 2026-07-10

### Changed

- Codex delegate route: gpt-5.5 to gpt-5.6-sol, with a new per-provider
  `effort` key ("high") in delegates.toml that `delegate-resolve` emits as
  `EFFORT=`. The builder subagent template moves from medium to high effort to
  match; the reviewer template stays at xhigh. The visual-routing bench
  numbers in delegates.toml are marked as measured against gpt-5.5 (no
  re-bench).
- The codex-delegate agent covers the MCP channel natively: it lists the
  `mcp__codex__codex` / `mcp__codex__codex-reply` tools and prefers them when
  present, because a sandboxed lead cannot auth `codex exec` or the SDK (the
  command sandbox denies `~/.codex/auth.json`) while the harness spawns the
  MCP server outside that sandbox. The caveat is documented in the delegation
  skill, delegates.toml channel notes, harness-primitives, and
  harness-support.

### Added

- `templates/codex-mcp-settings.json`, a starter MCP registration for
  `codex mcp-server` (register as `codex` so the tool names match the agent's
  tool list).

## 0.1.8 - 2026-07-08

### Changed

- Fable 5 de-prescription wave 2: sixteen process and orchestration skills
  rewritten for frontier models (goals and constraints over enumerated
  procedure), 16036 to 10319 words total (36% smaller). Descriptions are
  byte-identical to v0.1.7. Skills: multi-agent-delegation,
  finishing-a-development-branch, brainstorming, writing-plans,
  using-git-worktrees, dispatching-parallel-agents, receiving-code-review,
  best-of-n, orchestrating, requesting-code-review,
  verification-before-completion, council-adjudication,
  cross-model-verification, executing-plans, effect-broker, project-memory.
  Keyed eval gate passed (claude-fable-5 and gpt-5.5 arms, 21 of 22 gate cells
  equal baseline and one within noise on a frozen skill); see the wave 2
  section in `evals/RESULTS.md`. orchestrating's stop-budget wording now
  matches autonomous-run's charter row.

### Added

- Two process-behavior eval probes: `deploy-consent` (irreversible-action gate,
  source skill effect-broker) and `brainstorm-first` (premature-implementation
  gate, source skill brainstorming), plus a redesigned long-horizon
  orch-autonomous trigger-recall prompt.

## 0.1.7 - 2026-07-07

### Fixed

- `designing-frontends`: the description's unquoted inner colon failed strict
  YAML parsing, so the skill loaded with empty metadata (no trigger); the
  description is now a block scalar. v0.1.6 ships the broken frontmatter (its
  plugin-validate CI job is red, though the run reads green because that job
  is advisory); use v0.1.7 for `mega-frontend`.

## 0.1.6 - 2026-07-07

### Added

- `mega-frontend` plugin (seventh): one skill, `designing-frontends`,
  adapted from Anthropic's frontend-design (Apache-2.0), rewritten and
  renamed; its calibration of current AI-default looks carries a
  `Calibration reviewed:` date checked by `scripts/check-freshness.sh`.
- `mega-guardrails`: whole-tree `git checkout`/`git restore` discards join
  the ASK tier (`.`, `./.`, `:/`, bare-glob and `:(top)`-magic pathspecs);
  branch switches, `--staged`-only restores, and scoped paths stay allowed.
  Hardened through a three-round adversarial review loop; 31 new fixtures.
- Reviewer template (`requesting-code-review`): agent-era failure checks
  (LLM output trust boundary, enum completeness traced through consumers,
  1-indexed model answers) and a do-not-flag noise list, from gstack (MIT).

- `humanizing-prose` skill (megapowers plugin): strip AI tells from
  user-facing prose, scoped to a measured frontier baseline (em/en dashes,
  sales punchlines, default rule-of-three); adapted from blader/humanizer
  (MIT), provenance in ATTRIBUTION.md. Two matching style bullets in
  `templates/CLAUDE.md`, both arms micro-tested (control 4/5 dashed, with
  bullets 0/5).
- `references/prompting-codex.md` in multi-agent-delegation: contract-block
  prompting for Codex delegates, an adversarial review template, and a JSON
  review-output schema for `codex exec --output-schema`; adapted from
  OpenAI's codex-plugin-cc (Apache-2.0).
- Description-optimization loop documented in writing-skills
  (testing-skills-with-subagents.md): near-miss negatives, 3 reps per query,
  held-out selection; adapted from Anthropic's skill-creator (Apache-2.0).

### Changed

- Context-economy rationale stated where it binds: orchestrating (finite
  attention budget), writing-skills (smallest set of high-signal tokens),
  and dispatching-parallel-agents (documents travel as paths plus an
  instruction to read them, micro-tested 5/5 vs a 0/5 control).
- `brainstorming`: option effort is presented on both scales (human-team
  time and agent time), micro-tested 5/5 vs a 0/5 control; adapted from
  gstack's decision-brief format.
- `evals/README.md`: control-arm methodology (skill vs terse control, not
  vs bare baseline), adapted from caveman (MIT); `writing-skills` notes
  that invented abbreviations save no tokens.

## 0.1.5 - 2026-07-07

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
