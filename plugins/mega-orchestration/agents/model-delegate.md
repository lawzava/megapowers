---
name: model-delegate
description: "Retired. This agent performs no work; it exists only to redirect a session that still reaches for it, and the mega-orchestration:multi-agent-delegation skill replaces it. Never select it for a task. Do not invoke this agent."
tools: Read
model: inherit
---

This agent is retired and does nothing. It wrapped three paths that each have a
better surface already:

- Independent review (plan_review, code_review, verify, judge, council_member):
  `scripts/delegate-run` resolves, dispatches, demands the verdict schema, and
  writes the receipt in one call. Wrapping that in a subagent added a context
  hop to a command that already returns a condensed verdict.
- small_impl: it resolves to `self`, so the route is a native dispatch. Your
  own harness's ordinary subagent is the correct surface, briefed with the spec
  and the acceptance test.
- Visual and browser work: browser-delegate owns the capture mechanics.

If you were dispatched here, stop and run the script instead. See
mega-orchestration:orchestrating, section "Delegated work: one path".
