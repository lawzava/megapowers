# Cross-harness skill authoring best practices

This is the portable subset of current OpenAI and Anthropic guidance. Use the live sources for harness specific details:

* [OpenAI: Build skills](https://developers.openai.com/codex/skills)
* [Anthropic: Skill authoring best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)
* [Anthropic: Skills in Claude Code](https://code.claude.com/docs/en/slash-commands)
* [Agent Skills specification](https://agentskills.io/specification)

## Progressive disclosure

Agents discover skills from frontmatter metadata, load `SKILL.md` when selected, then read references or run scripts only when needed.

Metadata is the scarce layer. Names and descriptions compete with the system prompt,
conversation, and other installed skills. Codex caps its initial skill list at 2
percent of the context window or 8,000 characters when the context size is unknown.
It shortens descriptions first and may omit skills after the cap. Claude also keeps
model invocable skill descriptions in context.

The body has no startup cost, but once loaded it remains in the conversation.
References and scripts should keep optional detail out of the body.

## Frontmatter

Portable skills require `name` and `description`.

`name` uses lowercase letters, numbers, and hyphens, with at most 64 characters.
Prefer a specific gerund or action name over `helper` or `tools`.

`description` states what task the skill handles and when to use it. Write in
third person and put the main use case and trigger terms first. Include concrete
symptoms, technologies, or request language that distinguish the skill. Add a
neighboring skill boundary only when it prevents a plausible collision.

Do not summarize the workflow in the description. It selects the skill; the body defines the procedure.

Do not add invocation fields that only one harness understands. Keep harness
specific UI or policy metadata in that harness's configuration surface.

## Body

Assume the agent already understands common concepts. Include only knowledge,
constraints, decisions, examples, and checks that change the result.

Run a guidance-unit deletion test after drafting: remove each instruction,
bullet, field, and fragment in turn. Removal matters only if it changes
permitted behavior, a decision, an output, required evidence, or removes
wording that prevents a likely mistake. Otherwise the unit is a no-op; delete
it.

For scan-heavy workflow guidance, prefer a leading observable predicate,
action, artifact, gate, or concrete concept term when that improves recognition.
Define a nonstandard term at first use. Replace leading intensifiers and
mental-state prompts such as `carefully` or `think deeply` with that concrete
vocabulary.

State what to do. Explain why only where it prevents a likely mistake. Match instruction precision to risk:

* Use broad goals and heuristics when several approaches are valid.
* Use a preferred pattern when variation is acceptable.
* Use exact commands or scripts when the operation is fragile or must be
  deterministic.

Keep one skill focused on one job. Use imperative steps with explicit inputs,
outputs, and completion evidence. Quality critical workflows need a feedback
loop: run the check, fix the reported issue, and rerun until it passes or a real
blocker is reported.

Keep `SKILL.md` under 500 lines. This is a ceiling, not a target. Move detailed
reference material, examples, templates, and large schemas into supporting
files.

## References and scripts

Link every required reference directly from `SKILL.md`. Avoid reference chains; agents
may preview a nested file instead of reading it fully. Give long reference files a short contents section.

Use scripts when correctness is deterministic or repeated code generation would
waste context. State whether the agent should run a script or read it. Verify
dependencies rather than assuming they are installed. A hard dependency is
required for correct execution. Hard dependencies must not be skipped: block at
an explicit setup gate that says how to install, configure, or authorize them.
Optional enrichment does not block the correct core workflow. When it is
unavailable, skip it or use a stated fallback. Use fully qualified MCP tool
names when a skill depends on MCP.

Avoid time sensitive facts in durable instructions. Link to the live source or
isolate legacy details in a clearly labeled section.

## Evaluation

Create evaluations before extensive guidance:

1. Run representative tasks without the skill and record the actual failure.
2. Turn at least three real failures into scenarios.
3. Add the minimum instruction that fixes those failures.
4. Rerun the same scenarios and inspect the agent's path, not only the final
   answer.
5. Test every model family and harness the skill claims to support.

Test description recall and precision separately from body compliance. Positive
prompts should select the intended skill. Held out paraphrases should still
select it. Off topic and adversarial prompts should stay quiet.

## Release check

Before shipping, verify:

* The description is specific, front loaded, and distinguishes nearby skills.
* The body contains no explanation the target models already supply reliably.
* Safety, consent, verification, and evidence gates remain explicit.
* References are one level deep and loaded only when relevant.
* Examples are concrete and fewer than the rules they clarify.
* Trigger recall, precision, behavior evaluations, and repository validation
  pass.
