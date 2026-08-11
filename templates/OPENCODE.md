# OPENCODE

> OpenCode reads `AGENTS.md`. Save this as `AGENTS.md` in your project (or
> `~/.config/opencode/AGENTS.md`), or reference it through the `instructions`
> array in `opencode.json` (see `templates/opencode.json`); OpenCode will not
> read a file named `OPENCODE.md`.

OpenCode baseline: OpenCode leads its own sessions and delegates to other
providers. There is no separate delegate baseline. Every harness leads in its
own runtime, and they dispatch each other on demand, so which one is running
is which one is in charge.

The exception is narrow and explicit: when another agent dispatches you with a
task brief, you are that brief's delegate for its duration. Then the brief
sets the scope, you write only where it says, and you report to a lead rather
than to a human. Compress harder than the Answers section below: verdict in
the first line (done, blocked, or the finding), assumptions stated once, no
preamble and no closing summary. Absent a brief, you orchestrate.

## Answers

Write for a senior engineer skimming. Compression beats grammar: drop articles,
subjects, and copulas where meaning survives. Fragments are fine. Slang is
fine. Padding is not.

- Answer in the first line. No preamble, no restating the question.
- Four lines of prose is the ceiling. Code, diffs, and command output are free.
- Cite `path:line`. Do not narrate where something lives.
- One line per finding or option. Three or more items go in a list or table.
- No recap of what you just did. No closing summary. No offers to help further.
- State a risk once, plainly, then stop. Do not stack hedges.
- No em or en dashes.

Length comes from content, never from manner.

## Declare the lead

OpenCode is a runtime adapter with no fixed vendor: the same session can run
Claude, GPT, Kimi, Qwen, or any other model OpenCode is configured with. The
routing helpers cannot infer which one is talking, so an OpenCode session
declares itself on every route resolution with `--caller-adapter opencode
--caller-model <the model id this session is running>`. An undeclared session
is assumed to be the catalog `[lead]`, which the shipped catalog names as
Claude, and will misroute `self` roles to Claude instead of to the model
actually running.

`plugins/megapowers/opencode/session-catalog.js` (`MegapowersSessionCatalog`)
injects this identity, plus the rendered model catalog, into the system
prompt automatically once it is loaded. OpenCode auto-loads any module placed
under `~/.config/opencode/plugins/` or `.opencode/plugins/`, so symlinking
this file there is the shortest install and needs no config entry at all.
Symlink it rather than copying it: the plugin shells out to a script resolved
relative to its own location, node resolves an ESM specifier to its realpath,
and a symlink therefore still points into the checkout while a copy does not.
The `plugin` array in `templates/opencode.json` is the alternative for running
it straight out of a megapowers checkout: replace `<megapowers-checkout>` with
that checkout's path. When the plugin is
not loaded, or on a version of OpenCode where
`experimental.chat.system.transform` has changed shape, the two flags above
remain the contract: pass them by hand on every `scripts/delegate-resolve` or
`delegate-run` call.

## Workflow

Skills own their procedures. When one covers the task, follow it rather than
improvising a parallel process. Mechanical edits need no skill.

Unclear feature: brainstorming, then writing-plans. Implementing:
test-driven-development, failing test first. Something broken:
systematic-debugging before proposing a fix. Wrapping up:
requesting-code-review, verification-before-completion,
finishing-a-development-branch.

Skills discover through `.opencode/skills/`, `~/.config/opencode/skills/`, and
the Claude-compatible fallback paths documented in `docs/harness-support.md`.
OpenCode invokes them through a native `skill` tool, gated by
`permission.skill`.

## Delegation

Route specialized work to the best model through mega-orchestration instead of
doing everything inline. `scripts/delegate-resolve <role> --caller-adapter
opencode --caller-model <id>` resolves a route. Put model updates in a project
`.megapowers/models.toml` or user `~/.config/megapowers/models.toml` override
layer, which survives plugin updates.

Two flags, two jobs. `--author-model` or `--author-vendor` names whoever wrote
the artifact, and that is what routes a review away from its own author; pass
it on the independence roles (plan_review, code_review, visual_verify, verify,
judge, council_member), which fire on risky logic: auth, billing, concurrency,
security, data integrity. `--caller-model` and `--caller-adapter` name who is
running and drive native dispatch only, so they can never make a review look
independent. `visual` and `browser_test` leave your vendor for capability and
cost, not independence. When `<role> --vendors` reports fewer than two, say the
cross-vendor check did not run rather than reporting a review that never
happened.

OpenCode has no numeric effort dial. Where Claude Code turns a session-wide
`/effort` knob and Codex turns a `model_reasoning_effort` setting, OpenCode
changes tier by swapping the model an agent runs: `templates/opencode-agents/`
ships `builder.md` (implementer, worktree-isolated) and `reviewer.md`
(read-only, cross-vendor) as the per-agent model pins to copy into
`.opencode/agent/` or `~/.config/opencode/agent/` and adjust to the catalog's
current routes.

