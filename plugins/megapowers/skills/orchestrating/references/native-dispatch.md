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
- Give each parallel writing lane its own worktree. Use the harness's native
  isolation when it works. When it is missing or refuses the repository, use
  the lane tool:

  ```bash
  go run <skill-dir>/scripts/lanes.go add --name NAME [--base REF]
  go run <skill-dir>/scripts/lanes.go check
  go run <skill-dir>/scripts/lanes.go apply --name NAME
  go run <skill-dir>/scripts/lanes.go remove --name NAME [--discard]
  ```

  `add` creates a detached worktree under
  `$TMPDIR/megapowers-lanes/<repo>-<hash>/NAME` and prints its path for the
  brief. The lane edits only there. It may commit, but it never pushes or
  touches the lead branch. `check` reports each lane as `clean`, `applied`,
  `empty`, or `conflicts` against the lead worktree. It exits 3 on a conflict
  or when two unapplied lanes touch one file, which breaks disjoint ownership.
  `apply` writes one lane's patch, including its commits and untracked files,
  into the lead worktree without committing; the lead then reruns the real
  check. `remove` refuses unapplied work and ignored files, which the patch
  never carries, unless `--discard` is given.
  `diff --name NAME` prints the patch, and `list` shows every lane. A lane
  starts from a commit, so lead changes that are not yet committed are absent.
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
