# Native dispatch and lane lifecycle

Adapt argument names to the active harness version without changing the call
order.

Codex:

```text
a = spawn_agent(task_name="lane_a", message="bounded brief A")
b = spawn_agent(task_name="lane_b", message="bounded brief B")
# Continue independent lead work, then wait until a and b are terminal.
```

Claude Code:

```text
a = Agent(prompt="bounded brief A", effort="medium")
b = Agent(prompt="bounded brief B")
# Continue independent lead work, then collect a and b when both are terminal.
```

Claude Code runs agents in the background by default. In a non-interactive
run such as `claude -p`, ending the turn ends the session before background
agents report, so the answer loses their results. A scheduler or controller
that records the first end of turn as the run's result does the same: it
stores the interim note and marks the run done. In both cases, run lanes the
answer needs in the foreground (`run_in_background: false`), still issuing
them in one message so they run in parallel. Set `effort` per lane
when the brief or the capability registry names one; a mechanical lane rarely
needs the lead's effort.

## Lane lifecycle

- Wait on completion notifications or a blocking wait. Do not create
  placeholder agents or sleep loops to pass time.
- Before reporting a lane as running, confirm it is alive from the harness's
  task list, its output, or its process. A lane whose last output is hours old
  is stalled until shown otherwise.
- Worktree lanes multiply build output. Share build caches across worktrees
  where the toolchain allows it, such as a common `CARGO_TARGET_DIR` or the
  default Go cache, and check free disk before fanning out builds.
- After a lane's work is merged or discarded, remove its worktree and branch
  in the same step, within the cleanup authority the user gave.
- Run a long cross-provider review as a background lane, not a foreground
  call that blocks the lead.
- For a correction round, start a fresh agent with the original brief plus
  every later directive. Resume an existing agent only when it holds state that
  is costly to move; a resumed agent may drop directives given after its start.
- When a review or red-team loop spends its round budget without converging,
  stop and bring the open findings to the user instead of starting more
  rounds.
