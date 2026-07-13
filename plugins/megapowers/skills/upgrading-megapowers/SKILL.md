---
name: upgrading-megapowers
description: Use when asked to update, upgrade, refresh, or migrate Megapowers, check for a newer release, or discover new Megapowers plugins.
license: MIT
---

# Upgrading Megapowers

**Core principle:** Inspect first, preserve policy, approve one exact plan, then verify observed state.

Read [the channel reference](references/channels.md) before choosing commands. Use only the section for the detected harness and install channel. For initial installation, use the repository setup guide instead.

## 1. Inspect: read only

Identify the harness and every visible Megapowers source. Record installed and enabled plugins, versions, scopes, marketplace or repository, pins, symlinks, forks, local edits, duplicates, hook state, and, on Codex, CLI and app-server parity.

Inspect available plugins and upstream release metadata without changing local state when the channel permits it. If provenance is ambiguous or managed files have local edits, stop before any write and show the conflict.

## 2. Classify

Classify every installed plugin:

- Floating marketplace install: target the latest stable release from the same source.
- Explicit tag, ref, or version: preserve pinned policy. Move to a named pin only with approval; never convert it to floating.
- Symlinked checkout: update only a clean checkout with an unambiguous upstream.
- Fork: propose an upstream integration; never overwrite local work.
- Duplicate or unknown source: report it. Cleanup is a separate opt-in change.

“Latest” means latest stable unless the user names a version, ref, branch, or prerelease.

## 3. Compare and propose

Separate the plan into:

1. **Upgrades:** already-installed plugins with an applicable target.
2. **Available additions:** bundles not installed and not overlapping any visible installed plugin, skill, or component.

Rank additions relevant first using repository evidence: language manifests and source, frontend files or design work, and orchestration needs. Offer `show all` for the full catalog. Describe newly included skills inside an upgraded bundle as part of that upgrade, not as separate installs. Say “available but not installed” unless release history proves when a bundle was introduced.

Report visible overlap as a same-source duplicate or cross-source conflict, not an addition.
Do not install an overlapping bundle while both registrations would remain active. If the user explicitly wants it, propose a migration that selects the one registration to keep and lists any disable or removal as an opt-in write.

Optional additions start unselected. Never install one without explicit selection.

Present one summarized approval request immediately before the first write. Include known current and target versions or refs, unresolved target policy if the cache is stale, exact installed upgrades, selected additions, preserved pins, scopes, and sources, warnings, writes, restarts, and verification. Read-only inspection needs no approval. If refresh changes the plan materially, summarize the delta and ask again.

Example approval shape:

```text
Upgrade: <installed plugin>, <current> to <target policy or ref>
Add: <explicitly selected bundle, or none>
Preserve: <source, scope, pin policy>
Warnings: <local edits, duplicates, hooks, restart, or none>
Writes: <marketplace refresh and exact plugin operations>
Verify: <state probes>
Proceed?
```

## 4. Apply

After approval, refresh the selected source and upgrade the installed set first. Re-inspect and verify those upgrades before installing any selected additions.

Do not silently change pins or sources, remove duplicates, discard edits, edit settings, trust hooks, or add plugins. Use `effect-broker` for external effects when available, but do not require that optional plugin.

## 5. Verify

Re-read actual state. Confirm the approved plugin set, enabled state, versions or refs, pins, scopes, source, duplicates, expected skill discovery after any restart, and hook status. On Codex, compare CLI and app-server/plugin state.

On partial failure, stop before optional additions. Inspect again and report **applied, failed, and not attempted** actions. Give the safest recovery step. Never claim rollback, loading, or success without observing it.

## Common mistakes

- Refreshing a marketplace before the approval gate because it seems harmless. It is still a write.
- Calling every uninstalled bundle newly introduced. Availability does not prove release timing.
- Assuming a failed update restored the old version. Only observed state supports that claim.
