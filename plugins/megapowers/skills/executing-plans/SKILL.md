---
name: executing-plans
description: Use to execute a written implementation plan inline with one writer, especially coupled tasks. Use subagent-driven-development for independent tasks with per-task review.
license: MIT
---

# Executing Plans

Read the plan critically before starting. Use `megapowers:using-git-worktrees`
to establish or confirm an isolated, non-main workspace; work in place only if
the user has declined isolation. Resolve an ambiguous requirement, unsafe
assumption, or missing verification with the plan owner; do not invent scope
while executing.

Use this skill for coupled work with one writer. Use
`megapowers:subagent-driven-development` when independent tasks benefit from
separate ownership and review. A selected workflow grants no authority to
commit or change Git policy.

Execute tasks in order, invoke any named skills, and use the plan checkboxes as
the durable progress record. Check off a task only after its stated
verification passes. Resume from the plan, not recollection.

When verification fails, investigate and repair within the approved scope. Stop
and report a real blocker, such as a missing dependency or an unresolved plan
gap, rather than guessing. Revisit the plan if evidence invalidates its
approach.

After all tasks pass their verification, apply the project's review and
integration process. If it has no stronger process, use
`megapowers:requesting-code-review` then
`megapowers:finishing-a-development-branch`. Completion needs the evidence
required by `megapowers:verification-before-completion`.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
