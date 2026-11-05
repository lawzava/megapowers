# megapowers plugin

One workflow plugin for current Claude Code and Codex: task skills, an
optional Claude Code output style, and a self-diagnosis skill. It uses the planning, agents, goals, permissions, memory, worktrees,
and browser tools each harness already has. It ships no model router,
orchestration daemon, or vendor choice.

## Contents

- `skills/`: one skill per task type: orchestration, structured interviews,
  research, design, implementation with language-specific code judgment,
  debugging, verification, actions with real-world side effects, long
  unattended runs, independent review, prose, safe upgrades,
  writing agent instructions (including retrospectives, session recall, and
  memory hygiene), and self-diagnosis (`megapowers-doctor`).
  `skills/catalog.json` records whether each skill is stable or experimental,
  since portable skill frontmatter has no field for that.
- `output-styles/`: an optional native Claude Code style for direct, concise replies.
- `.claude-plugin/plugin.json` and `.codex-plugin/plugin.json`: plugin
  metadata for each harness.

## Output style

Claude Code: the style is off until selected. Choose `megapowers:Megapowers`
in the `/config` output-style picker or run
`/output-style megapowers:Megapowers`; the bare value `Megapowers` does not
resolve. The style preserves the built-in coding instructions and respects
another selected style.

Codex: the plugin does not apply a session-wide style. Invoke
`humanizing-prose` for writing tasks that need its editing guidance.
Bundled Go scripts need Go 1.25 or newer; the plugin contains no prebuilt
binaries.

## Install

Add the repository marketplace at the `release` branch, then install
`megapowers@megapowers`. The complete commands, fresh-session checks, and
diagnosis steps are in
[the installation guide](https://github.com/lawzava/megapowers/blob/main/docs/install.md).

## Security

The plugin supplies workflow guidance, with no command guard or gates. Use
the harness permission system and OS sandbox for enforcement. The
independent-review skill sends only an explicit file or immutable commit range,
after a secret scan. It sends without asking only to a reviewer you configured
in your capability registry or instructions; otherwise it asks first. Read the
[security policy](https://github.com/lawzava/megapowers/blob/main/SECURITY.md)
before enabling external review.

The guidance-only doctor skill asks the agent to use normal file tools to read
the plugin manifest version, optional `skills/catalog.json`, each listed skill's
`SKILL.md`, and the shipped agent registry template.

## Attribution

The workflow descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent, used under
the MIT License. See the
[full attribution](https://github.com/lawzava/megapowers/blob/main/ATTRIBUTION.md).
