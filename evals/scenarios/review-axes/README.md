# review-axes

Artifact oracle for the contract selected from a fresh-context behavioral
probe. The prompt and omissions below are the behavioral RED evidence. The
markers only prevent the resulting reviewer format from drifting later.

## Fresh-context prompt

```text
Read-only RED probe in /tmp/megapowers-skills-lessons. Read AGENTS.md, plugins/megapowers/skills/requesting-code-review/SKILL.md, and code-reviewer.md. Scenario: implementation is clean, well-tested, and idiomatic, but violates an explicit requirement; a second implementation meets the requirement but has a serious maintainability flaw. Show how the current reviewer would report and prioritize each. Assess whether the output keeps specification compliance and engineering standards as separately visible axes without re-ranking one into the other. No edits. Return proposed output plus gaps with exact line refs.
```

## Observed RED, 2026-07-15

The response had to place the missed requirement and maintainability defect in
one shared severity ladder. The current reviewer checks plan alignment at lines
53 to 56 and engineering concerns at lines 58 to 92, but its output contract at
lines 105 to 134 merges both into one `Strengths`, `Issues`, and `Assessment`
shape:

- No separate `Specification Compliance` and `Engineering Standards` output
  sections or local verdicts.
- `Critical`, `Important`, and `Minor` exist at lines 112 to 119, but only in
  the shared ladder, so findings can be reranked across concerns.
- The return summary at line 166 likewise reports one combined issue list.
- The final `Ready to merge?` contract already exists at line 132 and must be
  preserved while reporting both axes.

## Deterministic RED

`solve.sh` checks both axis structures and verdict criteria, rejects reversed
separation and authorization rules, scopes both local verdicts and
`Ready to merge?` to the output template's Final Assessment, verifies the
readiness mapping, and confirms every downstream review workflow blocks on a
Specification Compliance Fail. `check.sh` rejects a missing `out.txt`, any
absent marker, and every `MISSING` marker. On the pre-guidance reviewer it ends
with:

```text
RED: review-axes contract incomplete
```

## Mutation evidence

Removed the `engineering-axis-severities` line from an otherwise complete
`out.txt` fixture. `check.sh` exited 1 with:

```text
missing marker: engineering-axis-severities
```

Reversed the reviewer rule from `is specification noncompliance` to
`is not specification noncompliance`. `solve.sh` changed only the
specification marker to `MISSING`, and `check.sh` exited 1 with:

```text
MISSING specification-axis-severities
RED: review-axes contract incomplete
```

Removed the `Ready to merge?` line from the output template's Final Assessment
while leaving the example intact. Only `ready-to-merge-preserved` was
`MISSING`, and `check.sh` exited 1.

Reversed `Do not merge, average, or rerank` to `Merge, average, or rerank` in a
reviewer fixture. Only `findings-not-merged-or-reranked` was `MISSING`, and
`check.sh` exited 1.

Reversed the receiving workflow to say a Specification Compliance Fail `does
not block proceeding`. Only `specification-fail-blocks-downstream` was
`MISSING`, and `check.sh` exited 1.
