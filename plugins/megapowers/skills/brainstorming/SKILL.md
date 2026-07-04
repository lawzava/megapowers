---
name: brainstorming
description: Use before starting new or non-trivial design work — a new feature, component, or capability, or a behavior change whose approach is unclear — to explore intent, requirements, and design before implementing. Triggers on "I want to add <feature>", "work out the design", "figure out the approach", "let's build". Skip for mechanical edits, well-specified one-liners, and straightforward bug fixes.
license: MIT
---

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design.

## The Gate — Proportional to Blast Radius

You're in this skill because the work is non-trivial design work (the frontmatter already routes mechanical edits, one-liners, and straightforward bug fixes elsewhere). Still, scale the approval gate to how hard the work is to reverse — don't gate reversible work on a human when you don't need to:

- **Reversible, low-stakes work** (a config flag, a self-contained utility, a refactor covered by tests, anything a `git revert` undoes cleanly): present a short design so your intent is legible, then proceed. You don't have to stop and wait for a sign-off.
- **Hard-to-reverse or high-stakes work** (schema/data migrations, public API or contract changes, a new external dependency or service, anything touching auth, billing, payments, security, or concurrency, or work whose approach is genuinely ambiguous): present the design and get explicit approval before you implement.

When unsure which bucket you're in, treat it as the second. The goal is legibility and catching wrong assumptions early — not a mandatory human interrupt on every project.

## Checklist

Create a task for each of these items and complete them in order:

1. **Explore project context** — check files, docs, recent commits
2. **Offer the visual companion just-in-time** — see the Visual Companion section below for when and how.
3. **Ask clarifying questions** — one at a time, understand purpose/constraints/success criteria
4. **Propose 2-3 approaches** — with trade-offs and your recommendation
5. **Present design** — in sections scaled to their complexity; for hard-to-reverse work confirm each section before moving on (see The Gate)
6. **Write design doc** — save to `docs/megapowers/specs/YYYY-MM-DD-<topic>-design.md` (write the file; do not commit it — commits happen at the human's direction, not as a side effect of this skill)
7. **Spec self-review** — quick inline check for placeholders, contradictions, ambiguity, scope (see below)
8. **User reviews written spec** — for hard-to-reverse or high-stakes work, ask the user to review the spec before proceeding; for reversible work this is optional
9. **Transition to implementation** — invoke writing-plans skill to create implementation plan

## The Process

**Understanding the idea:**

- Check out the current project state first (files, docs, recent commits)
- Before asking detailed questions, assess scope: if the request describes multiple independent subsystems (e.g., "build a platform with chat, file storage, billing, and analytics"), flag this immediately. Don't spend questions refining details of a project that needs to be decomposed first.
- If the project is too large for a single spec, help the user decompose into sub-projects: what are the independent pieces, how do they relate, what order should they be built? Then brainstorm the first sub-project through the normal design flow. Each sub-project gets its own spec → plan → implementation cycle.
- For appropriately-scoped projects, ask questions one at a time — only one
  question per message; break bigger topics into multiple questions
- Prefer multiple choice questions when possible, but open-ended is fine too
- Focus on understanding: purpose, constraints, success criteria
- YAGNI ruthlessly: remove unnecessary features from every design

**Exploring approaches:**

- Propose 2-3 different approaches with trade-offs
- Present options conversationally with your recommendation and reasoning
- Lead with your recommended option and explain why

**Presenting the design:**

- Once you believe you understand what you're building, present the design
- Scale each section to its complexity: a few sentences if straightforward, up to 200-300 words if nuanced
- Cover: architecture, components, data flow, error handling, testing
- Confirm sections proportionally (see The Gate): for hard-to-reverse or high-stakes work, confirm each section before moving on; for reversible work, present the whole design so your intent is legible and proceed — don't stop for a per-section sign-off you don't need
- Be ready to go back and clarify if something doesn't make sense

**Design for isolation and clarity:**

- Break the system into smaller units that each have one clear purpose, communicate through well-defined interfaces, and can be understood and tested independently
- For each unit, you should be able to answer: what does it do, how do you use it, and what does it depend on?
- Can someone understand what a unit does without reading its internals? Can you change the internals without breaking consumers? If not, the boundaries need work.
- Smaller, well-bounded units are also easier for you to work with - you reason better about code you can hold in context at once, and your edits are more reliable when files are focused. When a file grows large, that's often a signal that it's doing too much.

**Working in existing codebases:**

- Explore the current structure before proposing changes. Follow existing patterns.
- Where existing code has problems that affect the work (e.g., a file that's grown too large, unclear boundaries, tangled responsibilities), include targeted improvements as part of the design - the way a good developer improves code they're working in.
- Don't propose unrelated refactoring. Stay focused on what serves the current goal.

## After the Design

**Documentation:**

- Write the validated design (spec) to `docs/megapowers/specs/YYYY-MM-DD-<topic>-design.md`
  - (User preferences for spec location override this default)
- Specs are handoff artifacts: senior-engineer register (see using-megapowers,
  Communication) — conclusion first, plain declarative prose, self-contained
- Do **not** commit the spec as a side effect of this skill. Leave it in the working
  tree; the human commits it (or asks you to) when they choose. Some setups also
  gitignore agent artifacts under `docs/`, so a forced commit would fight their tooling.

**Spec Self-Review:**
After writing the spec document, look at it with fresh eyes:

1. **Placeholder scan:** Any "TBD", "TODO", incomplete sections, or vague requirements? Fix them.
2. **Internal consistency:** Do any sections contradict each other? Does the architecture match the feature descriptions?
3. **Scope check:** Is this focused enough for a single implementation plan, or does it need decomposition?
4. **Ambiguity check:** Could any requirement be interpreted two different ways? If so, pick one and make it explicit.

Fix any issues inline. No need to re-review — just fix and move on.

**User Review (proportional):**
For hard-to-reverse or high-stakes work, after the spec review loop passes, ask the user to review the written spec before proceeding:

> "Spec written to `<path>`. Please review it and let me know if you want to make any changes before we start writing out the implementation plan."

Wait for their response, and if they request changes, make them and re-run the spec review loop. For reversible, low-stakes work you may note where the spec lives and move straight to the plan — surface it for review rather than blocking on it.

**Implementation:**

- Invoke the writing-plans skill to create a detailed implementation plan. This is the terminal state.
- Do not invoke any implementation skill from here. The only skill brainstorming hands off to is writing-plans.

## Visual Companion

A localhost browser companion for showing mockups, diagrams, and visual
options. Don't offer it upfront: wait until a question would genuinely be
clearer shown than told (a real mockup / layout / diagram question, not merely
a UI *topic*), then offer it as its own message and wait:

> "This next part might be easier if I show you — I can put together mockups, diagrams, and comparisons in a browser tab as we go. It's still new and can be token-intensive. Want me to? I'll open it for you."

If they accept, read `visual-companion.md` (in this skill's directory) before
proceeding — it carries the per-question browser-vs-terminal test and the
server workflow. If they decline, continue text-only and don't offer again
unless they raise it.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
