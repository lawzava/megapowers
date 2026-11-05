# Global instructions

A global instruction file holds personal defaults that should apply in every
repository on one machine: how the agent should act on authority, report
work, handle Git and scratch data, and respect machine limits. Project
commands, stack conventions, and team rules belong in repository files.
Procedures belong in skills. Hard blocks belong in permission settings or
hooks.

## Locations and loading

| Harness | Global file | Loads with project files |
|---|---|---|
| Claude Code | `~/.claude/CLAUDE.md`, plus `~/.claude/rules/*.md` | Concatenated before project files; also loads alongside a project `AGENTS.md` |
| Codex | `~/.codex/AGENTS.md`, or `AGENTS.override.md` there when non-empty; `CODEX_HOME` moves the directory | Concatenated first; later project files take precedence |

Claude Code does not resolve a conflict between user and project text: it may
follow either. Codex gives the later project text precedence, but only by
position. In both, state once that repository instructions and explicit user
requests override the global defaults, and keep the global file free of
project-specific rules that could conflict.

Codex stops adding instruction files once the combined size reaches
`project_doc_max_bytes`, 32 KiB by default, and the global file is counted
first. A long global file can truncate project guidance. Claude Code
recommends under 200 lines per `CLAUDE.md` file and loads every import at
launch, so splitting into imports or `~/.claude/rules/` organizes text without
saving context. Keep a global file under 150 lines and 8 KiB.

## Content

Keep a line only when it changes behavior in most repositories and the agent
cannot infer it from the repository at the point of use. Good candidates:

- the authority model: what proceeds without asking and which effects need
  confirmation;
- reporting rules: lead with the result, cite evidence, report failures and
  skipped steps as such;
- Git habits: when to commit, message format, history rules;
- scratch and cache locations, and why the defaults are wrong on this machine;
- environment boundaries, such as which hosts are production;
- recurring failures with a concrete trigger, such as a sandbox limit that
  makes an agent stop instead of requesting escalation.

Leave out language style guides, generic quality advice, directory maps,
restatements of installed skills or the output style, and rules the harness
already enforces. Put an invariant that must hold in `permissions.deny`, a
hook, or a sandbox setting, and keep at most one line explaining it.

Mark machine-specific facts as such. When a template is shared publicly,
leave those lines as placeholders.

## Wording for current models

Claude Opus 5.5 and GPT-6 Astra follow instruction files closely, so wording
written to push older models now overshoots:

- State the precedence among the user's request, repository instructions,
  the global file, and skills. Astra can stop early on unclear or conflicting
  guidance.
- Remove contradictions with the harness's built-in prompt. Codex's Astra
  prompt already covers autonomy, persistence, and progress updates, and says
  not to write tests for reversible, low-impact changes, so a test rule must
  state its own scope.
- Define done: implemented, run, inspected, and failures fixed. Name the
  early stops to avoid and the stops you want, and ask for status notes in
  the same message as the next action. Do not add a stop-for-review gate or a
  hard stop after N attempts; both pull the model toward stopping.
- For long work, ask for the task list in a file outside the working tree.
  Compaction summarizes older turns; the file keeps what is done and what is
  left.
- Tell the model to ask only when readings would lead to materially different
  work, and to flag a better approach in one sentence without changing scope.
- Grant safe local workflows, such as tests and builds, explicitly. Prepare
  outward effects so approval is the final step.
- Write plain, positive instructions with a reason where the reason is not
  obvious. Avoid capitalized emphasis, re-check or double-check orders, and
  requests to show reasoning. Emphasize a single line only after it is
  repeatedly ignored.
- Leave out unattended-run continuation text and time budgets. Anthropic
  scopes the first to runs without a human, and says time pressure can reduce
  verification.
- Claude Code delivers `CLAUDE.md` as a user message after the system prompt,
  so it adds to the preset instead of replacing it.

## Templates

Start from [global-CLAUDE.md](../assets/global-CLAUDE.md) or
[global-AGENTS.md](../assets/global-AGENTS.md). Each is a complete baseline
for its harness, written to work with Megapowers installed. Copy it to the
global path, delete sections that do not match how you work, and fill in the
machine section. Keep the `megapowers-global-baseline` comment so a later
revision can be compared with the version you adopted. Claude Code strips
HTML comments before injecting the file; Codex keeps them, so remove guidance
comments after you edit.

## Validate

Start a fresh session outside any repository and ask the agent to list its
active instructions. For Claude Code, also check `/context` or `/memory`. For
Codex, run `codex --ask-for-approval never "Summarize the current
instructions."`. Then repeat inside a repository with its own instructions,
and confirm the project text appears after the global text and that no rule
conflicts. After a material change, run one representative task with the old
and new file.

## Sources reviewed 2026-09-27

- [Anthropic, How Claude remembers your project](https://code.claude.com/docs/en/memory):
  user-scope `CLAUDE.md` and `~/.claude/rules/`, load order, conflicts, size
  target, imports, HTML comment stripping, and `AGENTS.md` loading.
- [OpenAI, Custom instructions with AGENTS.md](https://developers.openai.com/codex/guides/agents-md):
  global scope under `CODEX_HOME`, `AGENTS.override.md`, merge order, and
  `project_doc_max_bytes`.
- [Anthropic, Prompting Claude Opus 5.5](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5-5)
  and [Prompting Claude Opus 5](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5):
  named early stops, gathering context before acting, time budgets, scope
  discipline, and over-verification.
- [Anthropic, Getting the most out of Opus 5.5](https://claude.dev/blog/getting-the-most-out-of-opus-5-5/),
  reviewed 2026-10-03: status notes with the next action, a task list in a
  file for long runs, and the user's open decisions first in the final report.
- [Anthropic, Best practices for Claude Code](https://code.claude.com/docs/en/best-practices):
  pruning, single-line emphasis, and evidence instead of asserted success.
- [OpenAI, Using GPT-6](https://developers.openai.com/api/docs/guides/latest-model)
  and [Rethinking skills and prompts for GPT-6 Astra](https://developers.openai.com/blog/rethinking-skills-and-prompts-for-gpt-6-astra):
  conflicting guidance, inherited boundaries, completion, testing, safe
  workflow grants, and delegation.
- Codex 0.157.1 built-in instructions for `gpt-6-astra` and `gpt-6-sol`, read
  from the local model catalog cache on 2026-09-27. This is observed runtime
  text, not published documentation, and can change without notice.
