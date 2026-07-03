# agent-notify (template)

Push a notification when an agent needs you or finishes, so supervision costs
a glance at your phone instead of watching a terminal. Three small scripts,
copied and adapted from a working setup:

- `agent-notify` — the transport. Sends a Telegram message (and rings the
  terminal bell through tmux when present). Best-effort by design: a network
  blip or missing config never breaks the agent run that called it.
- `agent-notify-claude` — Claude Code hook wrapper. Filters noise: only
  permission prompts notify from the Notification event, questions and
  plan approvals notify from PreToolUse, and Stop suppresses the false
  "done" while background tasks are still running.
- `agent-notify-codex` — Codex `notify` program. Maps `agent-turn-complete`
  to "done".

## Install

```bash
cp agent-notify agent-notify-claude agent-notify-codex ~/.local/bin/
chmod +x ~/.local/bin/agent-notify*
mkdir -p ~/.config/agent-notify
printf 'TG_BOT_TOKEN=PUT_BOT_TOKEN_HERE\nTG_CHAT_ID=PUT_CHAT_ID_HERE\n' > ~/.config/agent-notify/telegram.env
chmod 600 ~/.config/agent-notify/telegram.env
```

Fill in the token (from @BotFather) and chat id. Then wire the hooks.

Claude Code, in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "Notification": [
      { "hooks": [ { "type": "command", "command": "$HOME/.local/bin/agent-notify-claude need" } ] }
    ],
    "PreToolUse": [
      { "matcher": "AskUserQuestion|ExitPlanMode",
        "hooks": [ { "type": "command", "command": "$HOME/.local/bin/agent-notify-claude ask" } ] }
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "$HOME/.local/bin/agent-notify-claude done", "async": true } ] }
    ]
  }
}
```

Codex, in `~/.codex/config.toml`:

```toml
notify = ["/absolute/path/to/.local/bin/agent-notify-codex"]
```

Test without an agent: `agent-notify need "hello from the template"`.

## Swapping the transport

Telegram is one choice, not the design. To use ntfy, Slack, or anything else,
replace the final `curl` block in `agent-notify`; the event mapping, tmux
bell, and hook wrappers stay as they are. Keep the two properties that make
it safe to wire into hooks: exit 0 on delivery failure, and a hard timeout on
the network call.
