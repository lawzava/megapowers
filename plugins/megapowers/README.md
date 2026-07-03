# megapowers (plugin)

The workflow core: fifteen process skills that take an idea to reviewed,
tested, merged code. Each phase of the work has its own skill with clear entry
conditions, so the right practice is applied at the right moment.

## Methodology

- Understand before building: brainstorm intent, requirements, and design
  before any code is written.
- Plan the work: turn a spec into an explicit, reviewable plan before touching
  code.
- Execute with discipline: drive implementation through tests first, debug
  systematically instead of guessing, and hand independent work to subagents
  or parallel agents when it helps.
- Verify and review: confirm the work does what it should before claiming
  completion, then request and receive code review with technical rigor.
- Integrate cleanly: use isolated worktrees for feature work and follow a
  structured path to finish and merge a development branch.

## Skills

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

## Discoverability and context cost

A SessionStart hook (Claude Code) injects the `using-megapowers` skill plus a
one-sentence preface at session start, so the agent checks for a matching
skill before acting instead of waiting for you to name one. The injection is
about 390 words (~520 tokens); the fifteen skill descriptions add about 740
words (~980 tokens) of always-on metadata. Skill bodies load only when a skill
is invoked. Verify yourself: `wc -w skills/using-megapowers/SKILL.md`.

## Install

```
/plugin install megapowers@megapowers
```

Five of these skills are also published as standalone marketplace entries:
`brainstorming`, `systematic-debugging`, `test-driven-development`,
`writing-plans`, and `writing-skills`. Install the bundle or a standalone
skill, not both: a skill installed twice registers twice.

## Attribution

megapowers is a restyled fork of
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License (© 2025 Jesse Vincent). See the repository `ATTRIBUTION.md` for
the full upstream notice and license terms.
