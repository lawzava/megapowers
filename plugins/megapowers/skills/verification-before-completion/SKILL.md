---
name: verification-before-completion
description: >-
  Use before claiming work is complete, fixed, passing, ready to merge or
  publish, and before commits or pull requests. Also use before stating a
  load-bearing fact the user will act on, such as what a file holds, what a
  system already does, that a tool cannot run, or that a security problem
  exists. Triggers on any success, status, capability, or finding claim.
license: MIT
---

# Verification Before Completion

**Core principle:** evidence before claims.

For each claim, identify the oracle, run it fresh, read its complete result,
and state only the status that result supports. A partial check, a linter, or
confidence is not evidence for a broader claim. If the oracle fails, report the
actual status and evidence instead of softening the claim. A bug regression
test must also fail without the fix.

## Claims that are not completion claims

The same gate applies to any fact the user will act on, stated mid-task: what a
file contains, what a system already supports, that a tool or command is
unavailable, or that a defect exists. These arrive before there is anything to
complete, so waiting for the completion report is waiting until after the user
has already acted.

Name the evidence class before stating the claim, because the three are not
interchangeable:

| class | what it proves |
|-------|----------------|
| executed check | the behavior, on this machine, now |
| artifact inspection | the artifact says so, not that anything uses it |
| inference | nothing; it is a hypothesis until a check runs |

A listing is not the file. `git status`, `ls`, and a file name prove existence,
never content or type. A string inside a binary proves the string is present,
not that the code path runs. Two earlier denials prove those two calls failed,
not that a capability is missing: probe the actual command before reporting it
unavailable.

A reported defect carries the command that reproduces it, in the same message.
A security finding with no reproduction is an inference, and saying it plainly
costs nothing next to withdrawing it later.

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

## Open the report with the verdict

A completion report starts with one line, before any detail:

```
VERIFIED: <claim>
NOT VERIFIED. Remaining: <what is unproven>
```

Nothing else goes on that line. A report that opens with what was done, then
buries an unproven item in paragraph four, reads as complete and gets acted on
as complete. If any acceptance criterion is unproven, the verdict is NOT
VERIFIED even when every other one passed, and even when the remaining work is
small. Dispatched agents whose results you have not read are unproven results,
not finished ones.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
