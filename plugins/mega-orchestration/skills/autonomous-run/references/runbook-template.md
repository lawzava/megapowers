# Runbook

The operating contract for one run. Two sections, and the split is the whole
point. MUST is blocking: every line is mechanically checkable, so a script or a
reader returns pass or fail on it without judgment. SHOULD is judgment, stated
because it is worth stating, and it never blocks. A constraint earns MUST by
being checkable, not by being important. Same lifecycle the hook-owned rules
use (`plugins/mega-orchestration/enforcement.toml`): rules ship advisory and get
promoted to blocking on evidence, never the other way around.

One section is a declared exception: Authorization. Placing an action in the
reversible, staged, or irreversible class is a human call, and those lines block
anyway, because an unwanted irreversible action is the one failure this contract
cannot undo. Every other judgment call lives in SHOULD.

Each constraint is stated once, at one level. The same rule written as both a
MUST and a SHOULD tells the run two different things about whether it blocks.
Keep run-specific rules in whichever half they belong to. A judgment call
promoted into MUST to be safe is how a run ends up blocked on nothing.

- [MUST (blocking)](#must-blocking)
  - [Sequence](#sequence)
  - [Ownership and freeze](#ownership-and-freeze)
  - [Working a milestone](#working-a-milestone)
  - [Journal](#journal)
  - [Evidence](#evidence)
  - [Authorization](#authorization)
  - [Closing](#closing)
- [SHOULD (advisory, never blocking)](#should-advisory-never-blocking)

## MUST (blocking)

### Sequence

1. Each milestone runs this order: read `status`, pick the next unmet
   milestone, do the work, run that milestone's declared acceptance check,
   journal the result, re-derive `status`.

### Ownership and freeze

2. `scripts/run-claim <run-id>` runs in every session that relies on the Stop
   hook to continue the run. Reading run files claims nothing.
3. `plan.md` is frozen with `scripts/run-init <run-id> --replan` before the
   first milestone is worked, so `plan-digest` exists.
4. A deliberate plan change is followed by another `--replan`.
5. `charter.md` is never edited after init. A changed goal is a new run.
6. `journal.md` is append-only and written only through `scripts/run-journal`.
   Never hand-edited, never tidied.
7. `status` is never hand-written. `scripts/run-derive-status <run-id>`
   produces it.
8. `charter.md` declares exactly one autonomy level.

### Working a milestone

9. A milestone completes only against the acceptance check declared for it in
   `plan.md`.
10. Before a new milestone opens, the completed milestones' acceptance checks
    named by `plan.md`'s `Regression subset:` line are re-run and pass.
11. Fix and re-verify attempts on one milestone stop at the `attempts` cap
    `charter.md` declares under Caps, default 3.
12. At that cap the milestone gets a `blocked` journal entry.
13. Each milestone boundary checkpoints the work through an already authorized
    commit or the durable ledger and working tree, then re-derives `status`.

### Journal

14. Every entry goes through `scripts/run-journal <run-id> <kind> <confidence>
    <msg>`; kind is action, decision, result, blocked, or paused; confidence is
    0.0 to 1.0.
15. Every message carries its milestone tag ("M2: ...").
16. A `result` entry names the declared check it ran and what that check output.
17. An entry for a delegated step cites the delegation artifact by path instead
    of pasting the brief.
18. Provenance is recorded: `run-init --model <model-id>`, or
    `MEGAPOWERS_MODEL=<id>` inlined on each `run-journal` call. An exported
    variable does not survive between tool calls; each runs in a fresh shell.

### Evidence

19. Every `charter.md` criterion appears verbatim as a row in `evidence.md`.
20. Each row's earned state is implemented, locally verified, or externally
    verified.
21. A row claims externally verified only when its external, UX, or database
    oracle column names a witness. Empty, `n/a`, or a local command there earns
    locally verified at most.
22. A row for an external database-backed criterion records caller, service,
    database, response, and visible-result cutpoints, with environment and
    correlation keys.

### Authorization

23. No commit, push, merge, deploy, or other external side effect happens
    without authorization that already existed. Starting a run grants none.
24. Every action is placed in one of the three classes
    `scripts/autonomy-level <level>` prints, REVERSIBLE, STAGED, or
    IRREVERSIBLE, before it runs. The class and the level `charter.md` declares
    decide together whether a human is needed.
25. An IRREVERSIBLE or high-blast-radius action waits for human approval at
    every level, staged through the effect broker where one exists.
26. A STAGED action follows the dial: proceed at `autonomous`, proceed with a
    journaled checkpoint at `on-the-loop`, pause for approval at `in-the-loop`.
27. A REVERSIBLE action inside the charter's blast radius proceeds without a
    human gate at every level.
28. At `in-the-loop`, each milestone boundary stops for approval before the
    next milestone opens.
29. In a scheduled or cloud run, where no permission prompt can reach a human,
    anything the effect broker would gate is simulated or deferred to an
    attended session, and this runbook records which.

### Closing

30. The run stops when every done-when criterion in `charter.md` is met or the
    charter's stop budget is exhausted.
31. `scripts/run-verify-status <run-id>` passes before any completion claim.
    STATE=done with LAST_VERIFY=none was never certified.
32. A pause is a trailing `paused` journal entry, not an abandoned session. Any
    later entry resumes the run.
33. `scripts/run-report <run-id>` runs at each checkpoint and at the end.
34. A controller roll, a compaction, or a session handoff happens only after
    the current milestone's journal entry is written and `status` re-derived.

## SHOULD (advisory, never blocking)

35. After a restart or a compaction, trust the files over memory. The journal
    and git history are the record.
36. Delegate a milestone when a different model or runtime is better at it
    (mega-orchestration:multi-agent-delegation).
37. Take inline execution for small or coupled milestones and subagent-driven
    execution for independently owned ones, then journal the choice.
38. Scale verification to the milestone's stakes. Money or auth earns a
    cross-model pass; a doc tweak does not.
39. Spend compute by stakes and uncertainty rather than uniformly.
40. Write an external dependency's acceptance check so it asserts where the
    dependency resolves from (for example its import path), not merely that it
    imports.
41. Treat the declared check as the real oracle. A substitute the work happens
    to pass closes nothing; rewrite the check instead of the claim.
42. Name what was tried and the next idea in a `blocked` entry. That entry is
    what the next session reads first.
43. Claim externally verified on a witness another person can look up, reached
    through the supported ordinary-user path rather than a developer shortcut.
44. Hand back early on a blocker only the human can clear instead of spending
    the remaining budget around it.
45. Near a stop budget, finish the current milestone cleanly and report instead
    of opening new work.
46. Roll the controller after 8 to 10 completed tasks, or before a task would
    cross 80 percent of the context or cache budget.
47. Reserve the last 20 percent of the budget for integration, verification,
    and synthesis.
48. Keep journal messages and report prose in the handoff register: conclusion
    first, declarative, self-contained.
49. Set journal confidence honestly. The report ranks decisions lowest
    confidence first, and that is where a human looks.
