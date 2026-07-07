# megapowers (plugin)

The workflow core: fifteen process skills that take an idea to reviewed,
tested, merged code. Each phase of the work has its own skill with clear entry
conditions, so the right practice is applied at the right moment.

## The skills, by phase

- Understand and design: `brainstorming`
- Plan: `writing-plans`
- Execute a plan: `executing-plans` (inline, yourself),
  `subagent-driven-development` (a fresh subagent per task, with per-task
  review), `dispatching-parallel-agents` (independent tasks in parallel)
- Implement: `test-driven-development` (write the failing test first),
  `systematic-debugging` (root cause before any fix)
- Verify and review: `verification-before-completion` (evidence before
  claiming done), `requesting-code-review`, `receiving-code-review`
- Integrate: `using-git-worktrees` (isolated feature work),
  `finishing-a-development-branch` (merge, PR, keep, or discard)
- Memory: `project-memory` (durable project knowledge across sessions)
- Meta: `using-megapowers` (the session-start check-for-a-skill rule),
  `writing-skills` (create and test new skills)

## Discoverability and context cost

A SessionStart hook (Claude Code) injects the `using-megapowers` skill plus a
one-sentence preface at session start, so the agent checks for a matching
skill before acting instead of waiting for you to name one. The injection is
about 290 words (~390 tokens); the fifteen skill descriptions add about 690
words (~920 tokens) of always-on metadata. Skill bodies load only when a skill
is invoked. Verify yourself: `bash hooks/tests/session-start.test.sh` prints
the exact payload word count (and gates it at 300).

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
