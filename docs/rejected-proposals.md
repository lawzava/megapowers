# Rejected proposals

Ideas reviewed for Megapowers and not adopted, with the reason, so later
reviews do not reopen them without new evidence. Add an entry when a proposal
is declined; remove one only when new evidence reverses it.

| Proposal | Source | Decided | Reason |
|---|---|---|---|
| Anti-rationalization tables and red-flag sections in each skill | addyosmani/agent-skills skill anatomy | 2026-10-21 | Loaded skill text already binds; the 425-word body cap keeps activation cheap, and upstream reports no measured gain from the longer form. |
| An always-injected router ("1% chance, you must invoke") | addyosmani/agent-skills `using-agent-skills` | 2026-10-21 | Megapowers removed its always-injected payload earlier; hooks gate the outward and completion steps instead. |
| One interview question at a time until high confidence | addyosmani/agent-skills `interview-me` | 2026-10-21 | `grill-me` asks the whole open frontier per round with a recommended answer. |
| Generic definition-of-done, security, and performance checklists | addyosmani/agent-skills `references/` | 2026-10-21 | Megapowers uses task oracles, run receipts, and named gaps instead of standing checklists. |
| Personas with slash-command orchestration | addyosmani/agent-skills `agents/` | 2026-10-21 | Native agents with bounded briefs cover the same need on both harnesses. |
| External actions proceed without asking; owner-run merges | pstack autonomy guidance, `autopilot-full` | 2026-10-21 | Conflicts with `safe-effects` authority checks and the never-merge rule. |
| Model-slug routing tables and Cursor-only features | pstack | 2026-10-21 | Vendor-specific; the capability registry stays portable. |
| Formal-verification skills (TLA+) | pstack `create-formal-verification-skill` | 2026-10-21 | Niche for a general plugin; revisit on user demand. |
| Tracker-publishing flows (tickets, specs, triage) | mattpocock/skills `to-tickets`, `to-spec`, `triage` | 2026-10-21 | External writes to trackers fall under `safe-effects`; no supported workflow needs a dedicated skill. |
| No nested-delegation limit in briefs | mattpocock/skills `.out-of-scope/subagent-recursion.md` | 2026-10-21 | Current Claude Code subagents can spawn agents, so the brief limit still matters. |
| A dedicated code-review skill | addyosmani `code-review-and-quality`, pstack review skills | 2026-10-21 | The removed `code-quality` skill fired in 2 of 9 recall probes; harness-native review commands cover ordinary review and `independent-review` covers cross-vendor review. |
| Skip the completion gate for commits in scratch repositories | session audit 2026-10-21 | 2026-10-21 | Scratch repositories are usually reached through shell variables the hook cannot resolve; a variable-based bypass would also skip real commits. The cost is one skill load per session. |
