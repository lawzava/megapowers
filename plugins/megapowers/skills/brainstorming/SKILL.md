---
name: brainstorming
description: Use to design a feature, capability, or unclear behavior change. Triggers on "add a feature" or "figure out the approach". Use wayfinding when uncertainty blocks scoping. Skip mechanical edits and bug fixes.
license: MIT
---

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design.

## The Gate: Proportional to Blast Radius

Scale the approval gate to how hard the work is to reverse:

- **Reversible, low-stakes work** (a config flag, a self-contained utility, a refactor covered by tests, anything a `git revert` undoes cleanly): present a short design so your intent is legible, then proceed. You do not have to stop and wait for a sign-off.
- **Hard-to-reverse or high-stakes work** (schema/data migrations, public API or contract changes, a new external dependency or service, anything touching auth, billing, payments, security, or concurrency, or work whose approach is genuinely ambiguous): present the design and get explicit approval before you implement.

When unsure which bucket you're in, treat it as the second. The goal is legibility and catching wrong assumptions early, not a mandatory human interrupt on every project.

## The Conversation

- Explore the current project state first (files, docs, recent commits). In existing codebases, follow existing patterns; include targeted improvements where existing problems affect the work, but propose no unrelated refactoring.
- If the request spans multiple independent subsystems, flag that before refining details. Help the user decompose into sub-projects, then brainstorm the first one; each sub-project gets its own spec, plan, and implementation cycle.
- One question per message. Focus on purpose, constraints, and success criteria. YAGNI ruthlessly.
- Propose 2-3 approaches with trade-offs, leading with your recommendation and reasoning. Give effort estimates on both scales, human-team time and agent time, so the compression is visible at decision time.
- Present the design in sections scaled to their complexity, covering architecture, components, data flow, error handling, and testing.
- Confirm sections proportionally (see The Gate): for hard-to-reverse or high-stakes work, confirm each section before moving on; for reversible work, present the whole design so your intent is legible and proceed.
- Design for isolation: break the system into units with one clear purpose and well-defined interfaces, each understandable and testable without reading its internals and changeable without breaking consumers.

## The Spec

Write the validated design to `docs/megapowers/specs/YYYY-MM-DD-<topic>-design.md` (user preferences for spec location override this default). Specs are handoff artifacts: senior-engineer register (see using-megapowers, Communication), conclusion first, plain declarative prose, self-contained.

Do not commit the spec as a side effect of this skill. Leave it in the working tree; the human commits it (or asks you to) when they choose. Some setups also gitignore agent artifacts under `docs/`, so a forced commit would fight their tooling.

After writing, reread the spec with fresh eyes: fix placeholders, internal contradictions, ambiguous requirements, and scope too broad for a single implementation plan. Fix issues inline and move on; no re-review needed.

For hard-to-reverse or high-stakes work, ask the user to review the written spec and wait for their response; apply requested changes and recheck. For reversible, low-stakes work, note where the spec lives and move straight to the plan, surfacing it for review rather than blocking on it.

## Handoff

Invoke the writing-plans skill to create a detailed implementation plan. This is the terminal state. Do not invoke any implementation skill from here. The only skill brainstorming hands off to is writing-plans.

## Showing Instead of Telling

When a question would be clearer shown than told (a real mockup, layout, or diagram question, not merely a UI topic), write a self-contained HTML or SVG mockup to a temp file and open or send it with whatever the harness provides. Keep it lightweight; regenerate the file as the conversation refines it.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
