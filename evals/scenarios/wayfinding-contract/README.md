# wayfinding-contract

Artifact oracle for the contract selected from a fresh-context behavioral
probe. The prompt and omissions below are the behavioral RED evidence. The
markers only prevent the resulting workflow and Codex sidecar from drifting.
The fixture arm also runs the repository's sidecar validator against a valid
copy, then changes `$wayfinding` to `$wrong-skill` and requires rejection. This
proves prompt-to-skill coupling instead of only grepping the canonical file.

## Mutation checks

The oracle rejects these contradictory or removed contracts:

- `issue tracker is not optional`
- `Never fail to implement or automatically commit.`
- removal of either `execute a plan` or `start an autonomous run` from the
  no-side-effect contract
- `Plan-ready is valid only when no approved design exists.`
- active `allow_implicit_invocation: true` followed by a commented-out `false`
- a non-boolean or quoted-string implicit policy, a too-short description, or
  a missing required interface field
- removal of the orchestrating route, README catalog entry, or brainstorming
  frontmatter boundary

The valid-fixture arm also adds the documented optional interface icon and
brand keys plus `dependencies`, preventing the focused parser from rejecting
official metadata it does not otherwise need to interpret.

## Fresh-context prompt

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md and plugins/megapowers/skills/{brainstorming,writing-plans}/SKILL.md plus plugins/mega-orchestration/skills/{orchestrating,autonomous-run}/SKILL.md. Scenario: a 3-month migration has unknown ownership, four unresolved architecture decisions, unclear sequencing, and no trustworthy issue tracker. The user wants help reducing uncertainty before a full spec or execution plan. Describe the current workflow, artifacts, how unknowns are represented, and the stop condition. Do not invent a new skill. No edits. Return proposed response and missing capability/guidance with exact line refs.
```

## Observed RED, 2026-07-15

The current workflow has downstream artifacts, but no durable pre-spec
uncertainty artifact:

- `brainstorming` explores context and converges on a written spec at lines 9
  to 11 and 32 to 44. `writing-plans` then decomposes approved requirements.
- `orchestrating:28-44` keeps undecomposable work inline or routes already
  shaped work. It does not preserve a long-lived uncertainty map.
- `autonomous-run:47-60` explicitly requires a goal that already survived
  design scrutiny, so its charter is too late for this scenario.
- None represents named fog, source trust, owners, decisions, evidence,
  dependencies, or a current frontier in a local artifact independent of an
  issue tracker.
- None provides a one-decision loop or stops at `spec-ready`, at `plan-ready`
  only when a design is already approved, or when blocked on named evidence.
- No wayfinding skill or Codex `agents/openai.yaml` sidecar exists, so there is
  no explicit-only Codex invocation policy or default `$wayfinding` prompt.

## Deterministic RED

`solve.sh` treats the absent skill and sidecar as missing contract markers
instead of trying to read them and crashing. `check.sh` rejects a missing
`out.txt`, any absent marker, and every `MISSING` marker. On the pre-guidance
tree it ends with:

```text
RED: wayfinding-contract incomplete
```
