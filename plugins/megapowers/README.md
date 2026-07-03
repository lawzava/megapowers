# megapowers (plugin)

A workflow methodology for Claude Code: a small set of process skills that
encode a disciplined way of moving from an idea to reviewed, tested, merged
code. Rather than a single monolithic assistant behavior, each phase of the
work has its own skill with clear entry conditions, so the right practice is
applied at the right moment.

## Methodology

The core loop is deliberate and evidence-driven:

- Understand before building. Start with brainstorming to explore intent,
  requirements, and design before any code is written.
- Plan the work. Turn a spec into an explicit, reviewable plan before touching
  code.
- Execute with discipline. Drive implementation through tests first, debug
  systematically instead of guessing, and delegate independent work to
  subagents or parallel agents when it helps.
- Verify and review. Confirm work actually does what it should before claiming
  completion, then request and receive code review with technical rigor.
- Integrate cleanly. Use isolated worktrees for feature work and follow a
  structured path to finish and merge a development branch.

## Attribution

megapowers is a restyled fork of
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License (© 2025 Jesse Vincent). See the repository `ATTRIBUTION.md` for
the full upstream notice and license terms.

## Skills

Fifteen skills ship with the plugin.

Process:

- brainstorming
- writing-plans
- executing-plans
- subagent-driven-development
- dispatching-parallel-agents
- test-driven-development
- systematic-debugging
- verification-before-completion
- requesting-code-review
- receiving-code-review
- using-git-worktrees
- finishing-a-development-branch

Memory:

- project-memory

Meta:

- using-megapowers
- writing-skills

## Discoverability

A SessionStart hook runs when a session begins and makes the skills
discoverable automatically, so the workflow practices surface at the right time
without manual lookup.

**Context cost, disclosed:** the hook injects the `using-megapowers` skill
plus a one-sentence preface (about 390 words, ~520 tokens), and the fifteen
skill descriptions add about 740 words (~980 tokens) of always-on metadata.
Skill bodies load only when a skill is invoked. Verify yourself:
`wc -w skills/using-megapowers/SKILL.md`.

## Install

```
/plugin install megapowers@megapowers
```

### Standalone skills

Five of these skills are also published as standalone marketplace entries:
`brainstorming`, `systematic-debugging`, `test-driven-development`,
`writing-plans`, and `writing-skills`. Install the bundle **or** a standalone
skill, not both — a skill installed twice registers twice.
