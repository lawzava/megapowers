# Antigravity CLI Tool Mapping

Antigravity CLI (`agy`) has its own runtime concepts. Map skill language to
those primitives instead of assuming Claude or Codex tool names.

## Subagents and Background Work

- Use `/agents` to inspect active, completed, killed, or failed subagents.
- Use `/tasks` for non-agent background processes such as builds and test runs.
- Use the local approval shortcuts and detail views provided by `agy`; do not
  assume another runtime's permission model.

When a skill says to dispatch a subagent, use Antigravity's native subagent
mechanism if available. Keep the assignment scoped, include acceptance criteria,
and capture the result in the lead thread.

## Artifacts

When a skill asks for a todo list, plan, review checkpoint, screenshot, or
implementation proposal, prefer an Antigravity artifact. Artifacts are the
native review surface for plans, diffs, visual media, comments, and approvals.

Keep artifacts current:

- create a checklist artifact at the start of multi-step work
- update completed steps as work progresses
- attach screenshots or visual media for UI claims
- use artifact comments or approvals for review checkpoints

## Plugins and Skills

Antigravity CLI plugins use a root `plugin.json`. Native CLI skills are flat
markdown files under `.agents/skills/`, `~/.gemini/antigravity-cli/skills/`, or
the plugin `skills/` directory. This repository keeps canonical skills in the
open agent layout `skills/<name>/SKILL.md`; convert or symlink only the specific
skills you need if your local `agy` build does not import that nested layout.

## Safety

This repo ships no Antigravity delegate route. If you add one in a
`multi-agent-delegation` override layer, first verify local `agy` command
behavior, approvals, artifact review, and where file edits are written.
