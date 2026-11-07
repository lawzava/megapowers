---
name: upgrading-megapowers
description: Use when asked to update, upgrade, refresh, reinstall, or migrate Megapowers, check for a newer release, or repair stale installed plugin state.
when_to_use: "Trigger phrases: update megapowers, upgrade the plugin, is there a newer release, reinstall megapowers, stale plugin cache, plugin path not found."
metadata:
  short-description: Inventory, approve, and verify a Megapowers upgrade
---

# Upgrading Megapowers

Before any write, inventory visible registrations: enabled state and version,
exposed scope, marketplace name and source, pin or ref, local edits, duplicates,
caches, and active sessions. Read the stable release and changelog without
writes. Stop on ambiguous provenance, managed-source edits, or conflicting
installations. If already current, report a verified no-op.

Preserve enabled state, source or channel, scope, pin, and local edits. A
floating Git marketplace follows its latest stable release; a pinned or local
checkout changes only when explicitly approved. Keep one installation channel
per harness and exclude unrelated cleanup.

Follow the [channel reference](references/channels.md) for the inventory
commands, the release-commit to marketplace-head comparison, and the refresh
and registration commands. Never install unreleased branch state as a stable
upgrade. A registration that tracks the default branch is a channel defect:
report it and propose re-registering at `release` as a separately approved
write.

Request one exact approval covering target harnesses, enabled state, source,
scope, pins, current and target versions, writes, restart or cache effects, and
verification. Apply only that approved channel. For an approved pinned
checkout, update only its exact path and ref and keep the existing marketplace
registration. If a write fails, stop, read back state, and report applied,
failed, and not attempted steps.

Verify registration output and marketplace-source inventory: enabled state and
version, source, exposed scope, returned install path, and cached byte parity
against the target ref. Report partial application precisely. Restart before
expecting new guidance. Never delete a stale or superseded cache while an active
session may use it; removal needs separate approval, exact directories, and
proof that sessions restarted. When registration itself may prune a cache a
session still uses, snapshot it first and restore it if pruning occurs. Do not
invoke a model or provider session without explicit authorization; stop at
registration and cache proof.
