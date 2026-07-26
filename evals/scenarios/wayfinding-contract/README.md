# wayfinding-contract

Artifact oracle for wayfinding's Codex skill-metadata sidecar. It asserts two
things and nothing else:

1. The shipped `agents/openai.yaml` keeps `allow_implicit_invocation: false` and
   names `$wayfinding` in its default prompt, so the skill stays explicit-only on
   Codex. `scripts/validate.sh` exempts wayfinding from the `.agents/skills`
   discovery links on exactly that basis, so this marker is what keeps the
   exemption honest.
2. `scripts/validate-codex-skill-metadata` actually enforces the sidecar schema.

## Mutation checks

Every validator marker is a mutation, so the validator cannot pass by being a
no-op. The shipped sidecar must be accepted; each variant below must be
rejected:

- `$wayfinding` changed to `$wrong-skill` (proves prompt-to-skill coupling
  rather than a grep of the canonical file)
- active `allow_implicit_invocation: true` followed by a commented-out `false`
- a non-boolean implicit policy (`sometimes`)
- a quoted-string implicit policy (`"false"`)
- a too-short `short_description`
- a missing required interface field (`display_name`)

The valid-fixture arm additionally adds the documented optional interface icon
and brand keys plus `dependencies`, proving the focused parser accepts official
metadata it does not otherwise need to interpret.

## Scope

This scenario deliberately asserts nothing about the wording of `SKILL.md`.
Skill prose is meant to be trimmed as the suite de-prescribes, and pinning
phrases in an eval only taxes that work without measuring anything. The earlier
version of this oracle carried ten such markers covering the map contract,
decision files, the one-decision loop, tracker optionality, and the
orchestrating and README cross-references; they were removed on 2026-07-26 for
that reason.

The cross-references were not simply dropped. An independent review pointed out
that for an explicit-only skill the orchestrating route is not wording, it is
the only way anything can reach the skill, so losing it silently is a
functional regression. That invariant now lives in `scripts/validate.sh` as the
explicit-only reachability check: any skill whose sidecar sets
`allow_implicit_invocation: false` must be named as `<plugin>:<skill>` by some
other shipped skill. It is mutation-tested by removing the orchestrating route
and confirming the check fails. The plugin README catalog entry is covered
separately by the existing "README lists all shipped skills" check. Both assert
that a reference exists, never how it is phrased.
