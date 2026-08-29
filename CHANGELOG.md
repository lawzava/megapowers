# Changelog

All plugins version together; the version in each Claude and Codex plugin
manifest (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`) matches
the repo release. Format: [Keep a Changelog](https://keepachangelog.com),
semver.

## 0.23.0 - 2026-08-29

### Fixed

- The Windows hook wrapper (`run-hook.cmd`) now propagates child hook exit
  codes instead of reporting stale success.
- The destructive-command guard denies piped and wrapped destruction
  (`xargs`, `timeout`), raw-device writes (`truncate`, `dd`, `cp` against block
  devices), the `/private/etc` and `/private/var` twins, other users' tilde
  homes, `$HOME/..` escapes, and named fork bombs, with new allow-by-design
  guards against false positives.

### Added

- A PowerShell-tool leg: `hooks.json` matches `Bash|PowerShell`, routing
  PowerShell calls through the same shared guard and patterns
  (`Remove-Item -Recurse`, `rd /s`).
- Evals: acceptance binds paired runs to the full comparison identity;
  behavioral rows require 40/64-character revisions and named `manifest`/`rows`
  artifacts; subprocess execution is bounded with process-group teardown; the
  sandbox broker fails closed on socket paths beyond the 100-byte bound and
  passes its contract under long `TMPDIR`; credentialed PR replay refuses to
  run until its oracle executes through the broker.
- CI: a Windows hook-test leg, a Codex install-smoke lane, a scheduled
  latest-version lane, a test-inventory parity gate, loud `SKIPPED` warnings,
  UTC-explicit freshness, a pinned Go 1.25 minimum with advisory staticcheck,
  Dependabot for SHA-pinned actions, and a tag-triggered release-attestation
  workflow.

## 0.22.0 - 2026-08-27

### Changed

- `orchestrating` now scans task lanes before deep work, delegates one bounded
  output-only investigation, batches independent read-heavy lanes, and uses
  native team or task coordination for four or more durable lanes. It also
  preflights child capabilities, limits inherited context, requires bounded
  structured returns, uses milestone handoffs and delta-only follow-ups, and
  controls nested delegation explicitly.
- The orchestration execution map documents direct-agent, durable-team,
  fresh-context, long-wait, and single-writer boundaries for Claude Code and
  Codex without adding a scheduler.

### Added

- Installed A/B evaluation now separates latent three-lane fan-out, bounded
  output-only work, and dependent inline work. Its fail-closed gates reject
  serial fan-out, failed or excess spawn attempts, missing completions, raw
  payload returns, invalid return schemas, oversized results, and unnecessary
  inline delegation.

## 0.21.0 - 2026-08-26

### Added

- `independent-review` requires explicit user egress authorization before an
  external dispatch and surfaces a dispatch stalled on a permission prompt or
  provider instead of waiting silently. Both rules come from the 2026-08-23/26
  session audit: a harness guardian denied two unapproved cross-vendor
  dispatches, and one dispatch stalled ~12 hours on a canceled prompt.
- `verify-and-finish` sweeps the diff for generated excess before a handoff or
  PR, and its trigger names headless and SDK-driven sessions, where the skill
  never fired despite five PR deliveries in the audited window.
- `mcp-setup` prints configuration keys, never values, after a live diagnostic
  echoed a database credential into a transcript.
- `safe-effects` keeps outward artifact names and text inside the approved
  disclosure; `autonomous-run` names provider limits and expired credentials
  as blocked states to surface once rather than wait out silently.
- `grill-me` declares interview rounds requested depth so the shipped prose
  caps do not conflict with full-frontier rounds, and gains its previously
  missing content regressions in `skill-contracts.test.sh`.
- `security-lint` flags machine-specific home paths outside the documented
  fictional fixture homes; `validate.sh` runs `go vet` per Go tool and the
  banned agent-workspace paths join the native-first removed-path contract.
- The docs contract derives the skill count and experimental set from
  `skills/catalog.json`, ending hand-maintained count drift.

### Changed

- `code-quality` triggers on decisions the repository does not settle instead
  of enumerating sibling skills' trigger nouns; `orchestrating` drops
  "high-stakes" from its trigger.
- Codex marketplace metadata gains the plugin description, and the upgrade
  channel reference warns that Codex `config.toml` marketplace metadata can
  lag the actual snapshot.
- Stale skill counts corrected in `README.md`, `evals/RESULTS.md`, and
  `.agents/skills/README.md`; README's experimental list now includes
  `grill-me`.

### Removed

- The dead `.megapowers/enforcement.toml` project layer (its enforcement
  engine was removed in 0.15.0) and its `.gitignore` re-include.
- The scenario runner (`evals/run.sh`, its fail-closed test, the empty
  scenario loop in `run-all.sh`, and the always-zero scenario column in the
  coverage inventory). No scenario ever shipped; the machinery returns with
  the first real one.

## 0.20.0 - 2026-08-26

### Added

- The experimental `memory-hygiene` skill audits Claude Code and Codex memory
  through a read-only manifest, previews one evidence-bound patch, and applies
  it automatically after exact user approval. Its Go validator rejects
  unsupported retained evidence, stale volatile facts, missing provenance,
  malformed dates, conflicts hidden by duplicate IDs, symlinks, and common
  credential patterns.

### Changed

- Skill inventory contracts move from fourteen to fifteen skills, and the
  total guidance budget moves from 4300 to 4600 words.

## 0.19.0 - 2026-08-25

Megapowers adds a round-based grilling interview as a fourteenth skill.

### Added

- `grill-me` stress-tests a plan, decision, idea, or draft through a
  round-based interview. It maps decisions as a tree, asks the unblocked
  frontier each round with one recommended answer per question, restates
  settled decisions and open branches every round, resolves facts itself,
  and grants no execution authority. Upstream credit is in ATTRIBUTION.md.

### Changed

- Skill inventory contracts move from thirteen to fourteen skills, and the
  total guidance budget from 3900 to 4300 words.

## 0.18.0 - 2026-08-25

Megapowers adds dependency-free behavior specifications to planning and
completion while preserving the native-first workflow.

### Changed

- `design-and-plan` creates proportional Markdown behavior contracts for
  non-trivial observable changes. Contracts separate requirements from
  implementation and include scope, scenarios, acceptance oracles, change
  deltas, and stale-specification reconciliation.
- `verify-and-finish` reconciles affected repository-owned behavior
  specifications with verified behavior before completion. It does not create
  durable specifications without repository convention or user approval.

## 0.17.1 - 2026-08-23

### Changed

- The shared output style now claims precedence over built-in harness
  communication and formatting guidance where they conflict. Probes showed
  both harnesses inject the style correctly, but conflicting built-in prose
  guidance survives alongside it; `keep-coding-instructions: false` was
  evaluated and rejected because it removes the task instructions while
  leaving the conflicting sections in place.

## 0.17.0 - 2026-08-23

This release targets the friction observed in real sessions since v0.16.0:
MCP plumbing, hidden review-provider failures, sandbox false negatives,
branch mismatches, and refusal loops under direct supervision.

### Added

- The experimental `mcp-setup` skill covers MCP server scope, the
  restart-before-new-tools rule, headless OAuth limits, fresh redacted
  verification probes, and sandbox-induced false failures.

### Changed

- `systematic-debugging` treats sandbox and permission restrictions as
  suspects and requires re-running a probe outside the restriction before
  declaring a tool broken.
- `verify-and-finish` confirms the checked-out branch matches the task's
  named target branch before a commit.
- `safe-effects` stops repeated refusals under direct interactive
  supervision: confirm the boundary once, then execute inside it.
- `upgrading-megapowers` expects registration to prune superseded caches and
  snapshots a cache that live sessions may still use before registering.

### Fixed

- The independent-review tool now allocates the receipt run directory before
  provider dispatch, so an unwritable destination fails before credentials or
  source reach the provider.
- A provider that exits zero with an empty review now surfaces a classified
  stderr diagnostic instead of discarding it, and nonzero exits also classify
  stdout-only diagnostics; authentication needles cover 401 and expired-token
  phrasing.

## 0.16.0 - 2026-08-20

Megapowers adds evidence-led research while tightening workflow routing,
verification, continuity, and technical prose.

### Added

- The experimental `evidence-research` skill classifies support for
  load-bearing claims and keeps research separate from implementation or
  publication authority.
- A lifecycle catalog records stable and experimental skills without extending
  portable skill frontmatter.
- Installed-plugin workflow diagnostics measure ordered skill selection,
  forbidden activations, required events, continuity routes, and approval
  boundaries.
- A content-minimized session-observability study and verification-map pilot
  support maintainer investigation without adding a runtime scheduler.

### Changed

- Planning, orchestration, autonomous work, code quality, effects, testing,
  review, and completion guidance now use stronger authority and evidence
  boundaries.
- Shared prose guidance adopts accountable attribution, concrete evaluation,
  and calibrated claim strength while preserving already-direct text.
- The primary skill guidance budget increases from 3,000 to 3,600 words for
  the twelfth skill and the strengthened workflow contracts.

## 0.15.4 - 2026-08-18

Megapowers makes direct, concise technical replies the default on Claude Code
and Codex without changing global user configuration.

### Added

- Claude Code receives a forced plugin output style with answer-first,
  ASD-STE100-inspired guidance and explicit prose budgets.
- Codex receives the same style as startup developer context through a bundled
  hook after the user reviews and trusts the plugin hooks.

### Changed

- The shared style preserves exact identifiers, commands, numbers, caveats,
  decisions, and material uncertainty while removing routine narration.
- Validation now checks both native adapters, shared-source parity, hook scope,
  documentation, and executable packaging.

## 0.15.3 - 2026-08-18

Delegation can use one personal capability registry across local harnesses
without restoring plugin-owned model routing.

### Added

- `orchestrating` reads an optional
  `~/.config/megapowers/agent-capabilities.md` registry before delegation.
- The documented registry schema separates portable role requirements from
  harness-native model, effort, access, and availability bindings.

### Changed

- Agent selection ranks only native bindings with known model and effort,
  choosing the fastest and then cheapest option that meets the task boundary.
- Manual and approved-external routes remain explicit and unranked; registry
  data cannot grant access, permissions, disclosure, writes, or side effects.
- The primary skill guidance budget increases from 2,850 to 3,000 words for
  the registry contract; the per-skill 400-word cap is unchanged.

### Fixed

- CI installs the `ripgrep` dependency used by deterministic contracts, and
  freshness metadata uses a runner-valid UTC review date.

## 0.15.2 - 2026-08-18

Megapowers restores the agent-driven upgrade workflow removed in `v0.15.0`
without bringing back legacy harnesses, model routing, or multi-plugin setup.

### Added

- `upgrading-megapowers` inspects installed provenance, preserves enabled state,
  source, scope, pins, and local edits, then obtains one exact approval before
  changing the detected Claude Code or Codex channel.
- Floating updates bind the marketplace head to the approved stable release
  commit before writing and verify registration plus exact cached bytes after.

### Fixed

- Codex updates re-register `megapowers@megapowers` after marketplace refresh;
  refresh alone does not replace the installed cache.
- Claude updates preserve the observed installation scope, while both channels
  use their actual source and install-path readback fields.
- Stale cache cleanup waits for active sessions to restart and requires separate
  approval; model-backed loading checks also remain separately authorized.

## 0.15.1 - 2026-08-18

Megapowers restores reliable native delegation, subscription-backed Claude
review, and strict boundaries around outward progress comments.

### Added

- Installed-plugin behavioral cases require three independent agent lanes to
  dispatch before waiting and fail closed on incomplete traces.
- Safe-effects evaluation exposes local tracker and pull-request comment
  helpers, then rejects every attempted unauthorized write regardless of its
  exit status.

### Fixed

- `orchestrating` explicitly authorizes native agents and subagents for two or
  more independent lanes, especially read-heavy discovery and review.
- The Claude review adapter preserves subscription OAuth, disables persistent
  customizations, and maps untrusted provider errors to secret-safe diagnostic
  categories.
- `safe-effects` separates implementation authority from public comment
  authority, while `humanizing-prose` keeps routine progress and test
  transcripts out of trackers and pull requests.

## 0.15.0 - 2026-08-16

Megapowers is now one native-first plugin for Claude Code and Codex. The
release removes overlapping workflows and prompt injection, keeps ten focused
skills, and keeps source-specific behavioral studies as optional diagnostics.

### Added

- `independent-review` packages a disclosure-first Go review tool that binds
  approved source and provider bytes before execution.
- Optional installed-plugin A/B and PR-replay runners use a hash-pinned isolation
  broker, strict schema-v1 evidence, and fail-closed scoring.
- Release preflight verifies the already-versioned clean candidate, rejects
  hidden plugin payloads, and runs the full deterministic validation stack.
- Agent-context contracts reject historical lineage text and cap the ten
  primary skills at 2,500 words total and 400 words each.

### Changed

- Seven plugins and 32 skills become one `megapowers` plugin with ten
  task-level skills: orchestration, design and planning, test-first
  implementation, debugging, verification, safe effects, autonomous work,
  independent review, human-facing prose, and code quality.
- Humanizing prose applies to every agent-authored plan, task brief, commit,
  response, review, PR, document, release note, and error while preserving
  facts and already-direct writing.
- The remaining command guard denies only high-confidence destructive actions;
  reversible-risk approval stays with the native harness.
- Validation is a compact 29-gate pipeline with pinned CI actions, full-tree
  security linting, exact-byte install smoke, and strict Claude validation.

### Removed

- OpenCode, Grok, model routing, session catalog injection, custom schedulers,
  status lines, format hooks, templates, browser agents, project memory, and
  legacy behavioral runners.

## 0.14.0 - 2026-08-16

Every defect the 2026-08 post-upgrade transcript audit confirmed, fixed: the
gates stop eating deliverables and trapping delegates, Grok's adapter ships in
the tree with its dead Stop guard corrected, and remote auth changes stop
crossing the guardrails in silence.

### Added

- The Grok adapter ships in `templates/grok/` (Stop and PreToolUse wrappers,
  hook manifest, lead charter with a delegate clause) with tests that drive
  Grok's real payload shapes. The installed copy's Stop hooks had never fired:
  Grok sends no `reason` field and the guard required one. `[adapters.grok]`
  joins the shipped catalogs.
- `review-ack` records a human's confirmed false-positive dismissal of the
  risky-logic gate, bound to the exact pending tree; any further edit re-arms
  the gate, and the pass it buys is announced, never silent.
- `deny-destructive` classifies the command an `ssh` invocation carries with
  the same tiers as a local one, and asks on account, group, credential,
  sudo-policy, and system-path ACL changes, locally or remote.
- The skill router fires on an explicit `/skill-name` typed anywhere in a
  prompt, and the dash gate now also governs subagent replies (SubagentStop).

### Fixed

- The risky-logic gate no longer displaces the reply it interrupted: every
  block tells the agent to restate its answer first, a dispatched reviewer is
  told the receipt is the dispatching session's to obtain, and a branch whose
  verify loop already hit the round cap gets the human hand-off instead of a
  launcher command that would be refused.
- `delegate-run` retains transcripts by default under
  `.git/megapowers-review-transcripts`, announces "round N of M" on every
  dispatch, and the reviewer prompt requires unexecuted oracles to be named
  as limitations rather than implied test runs.
- `delegate-resolve` retiers an uncataloged `--caller-model` onto a
  co-declared provider or a unique catalog model family instead of refusing.
- `review-package` skips untracked paths git cannot hash (sandbox-masked
  device nodes) instead of aborting the whole package.
- `deny-destructive` no longer prompts on the suite's own merged-worktree
  cleanup (`git branch -D worktree-agent-*`).
- The injection probe's credential marker needs an action verb aimed at the
  credential, so listings and docs naming `.env` or `~/.ssh` stay quiet.
- The session catalog's lead line names the actually running model when it
  differs from the catalog's lead model.
- The unscanned-content notice is said once per tree state instead of every
  stop.

## 0.13.0 - 2026-08-15

Upgrades stop guessing which baseline you adopted, and discovery gets cheap.

### Added

- Shipped baselines (`templates/CLAUDE.md`, `CODEX.md`, `OPENCODE.md`) open
  with a `megapowers-baseline vX.Y.Z` stamp that release.sh restamps each
  release. `upgrading-megapowers` reads the stamp and diffs an adopted
  baseline from the exact shipped ref instead of inferring it; unstamped
  copies fall back to inference, said out loud.
- `orchestrating` routes discovery (search or bulk reads across many files or
  sources) through an evidence-gathering subagent on a cheap tier: curated
  evidence packages with `path:line` cites, never raw dumps, conclusions
  staying with the lead.
- evals: the PR-reproduction study design is recorded as designed and awaiting
  a keyed run: merged PRs reset to their base commit, the agent's diff scored
  deterministically against the files the maintainers touched.

## 0.12.0 - 2026-08-12

Agent glue is Go, including inside a Python or TypeScript repository.

### Added

- mega-go `scripting-in-go`: stdlib `go run` helpers from scratch.

### Changed

- The three harness charters and `using-megapowers` tell agents to write Go
  instead of Python, Node, or multi-line bash. Existing CLIs stay as-is.
  Harness hook entrypoints stay shell; OpenCode plugins stay JavaScript.
- Project-memory helpers, effect-broker, check-freshness, check-enforcement,
  and security-lint now run as Go behind the same command names.

## 0.11.5 - 2026-08-12

### Fixed

- PR automation now keeps review replies in the existing thread and publishes
  only the current decision plus the minimum supporting evidence. The three
  harness charters and the review and prose skills reject progress narration,
  aggregate fix ledgers, repeated CI summaries, and closing recaps; necessary
  top-level status is capped at three bullets, and a re-review trigger stays a
  separate minimal comment.

## 0.11.4 - 2026-08-12

### Fixed

- The injected catalog still opened with `lead: claude frontier (claude-opus-5)` in every
  harness. 0.11.3 gave Codex a correction, but it landed under the whole block, five long
  lines after the claim it corrected, and OpenCode's trailer named caller flags without
  ever saying who was in charge; sessions read line two and believed it. Each adapter now
  passes `--caller <harness>` to `hooks/render-model-catalog`, which renders the running
  harness on the lead line and demotes the catalog `[lead]` to the labelled default for
  sessions that declare nothing. The Claude Code hook declares `claude` too, so a layer
  that points `[lead]` elsewhere no longer tells a Claude session something else leads.
  With the shipped catalog its block is byte-identical to before.
- The rendered-block clamp goes 1024B to 1400B for the longer lead line (the shipped
  catalog renders 1045B with a caller declared; the old clamp would have cut the Routes
  line). The Codex adapter drops its identity sentence for the flag alone, so a Codex
  session's injected payload is smaller than it was in 0.11.3, not larger.

## 0.11.3 - 2026-08-12

### Fixed

- A Codex session was told Claude leads it. The SessionStart adapter injected the
  rendered catalog verbatim, `lead: claude frontier (claude-opus-5)` included, and said
  nothing about who was running; `delegate-resolve` then answered every undeclared route
  with the catalog `[lead]` (`CALLER=assumed-lead`), so `self` roles left the running
  vendor and `DISPATCH=native` named a provider the session was not. The block now
  carries an identity line naming Codex as the lead and the `--caller-provider codex`
  flag that carries it into resolution, which is the parity the OpenCode plugin has had
  since 0.11.0. Provider only, no model id: the adapter is Codex by construction, while
  the model in `~/.codex/config.toml` is a default that a `--model` flag, a profile, or
  an in-session switch overrides, and a stale `--caller-model` exits 2 rather than
  sharpening the route.
- `templates/CODEX.md` presented a `[lead]` override as the way for Codex to lead, which
  predates the 0.8.0 charter. Declaring the caller is what makes the running harness the
  lead; the override only moves the default for sessions that declare nothing.

### Changed

- `docs/setup.md` records Codex's skills context budget, checked on codex-cli 0.147.0:
  past a certain total, session start warns and skill descriptions are shortened rather
  than skills dropped. All seven plugins are 32 skills at roughly 3KB and fit; a 54-skill
  surface across four marketplaces (about 9KB) did not. The Codex-config freshness
  sentinel was re-reviewed against the same build and its date bumped.

## 0.11.2 - 2026-08-11

### Fixed

- `templates/OPENCODE.md` cited its sibling template files (`templates/opencode.json`,
  `templates/opencode-agents/`) in its body. The same file instructs you to install it
  as your global `AGENTS.md`, at which point those citations become live lookups: an
  observed session on an unrelated task walked into the megapowers checkout and burned
  an external-directory prompt. Repo paths now appear only in the install blockquote,
  the charter states that its paths are provenance rather than work items, and
  `scripts/validate.sh` fails if one leaks back into the body.
- `evals/studies/install-smoke/run-smoke.sh` rejected a sentence-initial capital in its
  quote probe. The core-principle clause is committed lowercase mid-line after
  "Core principle:", so a model quoting it as a sentence failed a check whose skill had
  in fact loaded, which is what the codex arm did on both the v0.11.0 and v0.11.1 gates.
  Only the first character is relaxed; the mutation suite still rejects reconstructed
  phrasing and a mid-sentence case change.

### Changed

- `templates/opencode.json` is now a security posture rather than a starting point, at
  parity with `templates/settings.example.json` where OpenCode has the knobs and ahead
  of it where it has more. `permission.read` denies `.env` (as a bare name and as a
  nested path), secrets directories, ssh, aws, gnupg, netrc, pgpass, and private keys,
  while leaving `.env.example` readable. `permission.bash` denies by default and
  allowlists read-only commands. `external_directory`, `webfetch`, `websearch`, and
  `doom_loop` ask. `compaction.prune`, `compaction.tail_turns`, and `subagent_depth`
  serve long-running frontier-tier work; `share` is disabled and `autoupdate` only
  notifies, so a long session is not restarted underneath itself.
- The `permission.bash` map carries an explicit deny tier, because OpenCode's `--auto`
  approves everything that is not denied: an `ask` rule is not a protection under the
  one flag a long autonomous run is most likely to use. History rewrites, force pushes,
  `git clean -f`, `git branch -D`, `git stash drop`, `--no-verify`, `--no-gpg-sign`,
  `git add -f`, pipe-to-shell, and the catastrophic filesystem set stay denied in both
  modes. Verified under `--auto` on OpenCode 1.18.16: `git reset --hard HEAD` denied, a
  `.env` read denied, `git status` and `echo hello` ran unprompted. The trade-off is
  that those commands are also unavailable interactively.

## 0.11.1 - 2026-08-11

### Fixed

- `plugins/megapowers/opencode/session-catalog.js` read the session model id as
  `input.model.modelID`, but the OpenCode SDK's `Model` type carries it as `id`
  (only `chat.message` uses the `modelID` shape). Live sessions were told to
  resolve routes with `--caller-model unknown`, which `delegate-resolve` refuses
  with exit 2, so the injection that exists to make a BYO-model runtime
  declarable emitted a flag that could not resolve. The plugin now reads `id`
  with a `modelID` fallback, and omits the identity line entirely when neither
  is present: an undeclared session falls back to the catalog `[lead]`, while a
  session declaring `unknown` cannot route at all. The test fixtures had invented
  the `modelID` shape, which is why the suite stayed green against a broken
  build; they now use the real type and pin all three cases.

## 0.11.0 - 2026-08-11

OpenCode moves from portable-skill compatibility to a supported lead harness,
at parity with Codex: two plugins, agent role templates, a charter, and a
config template, all mirroring what Codex already had.

### Added

- `plugins/megapowers/opencode/session-catalog.js`
  (`MegapowersSessionCatalog`) injects the model catalog and a caller-identity
  line into the system prompt on every chat request, via
  `experimental.chat.system.transform`.
- `plugins/mega-guardrails/opencode/deny-destructive.js`
  (`MegapowersDenyDestructive`) enforces the DENY tier of the existing bash
  tripwire from `tool.execute.before`.
- `plugins/mega-orchestration/assets/opencode-agents/{builder,reviewer}.md`
  (mirrored at `templates/opencode-agents/`): builder pinned to the moonshot
  strong tier, reviewer to the qwen frontier tier, deliberately different
  vendors, with `reviewer.md` setting `permission: edit: deny` to remove the
  edit tool. Read-only stays a prompt contract: `bash: allow` still reaches the
  filesystem through a redirect or `sed -i`.
- `templates/OPENCODE.md` (the OpenCode charter) and `templates/opencode.json`
  (the config fragment wiring both plugins and the ASK-tier bash patterns).

### Changed

- `delegate-resolve` retiers a `self` route onto the nearest tier the
  caller's own provider publishes, reporting the substitution as
  `TIER_FALLBACK=<requested>-><resolved>` instead of erroring, so a
  bring-your-own-model OpenCode lead resolves every role even when its
  provider publishes a single tier.

Two gaps remain, both pinned to opencode 1.18.16: `permission.ask` does not
exist as a plugin hook, so the ASK tier is delivered declaratively through
`permission.bash` patterns instead, and how those patterns match a compound
command is unconfirmed; and `experimental.chat.system.transform`, which the
catalog injection depends on, is undocumented upstream and may change shape.

## 0.10.1 - 2026-08-10

Independence had one reachable alternate vendor, which 0.10.0 recorded as a
known limitation and named Kimi K3 as the candidate to fix. A fifteen-model
review eval settled the candidacy and two more besides, so the fallback chain
now has four vendors in it instead of one.

The eval: two code-review fixtures carrying thirteen planted defects between
them, plus three implementation tasks with hidden tests, same prompt and same
diffs for every model. Recall out of 13 was claude-opus-5 13, qwen3.8-max 13,
kimi-k3 13, deepseek-v4-pro 12, gpt-5.6-sol 12, grok-4.5 11. The three
implementation tasks saturated: every frontier model passed every hidden test,
including exactly-once execution under 24-way concurrency and crash-safe
writes. Nothing here is a claim about writing code, only about review recall,
which was the only thing that separated these models.

Two results outlived the ranking. Harness moved scores more than model choice
or reasoning effort did: gpt-5.6-sol hung on three of four opencode runs and ran
clean on codex, and claude-opus-5 ran 3x faster in Claude Code than in opencode
at identical recall, while raising effort to xhigh changed one model of four.
And that effect does not generalise, since running the opencode-hosted models
inside codex measured worse. Pin the channel before reaching for a bigger model.

### Added

- `qwen` and `moonshot` providers, hosted by the opencode harness, enabled by
  default. They earn rows under the rule that already governed the table: a
  provider is a harness someone runs, not a model with an endpoint. Independence
  goes from `ALTERNATES=1` to `ALTERNATES=3`, so one vendor outage no longer
  takes cross-vendor review with it.
- `deepseek` and `xai` providers, shipped `enabled = false`. Both carry complete
  tiers, measurements and notes; one line in an override layer routes them. They
  wait because the always-loaded session catalog is clamped to 1024 bytes and
  five providers with prose do not fit, which is a budget decision rather than a
  quality one.
- `mega-orchestration:configuring-model-routes`, which matches the shipped
  catalog to what a machine can actually reach and writes the difference to the
  user's own override layer, and `scripts/probe-routes` behind it: a read-only,
  offline probe reporting harness binaries, catalogued models each harness
  lists, and the resulting `ALTERNATES`. It distinguishes a failed model listing
  from an empty one, so a harness that cannot answer keeps its routes instead of
  losing them.
- A "Model routes" section in `docs/setup.md`, including how to enable the two
  providers that ship disabled.

### Changed

- `upgrading-megapowers` checks routing against the machine before its approval
  gate. A provider the new catalog routes to but this machine cannot reach is
  now an upgrade finding rather than a silent regression. The upgrade never
  edits routing itself; it hands that to `configuring-model-routes`.
- `gpt-5.6-sol` is pinned to the codex channel. Driven through opencode it hung
  on three of four review runs, roughly 36 minutes each producing a preamble and
  no findings, and the run that completed scored 5 of 13 against the 12 it
  scores natively.
- `gpt-5.6-terra` is documented as sol's fast fallback rather than a peer: one
  finding behind across the fixtures, missing the wrapped-error class sol
  catches, at 2.4x sol's speed.
- The session catalog budget moved from 900 to 980 bytes in both guards, still
  44 bytes inside the hook's own 1024-byte clamp, because the catalog went from
  two vendors to four.

## 0.10.0 - 2026-08-09

Alignment pass against four independently compiled August 2026 harness/model
research reports and the primary sources behind them: Artificial Analysis's
Coding Agent Index, tbench.ai Terminal-Bench 2.1, arXiv 2603.12123
(Cross-Context Review), and METR's GPT-5.6 Sol evaluation. Most of what those
reports prescribe was already shipped here; these are the places they disagreed
with the config or found it silent.

Then a pre-publish pass over the whole repository: a routing bug that only
showed up where the provider CLIs are absent, the accumulated duplication in the
always-on templates, and the design record that had grown into the config files
it documented.

### Added

- `delegate-resolve --allow-context-separation` and an `INDEPENDENCE` route
  field. Independence had exactly one reachable alternate vendor, so an outage
  took independent review with it entirely. The controlled evidence says that is
  backwards: the study behind independent review varied CONTEXT, not model
  (fresh-session artifact-only review at 28.6% F1 against 24.6% for same-session
  self-review, and 23.8% when the reviewer is handed the generation transcript,
  worse than doing nothing). Cross-vendor review is a motivated prior about
  uncorrelated blind spots, not a measured result. The flag therefore lets an
  unreachable cross-vendor route fall back to a fresh same-vendor session,
  labeled `INDEPENDENCE=context-separation` on the route and in the receipt.
  It is opt-in, never automatic; `judge` refuses it, because a fresh session
  does not remove self-preference from a blind ranking; and the receipt records
  `independent: false`, so the risky-logic Stop gate on auth, billing, payment,
  and concurrency keeps requiring the cross-vendor pass. That gate's scope is
  exactly the class where the vendor prior is worth paying for.

  Route-contract note: `INDEPENDENCE` is emitted on every independence-role
  resolution, with or without the flag, the same way `ALTERNATES` is. Without
  the flag, route selection and exit codes are unchanged, but stdout carries one
  additional line. That is deliberate rather than incidental: a consumer should
  be able to assert the strong claim positively instead of inferring it from an
  absent field, and the field's absence is reserved to mean "a resolver that
  predates it", which is how `delegate-run` reads it.
- Receipt schema: an optional `independence` field (`cross-vendor` |
  `context-separation`), with `independent` relaxed from `const: true` to a
  boolean bound to it by an `allOf`. Absent means cross-vendor, so every receipt
  written before the field keeps validating and no consumer can spell the
  degraded tier as the stronger claim.
- An acceptance-oracle ownership rule in `orchestrating`,
  `subagent-driven-development`, and the implementer prompt: a delegate never
  writes the test it is judged by, and a patch editing that test alongside the
  code goes back. Models optimize against a reachable verifier rather than the
  goal, and the executor-tier models are where that is measured hardest.
- A context rule in `autonomous-run`: budget the working set well under the
  nominal window and treat a reset that reloads the file contract as the normal
  way to continue, not as recovery. A session near what it believes is its limit
  wraps up prematurely, and compaction carries that pressure across where a
  reset does not. The journal and derived status already existed to make this
  cheap; nothing said to use them that way.

### Changed

- `max` is removed from the Claude provider's effort list. Artificial Analysis
  measures Opus 5 at max scoring below its own xhigh on composite index, cost,
  wall time, and code comprehension simultaneously. A rung worse on every axis
  than the rung beneath it is not an escalation, so the catalog now refuses to
  resolve it rather than merely declining to default to it. Codex keeps its
  `max` rung, which still buys something. `--effort max` by hand remains
  possible and remains a worse setting.
- Fan-out sizing in `orchestrating` is now bounded by where output lands rather
  than by task count: 3 to 5 children whose output returns into the
  orchestrator's context, wider only for bounded summaries written to a file.
  The previous "10 plus for wide research" applied one number to both shapes,
  and the limit is the orchestrator's accumulating context, not child
  availability.
- The third-vendor candidate note in `delegates.toml` now carries an acceptance
  bar rather than only a name: throughput, verbosity, mid-session model-switch
  fragility, and quota stability under load, with the reasoning for each. It
  also records why a hosted open-weight model behind an Anthropic-compatible
  proxy does not qualify, which is the same "a provider is a harness, not an
  endpoint" rule 0.9.1 established.
- The Sonnet 5 price ratio in the catalog is marked as dated: introductory
  pricing ends 2026-08-31, which moves the strong tier from roughly 40 to 60
  percent of frontier cost. The tier survives the change; the number does not.
- `agents/model-delegate.md` is deleted rather than shipped as a tombstone. The
  harness loads every agent definition into the selector, so a retired one
  spent description tokens in every session to describe a no-op and put a name
  in front of the model that the same description then had to forbid. Removal
  is the version of that with no cost; `delegation-routing.test.sh` now pins
  the file as absent.
- The delegation section of `templates/CLAUDE.md` and `templates/CODEX.md` is
  cut by roughly a quarter. It stated the `--author-*` versus `--caller-*`
  distinction three times and documented the exclusion behavior of a legacy
  config with no `[independence]` section, which the shipped config has. These
  templates are copied into a project `CLAUDE.md`, so every duplicated
  sentence is context spent in every session on that machine.
- Prose assertions in `killlist-antipatterns-absent`,
  `upgrading-megapowers-contract`, and the `validate.sh` scratch-storage check
  now match against the file with newlines collapsed, the convention
  `delegation-routing.test.sh` already used. A line-anchored grep for a
  sentence answers where a paragraph wraps rather than whether a rule is
  stated: reflowing a skill flipped five `present` checks to failing, and an
  `absent` check would have gone quiet the moment a banned sentence wrapped.
- Markdown body prose in 37 shipped files is rewrapped to a single column
  budget, and dash punctuation is gone from every shipped skill, agent, prompt
  template, and reference, which the repository's own `using-megapowers` rule
  and `auto-format` hook already required of them.

- The risky-logic gate's design record moved out of the config and the hook into
  `plugins/mega-orchestration/hooks/risky-logic-gate.md`: the audit numbers, the
  four rejected `exclude_globs`, the seven removed marker-table extensions with
  the proof each failed, and the five audit rounds in order. `enforcement.toml`
  goes from 239 lines to 95 and now carries what an editor needs at the point of
  edit; `delegate-nudge.sh` drops 165 lines, including one 20-line invariant that
  was spelled out twice. Executable content in both files is byte-identical, and
  the comments that state a line's correctness property stayed with the line:
  this gate has been re-broken by simplifying a narrowing into a hole, so the
  reasons live next to the code that depends on them.

### Fixed

- An override layer setting `providers.<p>.binary` was read, parsed, and then
  dropped. A shipped provider declares an `adapter`, and `adapter_val` sourced
  every value from that adapter section whenever the declaration existed, so
  the provider-keyed form models.toml's own header promises still works could
  not reach `binary` or `channel`. Nothing caught it for two reasons worth
  keeping: the shipped catalog sets both keys to the same value in both places,
  and on any machine with the real CLI installed the route still resolved, just
  to the binary the layer was trying to replace. An explicit provider-level
  value now wins and the adapter supplies the default. This is what turned 19
  `delegate-resolve` tests red on CI while they passed locally.
- `docs/setup.md` cited `v0.8.2`, a version that was never tagged; the
  `sandbox.credentials` object form shipped in 0.9.0. The release-certification
  and install-smoke commands no longer carry a stale `v0.5.0` example, and the
  pin-range sentence names `git ls-remote` instead of a version the next
  release invalidates.
- The `deny-destructive` fixtures no longer carry the maintainer's real home
  directory, which `AGENTS.md` forbids in shipped artifacts.
- `providers/opencode.md` carried a `Last reviewed:` date that
  `check-freshness.sh` did not read, so it could rot without failing the weekly
  job. It is on the list now, beside `codex.md`.

## 0.9.1 - 2026-08-07

Alignment pass against 2026 primary sources: Anthropic's harness-design,
containment, auto-mode, evals, and infrastructure-noise engineering posts,
OpenAI's GPT-5.6 prompting guidance, and the current Claude Code skills
reference.

### Added

- `mega-guardrails`: `scan-tool-output.sh`, a PostToolUse injection probe over
  WebFetch, WebSearch, Bash, and MCP results. Eleven marker classes; on a hit it
  adds one `additionalContext` reminder that tool output is data, never
  instructions. It never blocks and never rewrites. This is the input layer the
  plugin was missing: the other two hooks guard what the agent is about to do,
  and nothing guarded what it had just read.
- `score.go`: `pass^k` beside the pass rate. A pass rate says whether a skill
  usually binds; `pass^3` says whether a session can rely on it, which is the bar
  a discipline skill is for.
- `validate.sh`: post-compaction context budgets. Claude Code re-attaches only
  the first 5,000 tokens of each invoked skill under a combined 25,000-token cap,
  so a long body is truncated mid-session and a deep stack drops its earliest
  skill. Both are now pinned.

### Changed

- A provider is a harness, not an endpoint. The catalog holds no base URLs, API
  keys, or token names, and the comment above `[providers]` now says so: a model
  reachable only by hand-wiring an endpoint and a token is out of scope, and
  adding a provider means adding a harness. Moonshot is removed from the catalog,
  the fallback chains, the freshness list, and the reference set, along with an
  attempt to route it by splitting the adapter switch onto a `dialect` field.
  That attempt was withdrawn after two rounds of cross-vendor review found six
  ways for a route to name one vendor while dispatching to another; each fix was
  another denylist entry and severity rose round over round.
- Independence is single-route again and says so plainly in `delegates.toml`:
  anthropic-authored work has exactly one reachable reviewer vendor, reported as
  `ALTERNATES=1`. The fix is a third harness, not a third endpoint.
- `evals/README.md`: a noise floor for real-agent numbers. Infrastructure
  configuration alone moved Terminal-Bench 2.0 by 6 points in Anthropic's
  measurements, so a sub-3-point difference is noise unless configurations are
  documented and matched. The large results are unaffected.
- `docs/harness-support.md`: skill content lifecycle and the Claude Code
  frontmatter extensions beyond the six-field portable set.

### Removed

- Google Antigravity as a target. Supported harnesses are Claude Code, Codex,
  and OpenCode. Gone with it: the six root `plugin.json` manifests, the
  Antigravity blocks in `validate.sh`, the `agy` arm of the install-smoke study,
  the `antigravity-tools.md` skill reference, and the harness-support and
  harness-primitives sections. Skills are portable markdown and another harness
  may still load them, which is not the same as being supported or tested here.
  The published install-smoke count is left as measured, with its scope dated
  rather than rewritten.

### Fixed

- Three deterministic eval oracles that went red during 0.9.0 and shipped that
  way, against a `RESULTS.md` claim of 16/16. All three were stale oracles rather
  than broken skills, which is the failure mode a regression suite is supposed to
  make loud: `delegate-resolve` pinned `visual_verify` at `EFFORT=high` after the
  audit routed bounded roles at medium; the anti-pattern killlist matched one
  sentence about commit authority that was restated without being changed; and
  the reusable-workflow version ban flagged "the pre-0.8.2 bare-string form",
  which dates a format change rather than pinning a release. The version ban now
  strips historical spans rather than whole lines, so a real hardcoded version
  cannot hide beside one, and it strips only the `pre-X.Y.Z` form: an
  intermediate cut also accepted since/before/after/as of/from, which let
  "install from v9.9.9 and keep that release pinned" through. The oracle now
  self-tests that filter against those pins before trusting it, because a
  too-permissive strip turns the whole check into a no-op that still reports
  PASS.

## 0.9.0 - 2026-08-06

### Known issues

Eight rounds of independent cross-vendor review ran against this work. These are
what survived, disclosed rather than closed, because the gates here are accident
backstops and not security boundaries: a gap that needs a crafted payload is
worth naming, while chasing it costs the precision that makes a gate usable.

The risky-logic gate:

- `find_is_catastrophic` does not classify an exec target's arguments when the
  start path is not catastrophic AND the target is unrecognized. Pre-existing.
- A policy layer committed in the same turn as its own change is not announced,
  because the Stop hook runs after the commit.
- A deleted tracked binary, or one whose staged content is binary while the
  worktree copy is text, is classified from the worktree copy.
- An unclosed cgo `/* ... */` preamble line, and an unclosed `<!--[if IE]>`, are
  skipped as comments. Only the opening line; continuation lines scan.
- Three marker-table rows are kept without being airtight: `.cfg` and `.conf`
  name a purpose rather than a language, `.pl` is also Prolog, and a YAML literal
  block scalar makes a `#` line inert document data.
- The executable-bit test would disable the prose exclusions wholesale on a
  filesystem that marks everything executable. That direction is false positives,
  not misses.
- The chunk-length expansion in `nul_scan` counts characters, not bytes, under a
  multibyte locale, so the 1 MiB bound is per-character.
- A risky line removed in the index and restored in the worktree gates. The
  conservative direction, and the price of using the fingerprint's own
  decomposition.

`allow-read-only.sh`, which ships unregistered:

- It approves reading a secret without a prompt. Reading is not writing, and the
  hook's proof is about writes.
- Approval names a command, not an executable. A shadowed binary or an exported
  shell function passes, exactly as it passes a human clicking approve.
- It approves commands that may not terminate, such as `wc -c /dev/urandom`. A
  hang is a nuisance, not a breach.
- Three consecutive review rounds each found one write or exec hiding behind a
  flag name (`file -z`, `file -p`, `stat --cached`). Every remaining flag has now
  been read in its own manual and run under `strace`, but the base rate of that
  search has not reached zero.

Elsewhere:

- `delegate-resolve` still has one `grep -m1` in a pipeline, at the sole-candidate
  read. Its status is never consumed and the value is captured before the writer
  can be signalled, so it cannot produce a wrong answer today. It is the same
  SIGPIPE-under-pipefail shape as the bug that was swept, and a trap only if
  someone later tests its status.
- `agents/model-delegate.md` is a tombstone. Claude Code registers every agent
  file, so it still costs selector context; deleting it is a maintainer decision.

### Added

- An enforcement lifecycle. Every rule that can block a session, and every rule
  that only advises one, is declared in a plugin's `enforcement.toml` with a
  state of `off`, `advisory`, or `enforced`, the hook that owns it, the skill it
  comes from, and the date it was promoted. `state = "off"` in a project
  `.megapowers/enforcement.toml` is the supported per-repository opt-out, so
  silencing a gate no longer means patching a hook. `scripts/check-enforcement.sh`
  runs in `validate.sh` and asserts the declarations still describe the code: the
  named hook exists, and it actually reads the rules file. That last check is the
  point. The risky-logic keyword list used to live inline in the hook with nothing
  recording that the rule was enforced or when, so nothing could tell that the
  gate and the prose describing it had diverged.
- `scripts/session-metrics --rules` reports the four numbers that would have
  surfaced all of this on their own: advisory honor rate, gate fires with their
  per-project distribution, verify rounds reached before an approve, and
  delegate-run wall clock including the calls the harness backgrounded.

### Fixed

- The risky-logic gate scans what can execute. It matched a keyword list against
  the entire raw diff, every line and every file, so a 2026-08-05 audit of 1,324
  transcripts found it firing 141 times with 47 of those inside this repository,
  which ships markdown and shell and no auth or billing code. One session
  absorbed 37 consecutive blocks. It now reads added lines only, skips comment
  lines, skips prose formats, and never scans the files that define the gate, all
  declared in `enforcement.toml`. On this branch's own diff the old matcher trips
  on ten files and the new one on none, with the exclusion set unchanged at 20 of
  the 45 pending paths in the reviewed package.

  Every narrowing has to prove a property rather than trust a name, which is the
  rule five rounds of adversarial review kept teaching. `*.txt` is not a prose
  extension, because `requirements.txt` is a dependency manifest. Neither is
  `docs/`, because a path is not a format and `docs/auth.ts` executes. Neither is
  `*.mdx`, which embeds JSX. An excluded path is scanned anyway when it carries
  the executable bit or when line 1 is a `#!` shebang. A path that cannot be read
  at all is not scanned, because there is nothing to scan; it is announced by
  name, and it blocks only when the path itself names a risky category.
  Comment markers are chosen per extension and an unknown extension scans every
  line, because `#`, `--` and `;` execute in some languages; shebangs, `//go:`
  directives, and `// +build` are never treated as comments. A self-exclusion
  anchors on the running installation or a committed plugin manifest, never on a
  directory that merely carries the right name.
- The gate reads policy from committed content, not from the tree it is
  reviewing. A pending `.megapowers/enforcement.toml` could previously set
  `state = "off"` in the same change as new auth logic and silence the gate
  judging that change. Project layers now resolve through `git show HEAD:`, a
  pending edit is announced rather than honored in both directions, and a
  repository with no commits gets no project layer at all.
- The gate could fail open. The second `git diff` that drops excluded paths did
  not check its exit status, so exceeding `ARG_MAX` yielded an empty scan and a
  silent pass on a real finding. It now falls back to scanning the unreduced
  diff, which is the conservative direction.
- `delegate-run` stops after three consecutive dispatches of a role on a branch
  without an approve, and hands the decision to the human with the transcripts
  from the previous rounds. The ledger already counted rounds; nothing capped
  them, and the audit found a real branch at round 11 and a repository branch at
  round 22.
- `delegate-run` enforces its own 540 second budget rather than trusting a
  caller's `timeout`. Forty-four call sites declared 900 to 1800 seconds under a
  harness that caps a foreground command at 600, so verdicts were arriving after
  the lead had moved on. It also bounds the review package: one measured 674,630
  bytes across 11,183 lines, and a reviewer handed that reviews the beginning of
  it. Over budget, the package names every elided path and its line count instead
  of truncating silently, serves those bytes from the immutable snapshot rather
  than the live worktree, and the receipt still binds the complete tree. A round
  is reserved only after provider and credential preflight, so a setup failure no
  longer spends one: a sandboxed `codex exec` that cannot read `~/.codex/auth.json`
  burned a round of this branch's own budget before that fix.
- Reasoning effort is a routed decision. Roles that adjudicate or refute run
  `high`; scoped implementation, visual, and browser work run `medium`, following
  OpenAI's GPT-5.6 guidance that medium is the balanced starting point and that
  `high` or `xhigh` want eval evidence of a meaningful gain. `templates/codex-config.toml`
  drops from `xhigh` to `high` for the same reason: the audit measured 6,763
  turns at `xhigh` against 2,298 at `high`, so the escalation rung had become the
  floor and there was nothing left to escalate to.
- `run-init` scaffolds `runbook.md` from `references/runbook-template.md` instead
  of a heredoc, and the template is split into machine-checkable MUST lines and
  advisory SHOULD lines. It was nine numbered steps carrying roughly sixty
  constraints in compound sentences, which is where GPT-5.6's "conflicting rules
  can create more instability than missing detail" bites.
- A membership test spelled `printf ... | grep -q ...` under `set -o pipefail`
  returns FAILURE on a match that is not on the last line. `grep -q` exits at the
  first hit, the unwritten remainder kills the writer with SIGPIPE, and pipefail
  promotes that to the pipeline's status, so a value that is present reads as
  absent whenever the reader wins the race. It was measured at roughly 5% under
  load and 0% serially. `delegate-resolve` used that spelling at sixteen
  membership sites including route resolution, so under load it could skip a
  provider that has a required capability or refuse a valid tier: a flaky routing
  decision, not a flaky test. Swept repo-wide, including two live sites in
  `scripts/validate.sh` itself, the autonomous-run status scripts, and
  `mem-recall`.
- `deny-destructive.sh` recognizes a home directory named literally
  (`/home/<user>`, `/Users/<user>`, an expanded `$HOME`) and parent-relative
  escapes as catastrophic targets. It also classifies the argv of `find -exec`,
  `-execdir`, `-ok`, and `-okdir` through the same engine it applies to a bare
  command, so `find . -exec sh -c 'rm -rf /' \;` no longer launders a
  catastrophic command through a harmless start path. Across 60 realistic
  `find -exec` idioms the change moved no verdict, and across the fixture corpus
  it moved exactly one, from ask to deny.

### Added

- `mega-guardrails/hooks/allow-read-only.sh`, unregistered by default. A
  PreToolUse(Bash) hook that skips the approval prompt on ordinary inspection
  commands. It checks one positive property (every byte of the command is drawn
  from an inert set) rather than chasing a list of bypasses, so redirection,
  substitution, control operators, globs, and unresolvable quoting are all
  rejected by the same test. It never denies and never asks.

  It approves `ls`, `wc`, `stat`, `file`, `head`, and `tail`. It does **not**
  approve git, and that is the interesting part. An independent cross-vendor
  review found the hook claiming "proven read-only" over commands that write, and
  reproducing it took no attacker at all: on git 2.53.0 in a clean repository,
  `git status` and `git diff` rewrite `.git/index` whenever the cached stat data
  is stale, which is the state of any worktree somebody just edited. Worse,
  `git status`, `git diff`, and `git ls-files` execute the program named by the
  repository's own `core.fsmonitor`, and `git log -p`, `git show`, and `git diff`
  execute `diff.external`, a textconv filter, or `core.pager`. All of it is
  configured in `.git/config` and `.gitattributes`, files the hook never opens,
  so rejecting the command-line `-c` flag prevented none of it. Whether a git
  command writes is a property of the repository, not of the command string, and
  the string is all this hook sees. `git rev-parse` survived every probe and is
  gone with the rest, because keeping it means a per-subcommand exception list
  that fails open against the next git release.

  That costs real value. `git -C <path> status` is in the measured interruptions
  this hook was built to remove, and it prompts again. The decision string being
  true is worth more.

  The claim itself also changed. Approval now says what it proves: the command
  string carries no write construct and names an allowlisted command with
  allowlisted flags. It deliberately does not claim to know which executable each
  name resolves to, because it cannot. A shadowed `ls` earlier on `PATH`, or an
  exported shell function of the same name, passes every check. That gap is not
  closed in code and does not need to be: the prompt this hook replaces has the
  identical hole, since nobody approving `ls -la` is told which binary answers.
  An overstated claim is more dangerous than a narrow gap, because it invites
  reliance the mechanism cannot support.

  The same review caught that claim a second time, in the same sentence. The
  reason ended its list of absent constructs at "or control operator", flatly,
  while the most valuable thing the hook approves is `cd PATH && COMMAND`, which
  carries an `&&` and accounted for 4,348 of the 22,201 observed calls. The
  exception is now named in the reason rather than the shape dropped, and the
  suite asserts it mechanically instead of by eye. The test suite's own header had
  drifted the same way, defining an approval as proof the command was read-only,
  and it now states the string-scoped contract the hook actually asserts. Five of
  the seven findings in that round were documentation claiming more than the code
  did.

  Three flags went the same way as git, one per review round, each found by
  auditing the flag rather than by trusting the command name. `file -z` and
  `file --uncompress` exec a decompressor named by the operand's own first bytes,
  resolved through `PATH`: on file 5.46, `file -z a.lz` runs `lzip` and
  `file -z a.zst` runs `zstd`, so a file the agent merely inspected got to pick a
  program name. `file -p` is plainer still and sat on the list a round longer. It
  restores the access time through `utimensat`, a write to the inode of the file
  being inspected, and it is not even a faithful restore: the call carries whole
  seconds, so an ext4 file's mtime went from `.419860996` to `.000000000` and its
  ctime moved. Plain `file x` issues no `utime` call at all, so the syscall is the
  flag and not the read. `stat --cached` is gone because `--cached=never` asks for
  `AT_STATX_FORCE_SYNC`, which `statx(2)` says may require a network filesystem to
  perform a data writeback; a long flag matches by name with its value stripped,
  so the whole name goes rather than one mode. Nothing left on the allowlist
  writes or execs, and every surviving flag was read in its own manual and then
  run under `strace`.

  A read-only Bash allowlist was drafted for `templates/settings.example.json`
  and then dropped before shipping, because an independent cross-vendor review
  showed its premise was false. A `Bash(cmd:*)` rule matches on the command
  prefix, so `Bash(ls:*)` also matches `ls -la > ~/.bashrc`,
  `ls "$(curl -fsS URL)"`, and `ls "$(sh -c 'touch owned')"`. No prefix rule can
  say "this command, without redirection, substitution, or control operators",
  so no shell command is read-only at the prefix level. The 4,348 `cd X && ...`
  approval prompts the audit measured are real friction, but a preapproval that
  also grants writes, network access, and arbitrary subprocess execution is not
  the fix. Reducing them needs a matcher that parses argv.

### Changed

- Codex ships as a lead only. `templates/CODEX.md` was a delegate baseline whose
  first instruction was "you are a delegate"; it is deleted, and the former
  `templates/CODEX-LEAD.md` takes its name. One template per harness, both lead
  charters, because each harness leads in its own runtime and they dispatch each
  other on demand. Being dispatched is now a property of the task brief rather
  than of an instruction file: both templates carry the same paragraph saying a
  brief makes you its delegate for that brief's duration, which is also where
  the compressed reporting contract that the delegate baseline used to hold now
  lives. Update any `AGENTS.md` symlink pointing at `CODEX-LEAD.md`.
- `templates/codex-config.toml` disables `tool_suggest`, `apps`, and
  `image_generation`, worth 1,782 tokens per turn on codex-cli 0.146.0 measured
  against a 25,209-token baseline. `memories` and `web_search` cost more, 3,880
  and 2,280, but a lead uses both and they stay on.

### Added

- `templates/settings.example.json` turns off harness surfaces the plugins
  already cover: `disableWorkflows`, `includeGitInstructions`, `disableArtifact`,
  `disableClaudeAiConnectors`, per-skill `skillOverrides` for the bundled skills,
  and the `Explore`/`Plan` agent types. Worth 7,818 tokens of system prompt on
  every turn, measured, with the feature each one costs written down in
  [docs/setup.md](docs/setup.md#optional-templates).
- `upgrading-megapowers` covers the instruction files and settings baselines, not
  just plugin versions. No plugin ships `templates/`, so the skill had nothing to
  compare against and reported the only observable it had. It now fetches the
  baselines at the installed and target versions, reports what the baseline
  itself changed in between, and classifies each candidate file as absent,
  unrelated, or adopted first, because a file meant to be edited always differs
  from the shipped copy and that difference is not a finding. Adoption is
  inferred from the installed plugin version and the skill says so; a failed
  fetch reports that the check did not run rather than an empty drift set.
- The settings comparison flags a `sandbox.credentials` block still using the
  pre-0.8.2 bare-string form, which parses as invalid and takes every sibling key
  in that settings source with it.

### Fixed

- `templates/settings.example.json` listed `sandbox.credentials` as bare path and
  variable-name strings. Claude Code expects `{"path": ..., "mode": "deny"}` and
  `{"name": ..., "mode": "deny"}` objects. Copied wholesale, the file protected
  no credentials, and on 2.1.222 it also voided the keys beside them: a two-key
  file whose only fault was a bare-string credential entry left `disableWorkflows`
  unapplied at 31,446 tokens, against 25,432 once the entry became an object.
  The published reference describes invalid entries as stripped individually, so
  the blast radius may be version specific; the malformed shape is a bug either
  way.

## 0.8.1 - 2026-07-31

### Added

- Routes carry `CALLER`, either `declared` or `assumed-lead`. `DISPATCH=native`
  asserts the resolved provider is the calling session, and when nobody declared
  one that rested silently on the catalog `[lead]`. A non-lead session reading
  `native` would run its own subagent against another vendor's model believing it
  was home. The assumption is now stated on the route and warned about on stderr.
- `AUTHOR_VENDOR` is emitted once per author and is authoritative; `delegate-run`
  prefers it. Recovering an identity no longer means splitting a delimiter, so a
  vendor name cannot be cut in half and a blank identity cannot hide inside a
  joined string. The joined `AUTHOR_VENDORS` remains for older consumers.
- Independence routes carry `ALTERNATES`, the number of vendors that could still
  serve the role with the authors excluded. `ALTERNATES=1` says the next outage
  takes independent review with it, which is the shipped state until a third
  vendor is reachable.

### Fixed

- Reachability (section present, enabled, native or CLI installed) had been
  written twice, once for resolution and once for the `--vendors` probe, and the
  two drifted three times. It is now one function returning distinct statuses so
  each caller keeps its own policy: resolution still treats a missing section as
  fatal and preserves the exit-4 single-route contract for a disabled provider,
  while the probes skip. A test matrix pins `--vendors` to list the vendor the
  same role resolves to, including where the provider CLI is absent.
- `ALTERNATES` counted any reachable provider, including ones the role would
  reject for capability, tier, effort, or floor, which would report a spare route
  that does not exist. It now applies the same eligibility the probe does.
- `delegate-run` chose the author encoding by how many values it decoded rather
  than whether the repeated form was present, so a route could declare the
  authoritative encoding, deliver only empty records, and silently fall through
  to the joined one. Presence now decides and an empty record is refused.

### Known

- A chain candidate with no model at the role's tier is fatal in resolution but
  skipped by `--vendors`, so the probe can report a vendor resolution refuses to
  use. Both behaviours predate this release; the tests pin them so the
  inconsistency is visible. Reconciling it moves an exit code, so it is not done
  here.

## 0.8.0 - 2026-07-31

### Added

- Delegate routes carry `DISPATCH`. `native` means the route landed on the
  provider the calling session already is, so it belongs on that harness's own
  subagent, team, or workflow surface; `CHANNEL` and `BINARY` describe the
  cross-runtime path and apply to `cli` only. Calling your own CLI spawns a cold
  session, discards the context that made delegating worthwhile, and bills twice.
- `--author-model <id>` and `--author-provider <name>` name an artifact's author
  by model id or backend and let the resolver derive the vendor. A harness always
  knows the model it runs; it does not necessarily know the vendor name this
  catalog files that model under, and a BYO-model runtime (OpenCode, Cursor CLI,
  pi) has no fixed vendor to hardcode at all.
- `--caller-model <id>` and `--caller-provider <name>` name the session that is
  RUNNING. They feed native-dispatch detection only and never enter the exclusion
  set, `AUTHOR_VENDORS`, or a receipt, so declaring a caller cannot make a review
  look independent. Pass one when reviewing another agent's work: the author is
  excluded, the route comes back to you, and that is a native dispatch.
- A `[roles]` value of `self` routes to the caller's own provider. `small_impl`
  ships that way: a scoped implementation carries no independence requirement, so
  spending a third-party call on it buys nothing. `visual` and `browser_test`
  still leave the vendor for capability and cost, which the shipped guidance now
  states rather than implying a blanket in-vendor default.

### Fixed

- A native route required the provider's CLI to be installed, so a harness with
  no `claude` binary anywhere on PATH got no route at all for work it could
  obviously run on its own subagent. Reachability is now decided per candidate
  before the `command -v` check, on both the resolution and `--vendors` paths.
- `--author-vendor` accepts a provider name but excluded only that provider, so a
  sibling backend of the same vendor stayed eligible to review its own author's
  work. Author tokens are normalized to a vendor before exclusion, and a token
  naming neither a declared vendor nor a provider is refused rather than
  excluding nobody.
- Author exclusion applied to every role, so telling a harness to identify itself
  cost it its own vendor on roles that never asked for a second opinion. It now
  follows `[independence]`. A legacy table (no `[independence]` section and no use
  of the sentinel) keeps excluding unconditionally.
- A model id declared by providers of two different vendors could exclude the
  wrong vendor. Such an id is refused at resolution and reported by `--check`.
  One vendor behind several providers leaves independence intact but no longer
  guesses a backend for a `self` role.
- Identity is unique-or-refused per flag occurrence: every occurrence constrains
  the answer and they intersect, so contradictory identities are refused instead
  of resolved by precedence, and two ambiguous ids overlapping in one provider
  resolve to it.
- `delegate-run` trusted caller-supplied author vendors when a route omitted
  `AUTHOR_VENDORS`. It now discards its own input first, rejects a malformed or
  blank field, and verifies the reviewer CLI exists before dispatch: an
  independent review runs as an isolated one-shot session by design, so it always
  needs a reachable CLI and can never act on a native route.
- A role that is both `self`-routed and `[independence]`-bound is refused at
  resolution, not only by `--check`. A catalog that already declares
  `[providers.self]` keeps that provider's meaning on every path, reported by
  `--check`, so no existing route is silently redirected.

## 0.7.3 - 2026-07-30

### Fixed

- The risky-logic gate could report a clean tree it never read. `git diff`
  prints nothing and exits 128 when a tracked path is not a regular file, and
  the hook read the empty output as an empty diff. The sandbox creates that
  condition routinely by bind mounting `/dev/null` over deny-listed paths, so a
  tracked `.env.example` was enough. The status is now captured, non-regular
  paths are excluded with `literal` pathspec magic and the diff retried, and an
  uncomputable diff blocks with a diagnostic instead of passing.
- A tracked non-regular path whose name contained a glob metacharacter hid other
  files from the same gate. Without `literal`, a fifo named `x*.go` dropped every
  path starting with `x` from the diff the gate reads, and the name is chosen by
  whoever adds the file.
- The review receipt identified a change set rather than a change set on a base
  in a repository, so a receipt was portable between checkouts that shared a
  pending delta. Receipts are now `megapowers.review-receipt.v2` and bind
  `subject.base`; v1 receipts without it are refused rather than honored.
- The receipt fingerprint could be computed from a truncated stream, which made
  unrelated repositories share one id. It now builds to a file, propagates every
  status, and emits nothing rather than a partial hash when it cannot compute.
- `delegate-run` fingerprinted the subject before capturing the review package,
  so the tree could change between the two. Both now derive from one snapshot.
- On a tracked non-regular path the gate demanded a receipt that `delegate-run`
  could not produce, exiting with a raw git 128 that appeared in no exit map.
  Package capture uses the same exclusion logic, and an uncapturable package is
  exit 9 naming the path.
- `approve` was accepted alongside critical findings. The verdict schema now
  couples them, so the invariant is enforced rather than requested.
- Submodule pointer changes and `assume-unchanged` edits bypassed the risky
  scan entirely. Both are now gated, with a supplemental `subject.submodules`
  binding that leaves `subject.id` byte-identical.
- Replacement objects defeated base binding. Every git call in the gate, the
  launcher and the fingerprint now runs with `GIT_NO_REPLACE_OBJECTS=1`.
- Deleting a stale plugin cache version silently killed hooks in every session
  still pinned to it. `upgrading-megapowers` documents the ordering: upgrade,
  restart sessions, then delete.

### Added

- `skill-router.sh`, a `UserPromptSubmit` hook naming the one applicable skill
  when its trigger phrase is typed. Across eleven audited sessions there were
  four skill invocations, and the SessionStart reminder was in context for every
  missed one. Silence is the default: measured 0 matches on 92 ordinary prompts.
- `scripts/session-metrics`, which reports per-session and aggregate model
  latency, tool wall clock, batching and backgrounding from the transcript
  store, so a claim about session speed can be checked and can regress.
- A supplemental `subject.submodules` receipt field, and `--transcript-dir` on
  `delegate-run` so a cross-vendor review leaves a durable record.
- Moonshot as a third catalog vendor, shipped disabled, so cross-vendor
  independence is not a single point of failure once a channel is configured.

### Changed

- The prose register is enforced mechanically on written markdown and stated
  once instead of in four skills. `MEGAPOWERS_PROSE_REGISTER=off` disables it.
- The `strong` tier resolves to Sonnet 5 rather than collapsing onto the
  frontier model, so cheap work can route cheap.
- The five longest skills moved 234 lines of reference material into sibling
  files against 2 lines deleted.

### Known limitations

- The gate reads what `git diff` reports. Content it does not surface is outside
  the binding: ignored paths, bytes behind a clean filter, and source rendered
  binary by a committed `.gitattributes` rule. The last of these silences the
  risky-token scan and predates this release.
- The receipt is an unsigned local file, so anything able to write it can mint
  an approval. This is an accident backstop, not a security boundary.
- `assume-unchanged` is unusable with the gate by design, since the bit exists
  to make a local edit indistinguishable from no edit.

## 0.7.2 - 2026-07-27

### Fixed

- The destructive-command guard stopped asking about ordinary long commands.
  Its parsers now consume runs of characters instead of one byte at a time and
  run under `LC_ALL=C`, which cut a real 4.5k-char heredoc from ~750ms to
  ~40ms, so the length cap that degrades to a confirmation prompt moved from
  4000 to 16000 chars. In 1660 observed Bash calls the longest command was
  5445 chars, so every one of them used to be a coin flip against the cap and
  none of them reach it now.
- `curl … | python3 -c '<script>'` no longer asks. Piping a download into an
  interpreter that runs its own program (`-c`, `-e`, `-m`, or a script path)
  passes the download as DATA, which is how you read a JSON API from the
  shell. A bare `| bash`, `| bash -s -- --yes`, or `| python3 -` reads stdin as
  its program and still asks.

## 0.7.1 - 2026-07-27

### Changed

- The baseline commit rule now bounds the message. `templates/CLAUDE.md`,
  `templates/CODEX.md`, and `templates/CODEX-LEAD.md` say the subject line
  carries the change and a body is one sentence, added only when the why is not
  readable from the diff. `CONTRIBUTING.md` said "explain the why in the body"
  with no length, which read as an instruction to restate the diff in
  paragraphs.

## 0.7.0 - 2026-07-26

The catalog ships three models instead of six, and claude leads.

### Changed

- The catalog ships three models, one per job: `claude-opus-5` leads at `high`
  effort and does in-session teammate work at `medium`, `gpt-5.6-sol` is the
  critic (plan review, code review, verify, judge, research, computer use), and
  `gpt-5.6-terra` is the cheap executor for scoped implementation. `claude-fable-5`
  and `gpt-5.6-luna` are no longer routed; an override layer can bring either back.
- **Breaking:** `[lead]` is now `claude` frontier, not `codex`. Running Codex as
  the lead is an override layer, which is what `templates/CODEX-LEAD.md` already
  documents. Nothing else about the routing contract changed.
- Every delegated role routes to Codex by default, because the lead is now the
  usual artifact author and an in-vendor review is not a second opinion. The
  `[fallbacks]` chains still bounce back to Claude when Codex authored the
  artifact. `code_review` moved from the `strong` tier to `frontier`: reviews and
  judgement get Sol, and `small_impl` keeps `strong` for Terra.
- The `fast` tier left the scale along with the `gpt-5.6-luna` mapping that
  filled it. No role used it, and the `strong:low` floor already made it
  unroutable.
- The native Codex `reviewer` role moved to `gpt-5.6-sol`, following
  `code_review` to the frontier tier; `builder` stays on `gpt-5.6-terra`.
  `scripts/validate.sh` now pins each role to its own tier instead of checking
  both against `strong`, which had masked the mismatch.
- The Claude provider now declares `xhigh` alongside low/medium/high, matching
  what the Anthropic models and the `--effort` flag actually accept. `ultra`
  stays Codex-only: the Claude CLI has no equivalent rung.

## 0.6.1 - 2026-07-26

Audit remediation: drop tests that pinned prose, close two coverage gaps, and
stop three comments from claiming more than the code does.

### Added

- `scripts/validate.sh` checks that every shipped skill has a resolving
  `.agents/skills/<name>` symlink, with a one-line exemption list. Codex,
  OpenCode, and Antigravity discover skills in a checkout through those links,
  and nothing verified them, so `upgrading-megapowers` had been missing its
  link and was invisible to those harnesses. Link added.
- `delegate-resolve [<role>] --vendors` prints reachable vendors. With a role it
  walks that role's candidate chain under the same capability, tier, effort, and
  floor filters resolution uses; bare, it reports every installed provider.
  Independence needs two, and the count is what tells a caller whether a
  cross-vendor review is achievable. The role form exists because the machine
  having two vendors does not mean the verify chain can reach both.
- Execution coverage for `sdd-workspace` and `review-package`, the two shipped
  SDD helpers that had none. The only prior reference asserted the directory
  listing, so neither script was ever run by a test. 25 cases, mutation-tested.
- An explicit-only skill reachability check in `scripts/validate.sh`: a skill
  whose sidecar sets `allow_implicit_invocation: false` is never surfaced by
  implicit discovery, so it must be named as `<plugin>:<skill>` by some other
  shipped skill or nothing can route to it. Added after an independent review
  observed that dropping the `wayfinding` prose markers had also dropped the
  only guard on its orchestrating route, which is a functional invariant rather
  than wording. Mutation-tested by removing that route.
- `lib-toml.sh`, the restricted-TOML grammar shared by `render-model-catalog`
  and `delegate-resolve`, which each carried their own copy of the same awk.
  Ships as a byte-twin (plugins cannot locate each other at runtime) with a
  drift check in `validate.sh`, matching how `dispatch.sh` and `models.toml`
  are already handled.

### Changed

- The Stop-hook delegate nudge no longer prescribes a command that cannot
  succeed. With fewer than two reachable vendors, `delegate-run --role verify`
  exits 3 whatever `--author-vendor` is passed, so the gate still fires but
  asks for human sign-off and says plainly that the automated cross-vendor
  check did not run.
- `[efforts] scale` gains `ultra` and drops `max`. `templates/codex-complex.config.toml`
  ships `model_reasoning_effort = "ultra"`, which the catalog could not express,
  while `max` was documented as a value no shipped provider allows and still
  rendered into every session's catalog block.
- `plan_digest()` moved into `run-lib.sh`, which all three callers already
  source. It had been copy-pasted verbatim into `run-init`,
  `run-derive-status`, and `run-verify-status` with a comment asking
  maintainers to keep the copies in lockstep.
- The autonomous-run milestone digest is described as drift detection rather
  than tamper-proofing, in `SKILL.md` and in the scripts. It lives in the same
  agent-writable directory it describes and `--replan` re-freezes it on
  request, so it catches a silent mid-run redefinition of success, not a
  determined actor. `SECURITY.md` already put that threat model out of scope.
- The delegate nudge no longer scans its own source or `hooks/tests/`. Those
  files must contain the risky keyword list verbatim (the pattern, the block
  message naming the categories, and fixtures like `billing()` proving the gate
  fires), so editing the guard always tripped the guard and the resulting review
  request cited its own warning text as the risky change. Sibling hooks such as
  `deny-destructive.sh` stay scanned.
- `deny-destructive.sh` comments now match the code: quote-aware segmentation
  is there for precision (so `echo "rm -rf /"` is not denied), and `bash -c`
  recursion is there because nested accidents are real, not to win a race
  against deliberate obfuscation.

### Fixed

- `review-diff-id` and `delegate-run` aborted the entire review on any untracked
  entry that is not a readable regular file. A checkout with character devices
  in the repository root, or a dangling symlink anywhere, killed the review
  before it reached a model. Both now bind such an entry by identity (symlink
  target, or type and `lstat` metadata) rather than reading it, so the
  fingerprint still moves when one appears, disappears, or is retargeted in
  place. Three passes were needed here: the first attempt bound only a constant;
  the second still tested `-f` before `-L`, so a live symlink was bound to its
  target's contents and retargeting it between equal-content files moved nothing;
  the third found that an unreadable regular file (mode 000) also lands in this
  branch, where type plus size collided on a same-size rewrite. Non-regular
  entries now carry nanosecond mtime and inode, because an in-place rewrite
  reuses the inode and usually lands within the same second.
- The delegate nudge probed vendors globally rather than through the verify
  chain, so a vendor that role cannot route to still counted as an independent
  reviewer and the hook prescribed a launcher that would exit 3. It now probes
  `verify --vendors`.
- The delegate nudge mishandled a resolver reporting zero reachable vendors:
  `grep -c .` prints `0` but exits 1, so the count was discarded and the strict
  default left in place, prescribing a launcher that cannot resolve. Counted
  with `awk` now, and covered by a zero-vendor test.

### Removed

- Six eval scenarios that asserted only that particular phrases still appeared
  in shipped `SKILL.md` files: `review-axes`, `skill-authoring-quality`,
  `planning-graph-guidance`, `debugging-loop-guidance`,
  `swarm-primitive-invariants`, `polyglot-baseline-lessons`. They measured no
  behavior and taxed every de-prescription wave without catching a defect.
  `wayfinding-contract` kept its nine validator mutations and lost its ten
  prose markers. The deterministic suite goes from 21 scenarios to 15 while
  `validate.sh` goes from 393 to 385 checks; every remaining oracle runs a
  shipped script or hook.

## 0.6.0 - 2026-07-26

De-prescription release. Shipped guidance is rewritten against current vendor
context-engineering guidance for frontier models: fewer constraints, less
repetition across surfaces, and no history in files an agent loads.

### Added

- A response-style contract at the top of every shipped instruction template.
  Compression over grammar, answer in the first line, a four-line default prose
  ceiling with code and command output free, no recap or closing summary. The
  Codex delegate template carries the variant for output a lead reads.

### Changed

- `using-megapowers` no longer requires invoking a skill before any response,
  including clarifying questions and code reads. A skill that covers the task
  owns the procedure; mechanical edits, lookups, and conversation need none.
  The SessionStart preface matches.
- The rationalization sections in `test-driven-development` and
  `systematic-debugging` are recognition lists rather than arguments, at roughly
  half the length.
- `verification-before-completion` scopes its acceptance evidence map to
  multi-criterion or externally-verified work. The three-state ladder and the
  evidence-before-claims gate stay unconditional.
- Instruction templates drop the routing table that duplicated the skill listing
  and the delegation config archaeology.
- `validate.sh` asserts three semantic markers for scratch-storage guidance
  rather than the paragraph verbatim, and the recursive-guidance contract pins
  two safety rules per template instead of the full coordinator contract, which
  lives in `subagent-driven-development`.
- CONTRIBUTING sets a lower evidence bar for removing guidance than for adding
  it, and states that migration notes, superseded behavior, war stories, and
  measurement provenance belong in CHANGELOG.md or evals/RESULTS.md.

### Fixed

- The delegate nudge scanned documentation for its risk keywords, so prose that
  named those categories tripped the gate on doc-only edits. It now reads code
  paths only. Covered by `delegate-nudge-prose.test.sh`.

### Removed

- The dead `detect` provider key from both `models.toml` twins. No code read it.
- Pre-0.3 override archaeology from `models.toml`, `delegates.toml`,
  `delegate-resolve`, and the `multi-agent-delegation` body; a version-stamped
  harness note; three war stories in `subagent-driven-development`; two
  measurement anecdotes in `writing-skills`.

## 0.5.0 - 2026-07-23

Reliability and efficiency release based on a cross-harness audit of live
Codex and Claude sessions.

### Added

- A fail-closed independent-review launcher with author-vendor exclusion,
  role-specific model tier and effort, strict structured verdicts, complete
  worktree identities, atomic provenance receipts, and screenshot hashes for
  visual verification.
- An explicit browser-driver layer. Playwright captures evidence while a real
  independently routed vision model owns the visual verdict.
- Recursive ownership preflight rejects missing, globbed, duplicated, and
  parent-child-overlapping paths before shared-checkout writers launch.
- Autonomous runs carry literal acceptance evidence, verification states,
  external-system cutpoints, explicit session ownership, and report-time
  warnings for pending or blocked evidence.
- Deterministic tests cover launcher receipts, Stop-hook ownership and
  freshness, recursive ownership, and fail-closed eval phases.

### Changed

- Process skills use one workflow announcement and one progress surface,
  select a scoped TDD fast path when requirements are clear, scale review and
  planning to risk, cap review loops, batch mechanical work, avoid unchanged
  polling, and reserve context for integration and verification.
- Stop hooks distinguish controller, reviewer, plan, read-only, and exact-output
  contexts. Review completion requires a current receipt rather than a
  transcript marker; autonomous continuation requires an explicit run claim.
  Receipts are subject-bound accident backstops, not tamper-proof attestations.
- Eval and real-agent study runners propagate setup and actor failures.
  Scorecards report harness errors separately from genuine indeterminate
  outcomes and exclude neither as a fabricated pass.
- Freshness review defaults to 30 days and runs weekly. Native Claude manifest
  validation is a required CI gate.
- The install smoke installs every plugin, rejects all-SKIP results, and offers
  a strict post-publish mode that fetches and verifies an exact remote tag,
  commit, and manifest version before fresh-home Claude and Codex tasks.

### Removed

- Browser automation as a fake model provider.
- Transcript-marker suppression as proof of independent review.
- Workflow-implied commit authorization, redundant skill announcements,
  duplicate checklists, unbounded review retries, and routine unchanged-state
  polling.

## 0.4.1 - 2026-07-20

### Changed

- Agent baselines now keep large worktrees, build caches, browser profiles,
  and similar artifacts in a writable, capacity-checked `$TMPDIR`. `/tmp`
  remains available for small OS temporary files and IPC state.
- Code review worktrees follow the same scratch policy instead of hard-coding
  `/tmp`.

### Added

- Release validation checks that the Codex and Claude baselines and the shared
  review rubric retain the scratch-storage policy.

## 0.4.0 - 2026-07-19

Maintainability release: the 2026-07-18 over-engineering audit executed in
full. Roughly a fifth of the repo's line count is gone with no capability
loss beyond the two removals called out below.

### Removed

- **Breaking:** the brainstorming visual companion (hand-rolled WebSocket
  server, launcher scripts, frame template, operator manual). Brainstorming
  now writes static HTML/SVG mockups and opens them with whatever the
  harness provides.
- **Breaking:** the `dispatching-parallel-agents` skill. Its content lives
  in `mega-orchestration:orchestrating` (Parallel fan-out section);
  subagent-driven-development already covered plan execution.
- The deny-destructive ask tier for remote tools (aws, docker, terraform,
  tofu, kubectl). Real-world effects are the effect-broker skill's job;
  the hook keeps catastrophic local denies, destructive-git asks, and the
  curl-pipe-shell ask.
- evals: the never-run `head-to-head` study and the unreproducible
  `skill-effect` study tree (RESULTS.md keeps the published numbers), plus
  the unimplemented results-hash audit convention from the evals README.
- validate.sh prose-pin assertions (exact-sentence greps in docs and
  skills), the hook-handler count shape, and the Codex implicit-list
  budget model of a third-party renderer.

### Changed

- Hooks: one generic `dispatch.sh` per plugin (byte-twins, cmp-gated)
  replaces the four per-hook shims; every hook now routes through
  `run-hook.cmd`, making Windows handling uniform and gating delegate-nudge
  for Codex like its run-loop sibling. delegate-nudge is rewritten as a
  stateless-config nag with one static marker regex (no TOML-driven regex
  synthesis, sha256sum-only sentinel).
- evals: the four remaining study runners share `studies/lib.sh` (agent
  invocation, codex JSONL normalization, fan-out); the offline oracle
  selftests now run in validate.sh.
- Skills: the two reviewer prompt templates share `review-rubric.md`;
  golang-patterns is a compact design-choice checklist with the sqlite
  testDB footgun moved to greenfield-go-stack; systematic-debugging's three
  technique references merge into one `debugging-techniques.md`;
  testing-anti-patterns and testing-skills-with-subagents lose their
  duplicated recap sections.

### Added

- `scripts/release.sh <version>` stamps all plugin manifests and the doc
  install pins from the changelog, replacing the 17-file hand edit per
  release.
- `plugins/megapowers/hooks/lib-json.sh` (shared `escape_for_json`).
- AGENTS.md documents that mega-guardrails intentionally ships no root
  `plugin.json`.

## 0.3.9 - 2026-07-17

### Added

- Recursive coordinator guidance for native Codex and Claude Code subagents
  allows independent writers to use disjoint owned paths in one shared
  checkout, without a Megapowers runtime or worktree manager.
- Plans can now state parallel safety, exact path ownership, and whether a
  coordinator may split a task into independently testable children.

### Changed

- Each recursive coordinator joins and verifies its direct children, then
  returns one synthesized subtree result to its parent. Git operations remain
  with the top-level lead after its direct children finish.
- A lightweight contract test keeps the Codex and Claude Code guidance aligned
  without adding runtime code or agent-facing test output.

## 0.3.8 - 2026-07-15

### Added

- `mega-orchestration:wayfinding` maps long-horizon uncertainty before an
  honest specification or plan, using local decision records without requiring
  a tracker or commit behavior. Codex keeps it explicit-only through
  `agents/openai.yaml`.
- Five RED-backed artifact scenarios pin the new debugging, planning,
  authoring, review-axis, and wayfinding contracts. Validation now checks the
  supported Codex per-skill metadata shape and invocation policy.

### Changed

- Codex v2 guidance now treats native workers as same-model context shards,
  defaults independent work to fresh context, and keeps spawning, joins, and
  lifecycle ownership with the root agent.
- Debugging guidance now builds the smallest red-capable loop, ranks
  hypotheses by evidence and test cost, controls temporary probes, and defines
  substitute evidence for irreducibly external failures.
- Plans expose dependencies, blocker owners, unblock conditions, and staged
  expand-migrate-contract replacements. Planning, debugging, and project memory
  now distinguish repository context, ADRs, observed behavior, and historical
  hints.
- Skill authoring prunes no-op guidance and distinguishes hard dependencies
  from optional enrichment. Code review reports specification compliance and
  engineering standards as separate axes.

## 0.3.7 - 2026-07-14

### Added

- Codex `multi_agent_v2` setup guidance enables ten concurrent subagents and
  applies a model-visible depth-five task-path policy, with fresh-context
  dispatch guidance for independent reviewers and candidates.
- OpenAI's first-party `codex-plugin-cc` is documented as an optional Codex
  companion when Claude Code is the active lead.

### Changed

- Codex reviewer profiles now describe same-vendor code review explicitly and
  use the catalog's high review effort.
- Claude Code documentation distinguishes isolated subagents from experimental
  agent teams and no longer implies that agent teams enforce a single writer.
- Provider and orchestration guidance separates native Codex fan-out from
  cross-vendor independence and records v2's current context-fork behavior.

### Fixed

- The independent-review hook recognizes actual `codex-plugin-cc` review and
  adversarial-review commands without allowing its status command to suppress
  a required review.
- Validation locks the v2 config shape, mirrored model catalogs, reviewer
  profiles, and public release metadata against drift.

## 0.3.6 - 2026-07-14

### Added

- The core `upgrading-megapowers` skill inspects existing install channels,
  preserves pins and scopes, upgrades the installed set after one summarized
  approval, and offers relevant uninstalled plugin bundles separately.

### Changed

- Skill descriptions are shorter and retain the trigger cues and boundaries
  that distinguish neighboring workflows.
- The writing-skills reference now summarizes current OpenAI and Anthropic
  guidance instead of carrying a large vendor documentation snapshot.
- Validation measures complete Codex skill metadata, including a conservative
  reserve for the skill-root alias table, and fails before the 8,000-character
  fallback budget is exceeded.

## 0.3.5 - 2026-07-13

### Fixed

- Candidate anonymization preserves binary files and line endings, rejects
  symlinks, and still refuses any surviving authorship marker.
- Delegate resolution now honors empty-array overrides and enforces both
  halves of the configured tier and effort floor.
- Autonomous-run commands reject run IDs that can escape the run directory.
- Local validation ignores generated eval caches, checks new untracked files,
  uses no undeclared `rg` dependency, and measures the real SessionStart
  payload.

### Changed

- The Claude marketplace publishes the seven plugin bundles only. Individual
  skills remain available through the skills CLI for non-marketplace installs.
- Security lint requires an exact file allowlist for executable network fetches
  and reports when its unicode scan is unavailable.
- CI scores the JSON artifact from its single eval-suite run.
- Public docs now disclose the optional Node visual companion, the Go scorer,
  current Codex hook dispatch, and the Codex initial skills-list limit.

### Removed

- The unwired description-freeze script and its stale enforcement claims.
- The undeclared Pi harness reference.

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
  `evals/RESULTS.md` section 6.

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
