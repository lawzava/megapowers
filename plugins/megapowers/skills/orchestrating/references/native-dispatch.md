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

Claude Code runs agents in the background by default. Set `effort` per lane
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
- When a review or red-team loop spends its round budget without converging,
  stop and bring the open findings to the user instead of starting more
  rounds.
