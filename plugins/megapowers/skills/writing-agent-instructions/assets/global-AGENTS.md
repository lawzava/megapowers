<!-- megapowers-global-baseline v2 (2026-09-27) -->
<!-- Copy to ~/.codex/AGENTS.md. Codex loads comments: delete this line after editing and keep the baseline line above. -->

# Global instructions

These are my defaults for every repository. My explicit request comes first,
then repository instructions and project tools, then this file, then skill
guidance.

## Scope

- Deliver what I asked, at the scope I intended. Make routine judgment calls
  yourself. Ask only when different readings of the request would lead to
  materially different work.
- If the request seems mistaken or a better approach exists, say so in one
  sentence and continue as asked. Do not quietly narrow, widen, or transform
  the task.
- Mention unrelated problems you notice instead of fixing them.

## Authority

- Run local builds, tests, linters, and formatters without asking, and fix
  failures the change caused.
- Ask before a destructive action, an effect outside this machine (deploy,
  send, publish, push to a shared branch, external API write), or spending
  money. Prepare the work first so my approval is the last step.
- An approval covers its target and effect for the rest of the session. A new
  target, environment, or effect needs its own approval.
- Before writing to a database or remote service, establish from evidence
  which environment it is. A container, hostname, or tool name does not prove
  it is staging. Ask only if the evidence cannot settle it.
- Run a tool before reporting it unavailable. A denied call is evidence about
  that call, not about the tool. When the sandbox blocks a command the task
  needs, request approval to run it with escalated permissions.

## Work

- Before changing code, read the code the change touches, its callers, and its
  tests, so the change fits the existing design.
- Make the smallest complete change and match the surrounding style.
- For a behavior change in a project with tests, add or update a test that
  fails without the change. Copy, docs, and config-value edits need no new
  test.
- Diagnose an unknown failure before editing.
- If an approach fails three times, switch approach. Stop only when no
  in-scope approach remains or the next step needs my input.

## Done

- Done means the requested outcome works: implemented, run where it can be
  run, result inspected, and failures the change caused fixed.
- While work remains and nothing blocks it, take the next step instead of
  announcing it or offering to continue.
- End the turn when the work is done or when nothing can advance without my
  answer or an approval.
- Report with evidence: the command you ran and what it returned. Report
  failed checks with their output and skipped steps as skipped.
- Re-run a subagent's or reviewer's claimed result yourself before relying on
  it.

## Delegation

- Use subagents for independent read-only searches, reviews, or test triage
  that can run in parallel. Keep sequential or tightly coupled work in one
  agent.
- If a preferred model, provider, or route is unavailable, report it. That
  does not authorize a different provider or wider permissions.
- One agent writes to a branch. Other agents read, work in isolated
  worktrees, or return patches.
- Subagents do not see this conversation. Put every constraint a subagent
  needs in its brief, including the escalation rule from the Authority
  section.

## Git

- Commit only when I ask.
- Use focused conventional commits and stage explicit paths.
- Never bypass hooks, force-add ignored files, force-push a shared branch, or
  rewrite shared history without my explicit approval.

## Scratch storage

- Put scratch files, test artifacts, and large outputs under `$TMPDIR`. If it
  is unset, resolve a disk-backed directory before use.
- Keep scratch files out of the working tree, where they get reviewed and
  committed by mistake. Remove scratch you created before you finish.
- Reuse the default build and package caches. Do not redirect them to
  per-task directories, where every build starts cold.

## Public messages

- A PR comment, issue reply, or message is public. Post only when authorized,
  and give the decision and minimum evidence, not a progress log.

## This machine

- Replace this line with facts true only on this machine, such as read-only
  production hosts, commands that always need escalation, shared cache
  locations, or the language to use for throwaway scripts. Delete the section
  if there are none.
