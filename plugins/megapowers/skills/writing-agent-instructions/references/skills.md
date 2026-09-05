# Skill authoring

Use a skill when the model needs reusable task judgment, domain context, or a
workflow that should load only for relevant requests. Keep repository-wide
facts in repository instructions and hard enforcement in deterministic
tooling.

## Define the boundary

Start with representative requests. Include direct requests, indirect wording,
and near misses that share terms with the skill but need a different workflow.
Name the decision or capability the skill adds. If the model can already do the
task reliably from the repository and ordinary tools, do not add a skill.

The frontmatter `description` carries the actual trigger for Codex and is the
primary discovery text in the Agent Skills standard. Put what the skill does,
when it applies, and the narrowest useful boundary there. Front-load the main
use case because clients may shorten catalog entries. In this repository,
retain `when_to_use` and `metadata.short-description`, but do not rely on them
to repair a vague `description`. Keep implicit invocation enabled unless the
user explicitly requests an invocation-only workflow.

For an explicit-only workflow, configure each supported harness separately:

- For ChatGPT and Codex, set `policy.allow_implicit_invocation: false` in
  `agents/openai.yaml`. Keep the skill available for explicit invocation.
- For Claude Code, set `disable-model-invocation: true` in `SKILL.md`
  frontmatter.

One field does not configure the other harness. When a distributed skill must
be explicit-only in both, set both controls and verify both clients.

## Write the entrypoint

State decisions and constraints that materially change task execution. Prefer
outcomes and decision criteria over a fixed sequence when more than one method
can work. Preserve the user's chosen scope and current authority. A skill may
prepare an external action; it does not grant permission for that action.

Keep shared guidance in `SKILL.md`. Move substantial mode-specific procedures,
schemas, or examples into focused references, and link each reference at the
point where it becomes relevant. Avoid reference chains. Add a script only when
repeated mechanics need deterministic execution; make it self-contained,
bounded, and easy to verify. Do not add another scripting runtime for
instruction validation.

Do not add a README, changelog, installation guide, sample directory, UI
metadata, or compatibility field unless the distribution format or requested
workflow needs it.

## Validate behavior

Check format and discovery separately from usefulness. A valid file can still
trigger poorly or produce a worse result.

Use realistic cases with concrete paths, project context, incomplete evidence,
and competing instructions:

- Positive cases cover direct and indirect requests where the skill should
  materially improve the result.
- Negative cases are close alternatives where activation would distract or
  expand scope.
- Pressure cases test whether the skill preserves user intent, repository
  authority, existing edits, and effect boundaries when the prompt is long or
  urgent.

For a new skill or a material change to its trigger, scope, or workflow, run the
same cases against the prior version or a no-skill baseline. Inspect whether the
skill activated and whether the final artifact or decision met the case oracle.
Repeat model-based trigger tests because activation varies between runs. Revise
from patterns across cases, not one failed phrase. For typo, punctuation, link,
or quoting corrections that do not change behavior, use the relevant format,
link, and discovery checks without a baseline model run. Move stable,
machine-checkable requirements into a validator or test instead of adding more
prose.

## Sources reviewed 2026-09-05

- [OpenAI, Build skills](https://developers.openai.com/codex/skills): skill
  discovery, implicit activation, `agents/openai.yaml` invocation policy,
  focused authoring, and trigger testing.
- [Anthropic, Extend Claude with skills](https://code.claude.com/docs/en/skills):
  `disable-model-invocation`, supporting files, and baseline outcome evaluation.
- [Agent Skills specification](https://agentskills.io/specification): required
  frontmatter, resource layout, relative references, and progressive disclosure.
- [Agent Skills, Optimizing skill descriptions](https://agentskills.io/skill-creation/optimizing-descriptions):
  intent-focused descriptions and positive, near-miss negative, repeated trigger
  evaluation.
These links are evidence for current client behavior and format constraints.
Recheck them when authoring for a newer client release.
