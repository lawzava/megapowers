---
name: wayfinding
description: Use when long-horizon work has unknown ownership, unresolved decisions, or unclear sequencing that prevents an honest specification or implementation plan.
license: MIT
---

# Wayfinding

Reduce uncertainty until the work is ready for design or planning. Wayfinding
is discovery and decision resolution, not delivery.

Runtime invocation metadata stays outside this portable body in an
[adapter sidecar](agents/openai.yaml).

## Local contract

Use `.megapowers/wayfinding/<id>/` by default:

- `.megapowers/wayfinding/<id>/map.md`: outcome, known regions, fog, source
  trust, owners, named decisions, evidence, dependencies, and the current
  frontier.
- `decisions/<decision-id>.md`: one question, its owner, evidence and source
  trust, options, status, and consequences for the map.

The default `.megapowers/` path is ignored personal workspace state. If the
user wants a shared committed map, agree on a project-owned path first and use
the same contract there. Never force ignored artifacts into git. Mirroring the
map into an issue tracker is optional; the local artifacts remain complete
without a tracker.

## Loop

1. Read the strongest available repository and external sources. Record what
   each source establishes and how much it should be trusted.
2. Create or refresh `map.md`. Separate known regions from fog, name owners and
   unresolved decisions, record dependencies, and mark one current frontier.
3. Resolve one decision at a time. Write or update its decision file with the
   evidence, options, status, and map consequences.
4. Update the map after each decision: propagate consequences, revise fog and
   dependencies, then update the current frontier.

## Stop and hand off

- **Spec-ready:** the outcome, constraints, behavior, and success criteria can
  be designed without inventing missing facts. Hand off to
  `megapowers:brainstorming` when installed.
- **Plan-ready:** plan-ready is valid only when an approved design already
  exists and the remaining uncertainty no longer prevents honest
  decomposition. Hand off to `megapowers:writing-plans` when installed.
- **Blocked:** name the missing external evidence, its owner, and what would
  unblock the current frontier. Leave the map ready to resume.

These are terminal states. Do not hand directly to implementation or an
autonomous run.

## Safety and oracle

The readiness oracle is the current map plus one decision file for every
resolved decision. A handoff with unresolved named decisions is not spec-ready
or plan-ready. Do not implement, execute a plan, start an autonomous run, or
automatically commit while wayfinding.
