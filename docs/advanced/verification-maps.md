# Verification maps

`verification/megapowers.json` is a repository-local pilot. It maps a
user-visible or release-facing journey to its harness check, runner, isolated
state, cleanup boundary, and retained evidence.

The map does not generate tests or replace an oracle. It points maintainers to
the existing executable path. Add a journey only after the same verification
path has been rediscovered or misapplied more than once.

Treat a product failure and map drift as separate defects. A runner can expose a
product bug while the map remains correct. A passing product can also have a
stale map if its harness, command, cleanup, or evidence contract changed.

Keep entries portable and public-safe. Use argument arrays, not shell strings.
Name only runner-owned state and cleanup. Do not add personal paths, account
identifiers, or raw runtime data.