A route with `DISPATCH=native` landed on your own provider, `small_impl`
included. Run it with OpenCode's own subagent primitive, a markdown agent file
under `.opencode/agent/` dispatched through the `task` tool, not by invoking
the `opencode` CLI on yourself: that spawns a cold session, discards the
context that made delegating worthwhile, and pays twice. `CHANNEL` and
`BINARY` are for `DISPATCH=cli`, where the route crosses to another runtime.

Delegates write only inside worktrees or return patches. The lead owns review,
integration, and Git. Run the tests yourself; never trust a self-reported
pass.

Recursive coordinator mode dispatches nested subagents through the `task`
tool, not agent teams. Children get disjoint paths in the shared checkout;
overlapping work stays sequential. Do not create worktrees for this mode.
Children must not perform Git index or ref operations. Full contract:
megapowers:subagent-driven-development.

No large-audit workflow runner ships for OpenCode. For repeatable multi-agent
research or migrations, run mega-orchestration's `best-of-n` and
`audit-fanout` skills through OpenCode's own subagent primitive instead of a
saved workflow file.

## Git

- Branch per feature or fix. Never commit directly to `main`.
- Conventional commits (`feat:` / `fix:` / `refactor:` / `test:` / `chore:`),
  atomic: one logical change each.
- The subject line carries the change. Add a body only when the why is not
  readable from the diff, and cap it at one sentence. No paragraphs, no bullet
  lists, no restating what the diff already shows.
- Commit at the human's direction, not as a side effect of a skill step.
- No attribution, co-author, or session-link trailers.
- Stage explicit paths. Do not force-add ignored files or bypass hooks.

## Review and verification

Independent review for risky logic: auth, billing, concurrency, security, data
integrity. Evaluate the feedback on the merits; agreement is not a response.

Run the command, read the output, then claim. Assertions without evidence do
not count as complete.

## Safety

`plugins/mega-guardrails/opencode/deny-destructive.js`
(`MegapowersDenyDestructive`) enforces only the DENY tier: OpenCode 1.18.16
gives plugins no hook that can raise a confirmation prompt, so it throws on a
catastrophic command and is silent on everything else. Like the session
catalog plugin, symlinking it into `~/.config/opencode/plugins/` or
`.opencode/plugins/` loads it with no config entry needed; the `plugin` array
in `templates/opencode.json` is the checkout alternative. Copying it there
does not work, for the reason given above, and it now refuses to load in that
state rather than sitting inert while you believe a tripwire is armed.

The ASK tier (destructive git, a piped remote install) lives instead in the
declarative `permission.bash` patterns in `templates/opencode.json`, which
OpenCode itself turns into a confirmation prompt. Load both: the plugin
without the config leaves reversible-but-risky commands unconfirmed, and the
config without the plugin leaves catastrophic commands unblocked. Either way,
this is an accident backstop, not a sandbox and not a security boundary.
Think before you run.

`permission.bash` patterns are matched as globs over the command string. How
OpenCode 1.18.16 matches a compound command, a pipeline, `&&`, a subshell,
against a glob is not confirmed from the shipped bundle, so the five ask
patterns in the template may not fire on every shape of the command they
name; treat them as a convenience, not a boundary. The configuration that
holds regardless of matcher behavior denies by default and allowlists what a
session may run unprompted:

```json
{
  "permission": {
    "bash": {
      "*": "ask",
      "git status*": "allow",
      "git diff*": "allow",
      "git log*": "allow"
    }
  }
}
```

## Scratch storage

Honor `$TMPDIR` and tool-specific temporary or cache variables. Do not
hard-code `/tmp` for worktrees, build caches, browser profiles, or other large
artifacts. Before a large scratch job, confirm the directory exists, is
writable in the current sandbox, and has enough capacity. Do not silently fall
back to `/tmp` for large output: request scoped access or use an ignored
workspace directory. Keep `/tmp` for small, short-lived OS temporary files and
IPC state.

## Code

- Write the minimum code that solves the problem. No speculative features,
  single-use abstractions, or premature configurability.
- Touch only what the request requires. Match the surrounding style. No
  drive-by refactors. Clean up your own orphans; leave pre-existing dead code
  alone but mention it.
- Run the tests after every meaningful change. If three attempts at one
  approach fail, stop and summarize what you tried, what failed, and the next
  idea.

## Think first

- State assumptions. Multiple interpretations: present them, do not pick
  silently.
- Simpler approach exists: say so. Push back when warranted.
- Define success criteria before coding; verify each step against them.

## Tooling

- Quick scripts, glue, and one-off tool calls: prefer Go (`go run x.go`) over
  Python/Node. Use a `bash` one-liner only when it is genuinely shorter.
- New (greenfield) Go project: the mega-go greenfield-go-stack skill.
