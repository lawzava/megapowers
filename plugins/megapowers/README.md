# megapowers plugin

One workflow plugin for current Claude Code and Codex: task skills.
It uses the planning, agents, goals,
permissions, memory, worktrees, and browser tools each harness already has.
It ships no model router,
orchestration daemon, or vendor choice.

## Contents

- `skills/`: one skill per task type: orchestration, structured interviews,
  research, design, implementation with language-specific code judgment,
  debugging, verification, actions with real-world side effects, long
  unattended runs, independent review, prose, and writing agent instructions
  (including retrospectives, session recall, and memory hygiene).
  `skills/catalog.json` records whether each skill is stable or experimental,
  since portable skill frontmatter has no field for that.
- `skills/index.json`: the OpenCode HTTP catalog for all 13 skills and their files.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: plugin
  metadata for each harness.

## Communication style

The plugin does not apply a session-wide style. Invoke
`humanizing-prose` for writing tasks that need its editing guidance.
An optional Claude Code output style lives in repository-only
[extras](https://github.com/lawzava/megapowers/blob/main/extras/README.md#output-style)
for manual installation.

The plugin contains no executable code, hooks, or MCP configuration. It needs
no Go runtime. Optional helpers and setup skills stay in repository-only extras.

## Install

Add the repository marketplace at the `release` branch, then install
`megapowers@megapowers`. The complete commands, fresh-session checks, and
diagnosis steps are in
[the installation guide](https://github.com/lawzava/megapowers/blob/main/docs/install.md).

## Security

The plugin supplies workflow guidance, with no command guard or gates. Use
the harness permission system and OS sandbox for enforcement. The
independent-review skill supplies guidance only. The agent chooses another
available model under your reviewer instructions and checks secrets and
disclosure scope before sending material off-machine. Read the
[security policy](https://github.com/lawzava/megapowers/blob/main/SECURITY.md)
before enabling external review.

Repository-only helpers and setup skills are described in
[extras](https://github.com/lawzava/megapowers/blob/main/extras/README.md).
They do not ship with this plugin or its HTTP skill catalog.

## Attribution

The workflow descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License. See the
[full attribution](https://github.com/lawzava/megapowers/blob/main/ATTRIBUTION.md).
