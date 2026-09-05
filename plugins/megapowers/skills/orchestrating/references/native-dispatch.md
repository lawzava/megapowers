# Native dispatch examples

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
a = Agent(prompt="bounded brief A", run_in_background=true)
b = Agent(prompt="bounded brief B", run_in_background=true)
# Continue independent lead work, then collect a and b when both are terminal.
```
