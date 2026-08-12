# OpenCode session catalog plugin

Gives an OpenCode-led session the same always-in-context model catalog that
Claude Code's `SessionStart` hook and Codex's `codex-session-catalog.sh`
inject: the tier scale, the enabled providers, and the flags a session needs
to declare itself (`--caller-model`, `--caller-adapter`) when resolving
delegate routes. Without this, an OpenCode lead starts blind to all of that.

## What it does

`session-catalog.js` exports `MegapowersSessionCatalog`, a plugin that
implements `experimental.chat.system.transform`. It shells out to
`../hooks/render-model-catalog --caller opencode` and appends the rendered
catalog block plus one caller-identity line to the system prompt. The `--caller`
flag is what puts OpenCode on the block's lead line; without it the block
renders the catalog `[lead]` (Claude, in the shipped copy) and an OpenCode
session reads that as the answer to who is in charge.
That append happens on EVERY chat request, because opencode builds the system
prompt array fresh per request; appending once per session would put the
catalog in the first turn and nowhere else. The shell-out is what gets cached,
once per process, so appending every turn costs nothing after the first.

The caller-identity line states, verbatim, which `providerID/modelID` the
session is running and that route resolution takes `--caller-model <modelID>
--caller-adapter opencode`, the payload that lets a BYO-model runtime
declare itself without the human remembering the flag.

## How to load it

Add it to the `plugin` array in `opencode.json`/`opencode.jsonc`:

```json
{
  "plugin": ["<megapowers-checkout>/plugins/megapowers/opencode/session-catalog.js"]
}
```

or symlink the file under `~/.config/opencode/plugins/` for it to load into
every project. Symlink rather than copy. This plugin resolves
`../hooks/render-model-catalog` relative to its own file, and node resolves an
ESM specifier to its realpath, so a symlink still points into the checkout
while a copy points at a `../hooks/` that is not there. A copied install
prints one error to stderr and then injects nothing.

## Caveats

- `experimental.chat.system.transform` is **undocumented upstream**: it is
  present and triggered in opencode 1.18.16 and in the installed
  `@opencode-ai/plugin` 1.17.12 types, but it does not appear in
  opencode.ai/docs/plugins. It may change shape or disappear in a later
  opencode release without notice.
- Because of that, the plugin is written to fail open at every step: if the
  hook is never called, if `render-model-catalog` fails, or if the hook's
  input/output shape changes underneath it, the plugin no-ops rather than
  breaking a chat turn or failing the session. The one case it will not pass
  over in silence is a missing `render-model-catalog`, which means the plugin
  was copied instead of symlinked; that prints an error once. The guardrail
  plugin treats the same misinstall as fatal and refuses to load, because a
  missing catalog is merely absent while a missing tripwire looks exactly like
  a clean session.
