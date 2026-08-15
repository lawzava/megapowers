---
name: upgrading-megapowers
description: Use when asked to update, upgrade, refresh, or migrate Megapowers, check for a newer release, or discover new Megapowers plugins.
license: MIT
---

# Upgrading Megapowers

Inspect first, preserve the existing installation policy, approve one exact
plan, then verify observed state. Read [the channel
reference](references/channels.md) and use only the detected installation
channel.

## 1. Inspect: read only

Identify every visible installation, including enabled plugins, versions,
scopes, sources, pins, local edits, duplicates, and hook or settings state.
Inspect available releases without changing local state. If provenance is
ambiguous or managed files have local edits, stop before the first write and
report the conflict.

## 2. Classify

Keep a floating install on its source's latest stable release. Preserve source
and scope, and preserve a pin unless the user explicitly approves changing it.
Treat a checkout with local edits, a fork, or a duplicate as a separate
decision, never an overwrite or cleanup implied by an upgrade.

Classify a user-owned baseline as absent, unrelated, or adopted. An adopted
baseline usually opens with a `megapowers-baseline vX.Y.Z` comment; that stamp
is the installed ref, so use it and skip inference. Without a stamp, infer the
ref and say so. Either way, compare the shipped baseline at that ref with the
target baseline, not with the user's edited file, and preserve the user's
edits; the diff between shipped refs is what you offer to apply. If the fetch
fails, report that the comparison did not run; an empty result is not no
drift.

## 3. Compare and propose

Separate upgrades from available additions and user-owned configuration drift.
Offer relevant additions first and `show all` for the full catalog. An addition
is optional until explicitly selected. Exclude a plugin overlapping any visible
component, and do not install both registrations simultaneously; propose an
explicit migration instead.

An upgrade can change the shipped model catalog, so check routing against this
machine rather than assuming it still resolves. Run mega-orchestration's
`probe-routes` and compare: a provider the new catalog routes to but this
machine cannot reach is an upgrade finding, not a silent regression, and so is a
newly shipped provider that is reachable here but disabled. Report both with the
`ALTERNATES` count. Hand any resulting configuration change to
`mega-orchestration:configuring-model-routes`, which writes the user's own
override layer; an upgrade never edits routing on the user's behalf.

Before any write, request one summarized approval covering targets, sources,
scopes, preserved pins, selected additions, expected writes, restart needs, and
verification. Read-only inspection needs no approval.

## 4. Apply

After approval, apply the selected upgrades first, then re-inspect before any
selected addition. Do not silently change pins, sources, scopes, settings,
hooks, local edits, or duplicates.

## 5. Verify

Re-read actual state and confirm the approved plugin set, enabled state,
versions or refs, pins, scopes, sources, duplicate status, and required restart
or hook state. On partial failure, stop before optional additions and report
applied, failed, and not attempted actions with the safest recovery step.

Stale cached version directories are not inert: a session that locates plugin
scripts by glob picks whichever version the glob finds first, and the 2026-08
audit caught a 0.11.5 `delegate-resolve` serving a session running 0.12.0. Do
not remove them until every session using them has restarted; once sessions
have restarted, remove the superseded version directories from the plugin
cache as part of the upgrade.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
