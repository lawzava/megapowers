# PR body

Write the description for a reviewer who has two minutes. Follow the
repository's pull request template when it has one; this guide fills gaps.

## Summary

Lead with what changes for the user or caller, in one or two sentences. Then
show the change in its smallest visual form: a short call tree, file tree,
before and after snippet, or Mermaid diagram. Skip the visual when one
sentence already says it.

## Evidence

Show before and after for each behavior the change claims:

- the command or user journey you ran, and its result on each side;
- the commit the evidence belongs to;
- what the evidence does not cover, such as deployment, other platforms, or
  load.

Link saved logs or screenshots instead of pasting long output. Keep sensitive values,
private hosts, and raw agent transcripts out of the body.

## Merge risk

State whether merging is a two-way door (a revert fully undoes it) or a
one-way door (data migration, published artifact, external message, deleted
state). For a one-way door, name the blast radius: who or what is affected and
how you would recover. Name the reviewers or checks the risk needs.

## Scope

List anything the change does not do that a reader might expect, and any
follow-up work. Mention unrelated problems you noticed instead of fixing them
in this pull request.

Opening, editing, or commenting on a pull request is an outward write: follow
`safe-effects` for the destination and wording.
