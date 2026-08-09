---
name: verification-before-completion
description: Use before claiming work is complete, fixed, passing, ready to merge or publish, and before commits or pull requests. Triggers on any success or status claim.
license: MIT
---

# Verification Before Completion

**Core principle:** evidence before claims.

For each claim, identify the oracle, run it fresh, read its complete result,
and state only the status that result supports. A partial check, a linter, or
confidence is not evidence for a broader claim. If the oracle fails, report the
actual status and evidence instead of softening the claim. A bug regression
test must also fail without the fix.

Keep implementation, local verification, and external verification distinct. A
local test does not prove a deployment, external service, or user-visible
result. Exercise the behavior through the interface a real user or caller uses,
such as the deployed URL, published CLI, or public API, when that behavior is
in scope.

For work with multiple acceptance criteria or external witnesses, copy each
criterion verbatim into a map with its implementation target, local oracle,
required external oracle, earned state, and evidence. For external proof,
record the environment and correlation key at each cutpoint: caller request,
service receipt and decision, target read or write, outward response, and
user-visible result.

If a required tool, input, or environment is unavailable, disclose it. A
fallback does not satisfy the original requirement until the requirement owner
accepts it.

The gate applies before a completion report, commit, pull request, handoff, or
next task.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
