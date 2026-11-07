# Native dispatch and lane lifecycle

## Harness-specific examples (optional adapter)

Use the active tool schema, not these examples as an API promise. Bindings,
model selection, permissions, hooks and local session setup belong in the
user's configuration. Adapt argument names to the active harness version
without changing dispatch-before-join ordering. For any other Agent Skills
harness, use its native subagent and wait tools; if none exist, run the lanes
inline, one at a time. No external worker CLI is required.

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

OpenCode v2:

```text
a = subagent(agent="<configured-role>", description="lane A", prompt="bounded brief A", background=true)
b = subagent(agent="<configured-role>", description="lane B", prompt="bounded brief B", background=true)
# Continue independent lead work; collect terminal notifications for a and b.
```

omp (oh-my-pi, with task batching enabled):

```text
batch = task(context="shared constraints", tasks=[
  {name:"LaneA", agent:"<configured-role>", task:"bounded brief A", solutionSpace:"bounded outcome A"},
  {name:"LaneB", agent:"<configured-role>", task:"bounded brief B", solutionSpace:"bounded outcome B"}])
# Continue independent lead work; collect every terminal result in batch.
```

Native isolation is a separate capability, not implied by spawning a child.
Confirm the worker's root and base revision before it writes. For example,
native worktrees can start from a default branch rather than the lead's HEAD.
Use only isolation fields actually exposed by the active schema; otherwise
assign an existing isolated checkout or create an isolated Git worktree below. Disable automatic
integration where the harness permits it until fence and leak checks pass.

Background defaults depend on the harness and mode. In a non-interactive
run such as `claude -p`, ending the turn ends the session before background
agents report, so the answer loses their results. A scheduler or controller
that records the first end of turn as the run's result does the same: it
stores the interim note and marks the run done. In both cases, run lanes the
answer needs in the foreground (`run_in_background: false`), still issuing
them in one message so they run in parallel when the active schema supports
that call shape. If a foreground flag is absent, stay in the active session and
collect terminal notifications before returning the final answer. Set effort per lane
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
  normal Git worktree tools. Record the immutable base and worker root in
  the brief. The lane edits only there and never pushes or touches the lead
  branch. Check each patch for conflicts and overlapping ownership before
  applying it. Include commits and untracked files in the review. The lead
  applies one reviewed patch at a time, then reruns the real check. Preserve
  unapplied and ignored work until explicitly discarded. Worktrees start from
  commits, so uncommitted lead changes are absent. If isolation is unavailable,
  run writing lanes sequentially.
- Worktree lanes multiply build output. Share build caches across worktrees
  where the toolchain allows it, such as a common `CARGO_TARGET_DIR` or the
  default Go cache, and check free disk before fanning out builds.
- After a lane's work is merged or discarded, remove its worktree and branch
  in the same step, within the cleanup authority the user gave.
- Run a long independent review as a background lane when independent lead
  work remains and the session can collect its terminal result; otherwise use
  foreground review. Choose the available reviewer under the user's independence
  rules and check secrets and disclosure scope before sending off-machine.
- For a correction round, start a fresh agent with the original brief plus
  every later directive. A resumed agent may drop directives given after its
  start; keep correction context explicit and do not reuse it for a new round.
- When a review or red-team loop spends its round budget without converging,
  stop and bring the open findings to the user instead of starting more
  rounds.

Example sources read 2026-11-05 (documentation, not install-test claims):
[Claude Code](https://code.claude.com/docs/en/sub-agents),
[Codex](https://developers.openai.com/codex/agent-configuration/subagents),
[OpenCode v2](https://opencode.ai/v2/docs/agents), and
[omp](https://github.com/can1357/oh-my-pi/blob/main/docs/tools/task.md).
