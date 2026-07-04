# Agent install

You are a coding agent asked to install megapowers on this machine. Follow
these steps exactly, verify each one, and end with the report at the bottom.
Human-oriented docs (what megapowers is, updating, fleet sync):
[`setup.md`](./setup.md).

Ground rules:

- One channel per agent per machine. A skill registered twice fires twice.
  Before installing, check what is already present (step 1) and never add a
  second channel for the same harness.
- Do not widen your own permissions. If a step edits settings or instruction
  files (step 4), show the user the exact change and get approval first.
  Everything in steps 2 and 3 is additive plugin/skill installation and needs
  no such approval beyond your harness's normal prompts.
- Verify with commands, not assumptions. Every step has a check; run it.

## 1. Detect the environment

Determine which harness you are running in (you know this about yourself:
Claude Code, Codex, OpenCode, Antigravity, or another Agent Skills harness)
and check for existing installs:

- Claude Code: `claude plugin list 2>/dev/null | grep -i mega`
- Codex: `codex plugin list 2>/dev/null | grep -i mega`
- skills-CLI installs: `ls ~/.agents/skills ~/.claude/skills ~/.gemini/config/skills 2>/dev/null`
  and the project's `skills-lock.json`. These are shared directories that
  multiple harnesses read (see the matrix in step 2), so a hit can mean skills
  are already registered for more than one harness.
- Superpowers: `claude plugin list 2>/dev/null | grep -i superpowers`. The
  megapowers `megapowers` plugin is a superset of its process core; if the
  user wants both removed/replaced, uninstall superpowers first (ask, don't
  assume).

If megapowers is already installed for your harness, skip to step 5 and
verify instead of reinstalling.

## 2. Install for your harness

**Claude Code** (full bundle: skills + hooks + delegate agents):

```
claude plugin marketplace add lawzava/megapowers
claude plugin install megapowers@megapowers
claude plugin install mega-orchestration@megapowers
claude plugin install mega-guardrails@megapowers
```

Add `mega-go`, `mega-python`, `mega-ts` if the user works in those languages;
omit `mega-guardrails` if the user does not want the safety hooks.
Interactive sessions can use `/plugin` instead.

**Codex** (skills + marketplace metadata; the guardrail hooks are not ported
here yet):

```
codex plugin marketplace add lawzava/megapowers
codex plugin add megapowers@megapowers
codex plugin add mega-orchestration@megapowers
```

The verb is `add`, not `install`. `codex plugin marketplace add` accepts
`owner/repo[@ref]` (codex-cli 0.142.5+); unpinned tracks the default branch.
Updates: `codex plugin marketplace upgrade megapowers`, then re-run
`codex plugin add` for each plugin. Change-controlled installs pin with
`@v0.1.2` instead and update by re-adding at the new tag. To track a fork,
clone it and run `codex plugin marketplace add ./` from the checkout.

**OpenCode, Antigravity, or any other Agent Skills harness** (skills only):

CAUTION first: the skills CLI installs skills for many agents into SHARED skill
directories that several harnesses read, so one global install can register the
same skills in more than one harness at once:

| Shared global directory | Also read by                 |
|-------------------------|------------------------------|
| `~/.agents/skills/`     | Claude Code, OpenCode, Codex |
| `~/.claude/skills/`     | Claude Code, OpenCode        |

If step 1 found Claude Code plugins (or a prior `~/.claude/skills` install), do
NOT install globally into a shared directory: skills would register (and fire)
twice. Use a tool-specific path instead (for OpenCode, symlink from a checkout
into `~/.config/opencode/skills/`, or set `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1`
so OpenCode ignores the Claude paths), or install per-project without `-g`.

If step 1 found no conflicting channel, install globally:

```
npx skills add lawzava/megapowers -g -y -s '*' -a <your-agent-name>
```

Set `-a` to your harness, e.g. `-a opencode` or `-a antigravity`; `-a '*'`
targets every agent the skills CLI supports (see skills.sh for the accepted
names). Drop `-g` to install into the current project only.

## 3. Verify the install

Run the same probe the repo's install-smoke study uses: load the
test-driven-development skill and quote its core principle sentence verbatim.
Outside this guide and the setup doc, the sentence exists only inside the
skill body, so run the probe in a fresh session or a subagent that does not
have this guide in context; a correct quote from there proves discovery and
loading end to end. Expected sentence:

> if you didn't watch the test fail, you don't know whether it tests the
> right thing

Also confirm the listing: `claude plugin list` / `codex plugin list` shows
the plugins with matching versions, or the skills directory contains the
skill folders.

## 4. Optional, with explicit user approval

Present these to the user; apply only what they approve:

- Statusline (Claude Code, Linux): copy the plugin's `statusline.sh` to a
  stable path the user chooses and point `statusLine.command` in
  `~/.claude/settings.json` at it (installed-plugin paths are overwritten on
  update; see the mega-guardrails README).
- Settings baseline: merge wanted keys from
  [`templates/settings.example.json`](../templates/settings.example.json)
  (attribution off, secret-path denies, sandbox credential blocks). The
  `permissions.allow` entries widen agent permissions; never add those
  without the user's explicit yes.
- Instructions baseline: offer [`templates/CLAUDE.md`](../templates/CLAUDE.md)
  as the project or global instruction file. Merge, don't overwrite; back up
  the existing file first.
- Remove superseded duplicates: if the user previously hand-installed copies
  of these hooks or skills (session-start, deny-destructive, auto-format, or
  standalone skill folders that the bundles now provide), list them and offer
  to remove the old copies. Duplicates run twice.

## 5. Report

End with this table, filled with actual command output, plus anything you
skipped and why:

| Check | Result |
|---|---|
| Harness + channel used | e.g. Claude Code, native marketplace |
| Plugins/skills installed | names + versions from the list command |
| First-task probe (step 3) | quoted sentence matched: yes/no |
| Duplicates found/avoided | e.g. none; or "~/.agents/skills skipped, Claude plugins present" |
| Optional steps applied | which of step 4, with user approval noted |
