# Plan Format

The document structure a plan produces. Read this before writing the plan file.
Parallel safety and ownership stay in SKILL.md because
`scripts/ownership-preflight` parses them.

## Plan Document Header

Every plan starts with this header:

```markdown
# [Feature Name] Implementation Plan

> **For agentic workers:** Required sub-skill: use megapowers:subagent-driven-development (recommended) or megapowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

**Tech Stack:** [Key technologies/libraries]

## Global Constraints

[The spec's project-wide requirements: version floors, dependency limits,
naming and copy rules, platform requirements. One line each, exact values
copied verbatim from the spec. Every task's requirements implicitly include
this section.]

---
```

The checkbox syntax is a contract: executing-plans flips these boxes as its
progress ledger.

## Task Structure

Each task (`### Task N: [Component Name]`) declares its files, interfaces, and
`Blocked by` task relationships, then its steps as checkbox (`- [ ]`) items.
Use `Blocked by: None` when it has no dependency. For every material unresolved
input, add `Blocker:`, `Owner:`, and `Unblocks when:` fields and mark the
affected task not execution-ready until that condition is met.

**Files:** exact paths, grouped as Create, Modify, and Test. Prefer symbols or
section names over unstable line ranges. Include exact line ranges only when a
subtle algorithm, protocol, or interface cannot be implemented reliably
without them.

**Interfaces:** what the task consumes from earlier tasks and produces for
later ones, with exact function names, parameter and return types. A task's
implementer sees only their own task; this block is how they learn the
names and types neighboring tasks use.

For a broad compatibility-sensitive replacement whose consumers cannot change
atomically, order separate expand, migrate, and contract tasks. Expand adds the
compatible path, migrate moves every consumer while both paths work, and
contract removes the old path only after migration is verified. Each stage
must leave tests and deployment green for every supported mixed state.

**Steps:** each names one observable outcome. State the failing behavior, the
minimal implementation target, and the exact verification command with its
expected result. Include code only for a subtle algorithm, protocol, fixture,
or interface where prose would leave a binding decision unresolved. The
checkpoint records progress and commits only when separate authorization
already exists.

## No Placeholders or Binding Gaps

Every step contains the decisions the engineer needs. These are plan failures:
- "TBD", "TODO", "implement later", "fill in details"
- "Add appropriate error handling" / "add validation" / "handle edge cases"
- "Write tests for the above" without naming behavior and an oracle
- "Similar to Task N" when the dependency or interface remains implicit
- A requirement with no owning task or verification command
- References to types, functions, or methods not defined in any task
