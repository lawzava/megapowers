# Independent review

Get an independent opinion from another model when executable checks leave
material uncertainty in security, authentication, billing, concurrency, data
integrity, or similar work. The skill supplies guidance only, with no provider
launcher or credential handling.

Follow the user's global CLAUDE.md or AGENTS.md for the definition of
independence and reviewer choice. Otherwise choose an available native
subagent with a different model or the harness's native
review. Prefer cross-vendor review when available; it is optional unless the
user requires it. Report an unavailable required reviewer without silently
replacing it. No reviewer binding ships with the plugin.

Give the reviewer artifact identity, requirements, acceptance boundary, scope,
and check evidence. Keep access read-only. Inspect the brief and accessible
files for secrets and disclosure scope before sending anything off-machine,
including through native review. Follow existing authorization and permission
rules. The skill provides no automated secret scan or sandbox.

Require `verdict`, `scope.creep`, `scope.shrink`, and `findings`. Each finding
names severity (blocker, major, or nit), file, line, claim, and evidence. Require
explicit empty lists and verification limits. The lead computes fail when any
blocker, major, creep, or shrink exists, regardless of the stated verdict.
Nits never fail. Missing fields or results are gaps, never passes. Verify each
finding, record disproof for false positives, fix real failures, and rerun the
acceptance oracle.

Set the round cap before dispatch. Use a fresh reviewer for each correction
round without resetting the cap. Supply the original brief and later
directives, then review only fixes since the prior reviewed artifact,
affected boundaries, and new evidence. Preserve artifact identities and
relevant uncommitted content so the delta is reliable. Inspect each correction
brief for secrets and disclosure scope. Exhaustion leaves unresolved findings
open for the user. Join every review before completion.
