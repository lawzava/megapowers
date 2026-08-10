---
name: configuring-model-routes
description: Use when setting up or revising which models and harnesses this machine delegates to, when delegation resolves to a provider that is not installed, or after an upgrade changes the shipped catalog. Triggers on "configure my models", "set up delegation", "no available route", or "which models should I use".
license: MIT
---

# Configuring Model Routes

Match the shipped catalog to what this machine can actually reach, and write the
difference to the user's own override layer. The shipped catalog is an opinion
about good routes; it cannot know which CLIs are installed or which vendors the
user pays for. That gap is what this skill closes.

Never edit a shipped file. The layers are project `.megapowers/`, then user
`~/.config/megapowers/`, then shipped, and the first two survive plugin
upgrades. Default to the user layer: harness availability is a fact about the
machine, not about one repository.

## 1. Probe before proposing

Run `scripts/probe-routes` (in the multi-agent-delegation skill) and read the
result. It reports, per catalogued provider, whether the harness binary exists,
which catalogued models that harness lists, and the resulting `ALTERNATES`
count. It is read-only and offline; it writes nothing and spends nothing.

Do not skip it and reason from memory of what is usually installed. The whole
point is that this machine is not the usual one.

Two answers mean different things and must not be conflated. `models=none of
the catalogued ones` means the harness is there and genuinely does not carry
them. `models=unknown` means the listing command failed, so the catalog stands
and the route survives; report it as unverified rather than as broken.

## 2. Read the gap out loud

State three things before proposing any edit:

- Which routes the shipped catalog wants that this machine cannot reach.
- Which reachable providers the catalog ships disabled, so they are available
  but unused.
- The `ALTERNATES` count, and whether cross-vendor review can run at all. Below
  one, say so plainly: no amount of configuration produces a second opinion
  from a single vendor.

`probe-routes` answers "is this configured", not "will this respond". An expired
login, an account gate, or an exhausted quota all look reachable to it. Say that
when it matters, and use `delegate-resolve <role> --vendors` to confirm a route
before anything depends on it.

## 3. Recommend, then ask

Propose a concrete layer and explain each line in one clause: what it enables or
disables, and what measurement or constraint stands behind it. Recommend
defaults rather than presenting an open menu, and make the recommendation
first so it is the easy answer.

Ask the user before writing. One consolidated question covering the whole
proposed layer, not one question per provider. Offer the obvious alternatives,
including writing nothing and taking the output as a diff to apply by hand.

The user's answer wins over the recommendation, including when it costs
quality or independence. Record the choice; do not relitigate it on the next
run.

## 4. Write only the difference

Write the smallest layer that expresses the decision. A key that already matches
the shipped catalog does not belong in an override: it is noise that silently
pins a value the shipped file may later improve.

`probe-routes --suggest` emits a starting point. Treat it as a draft, not as the
answer; it enables what it found and knows nothing about what the user wants.

Show the exact file content and path before writing. After writing, re-run
`delegate-resolve --check` and one representative role to confirm the layer
parses and resolves the way the user was told it would.

## 5. Verify against behaviour, not against the file

A layer that parses is not a layer that works. Confirm at least one independence
role resolves cross-vendor and reports the `ALTERNATES` the user was promised.
If a route was configured for a provider whose auth is unverified, say which
routes remain unproven rather than reporting success.

Origin: written for megapowers.
