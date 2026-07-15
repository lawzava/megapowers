# skill-authoring-quality

Artifact oracle for the contract selected from a fresh-context behavioral
probe. The prompt and omissions below are the behavioral RED evidence. The
markers only prevent the resulting guidance from drifting later.

## Fresh-context prompt

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md, plugins/megapowers/skills/writing-skills/SKILL.md, plugins/megapowers/skills/writing-skills/authoring-best-practices.md, and de-prescription-rubric.md. Audit this mini-skill text: 'Use when improving code. Carefully inspect the code. Think deeply. Follow best practices. Make changes. Verify the result. You may read docs, run tests, and ask questions. If another skill is relevant, use it.' Identify whether completion is checkable, which sentences are no-ops, hard vs soft dependencies, and whether leading words help scanning. Explain which current guidance would catch each issue and which guidance is missing, with line refs. No edits.
```

## Observed RED, 2026-07-15

The response could use the completion-evidence rule at
`authoring-best-practices.md:52-55` to reject an uncheckable finish. It could
also use the general minimum-guidance rule at lines 42 to 45 and the rubric's
micro-instruction removal rule at lines 11 to 18. It still omitted three
checkable authoring gates:

- No guidance-unit deletion test for identifying instructions, bullets, fields,
  or fragments whose removal changes no behavior. The probe exposed sentence
  no-ops; the same gap applies to the other guidance forms.
- No hard-dependency versus optional-enrichment distinction. Lines 66 to 69
  say to verify dependencies, but do not gate hard setup or require graceful
  degradation when optional enrichment is unavailable.
- No scan-leading vocabulary rule. Lines 30 to 35 front-load description
  triggers, but do not prefer observable predicates, actions, artifacts, or
  gates as the leading words of workflow guidance.

## Deterministic RED

`solve.sh` requires each selected contract in the authoring guide and its
matching keep or removal criterion in the de-prescription rubric. `check.sh`
rejects a missing `out.txt`, any absent marker, and every `MISSING` marker. On
the pre-guidance files it ends with:

```text
RED: skill-authoring-quality contract incomplete
```

## Mutation evidence

Removed the `observable-leading-vocabulary` line from an otherwise complete
`out.txt` fixture. `check.sh` exited 1 with:

```text
missing marker: observable-leading-vocabulary
```

Copied the completed guide and rubric into a dependency fixture. With the valid
rules `hard dependencies must not be skipped` and `optional enrichment does not
block`, all four markers were `OK` and `check.sh` exited 0. After reversing the
rules to allow skipping hard dependencies without setup and make optional
enrichment block the core workflow, the hard-dependency and optional-enrichment
markers were `MISSING` and `check.sh` exited 1.

Changed the guide rule to `Do not run a guidance-unit deletion test` and added
the same reversal to the rubric fixture. Only
`guidance-unit-deletion-no-op` was `MISSING`, and `check.sh` exited 1.

Changed the guide and rubric rules to `do not prefer` observable or concrete
leading vocabulary. Only `observable-leading-vocabulary` was `MISSING`, and
`check.sh` exited 1.
