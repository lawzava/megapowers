# Repository instructions

Use repository instructions for facts and rules that should be present across
ordinary work in a repository or one of its subtrees. Put a task-specific
procedure in a skill and a mandatory control in deterministic tooling.

## Choose scope and source of truth

Place broad shared guidance at the repository root. Place package or service
guidance beside the narrowest subtree it governs. A nested file should contain
only the delta from its ancestors. Before adding one, confirm that the target
harness discovers it from the session's working directory and file access
pattern.

Codex builds its project instruction chain from the repository root down to the
current working directory when a run starts. It reads at most one applicable
instruction file in each directory, and guidance closer to the working
directory appears later and takes precedence.

Claude Code loads `CLAUDE.md` files above the working directory at launch and
loads nested files when it reads within their subtrees. It does not treat
`AGENTS.md` as its native project instruction file. When both harnesses need the
same project guidance, keep `AGENTS.md` as the shared source and make
`CLAUDE.md` import it with `@AGENTS.md`; add Claude-specific content only when
behavior truly differs. An import organizes content but does not reduce the
amount loaded into context.

Do not mirror the same rules across root and nested files. Do not create files
for harnesses the project does not support.

## Write from observed evidence

Inspect the commands, configuration, code, tests, and nearest instruction files.
Keep facts an agent cannot safely infer at the point of use, such as:

- the exact focused and canonical verification commands;
- ownership or directory boundaries that prevent cross-service changes;
- non-obvious architecture constraints and their practical consequence;
- environment distinctions that change what operations are safe;
- recurring failure modes backed by an observed example.

State the actor, condition, scope, and expected result. Use exact identifiers
and commands. Avoid vague quality demands, generated directory inventories,
generic language advice, and explanations copied from upstream manuals. Remove
stale or conflicting guidance instead of adding another precedence rule.

Use `always` or `never` only for an actual invariant. If violating a rule must
be impossible, implement the check in a hook, permission setting, linter, test,
or other deterministic boundary and keep the instruction as a short explanation.
Do not add routine approvals or pauses where repository policy and the user's
request already provide authority.

## Validate effective scope

For a new file or material scope or behavior change, start a fresh session in
the directory the instruction is meant to govern and ask the harness to report
the effective instructions. Verify broad guidance, the nested delta, and any
intended override. Also test a sibling directory where the nested rule must not
apply. For Claude Code, inspect `/context`; for Codex, use a fresh non-mutating
run. Treat discovery evidence as distinct from task behavior, then run a
representative task with and without the change. For a typo, link repair, or
small factual correction that does not change behavior, use proportional
format, link, and file-scope checks without a baseline model run.

## Sources reviewed 2026-09-05

- [OpenAI, Custom instructions with AGENTS.md](https://developers.openai.com/codex/guides/agents-md):
  startup discovery, root-to-working-directory order, nearest-scope precedence,
  and the combined project-instruction size limit.
- [Anthropic, How Claude remembers your project](https://code.claude.com/docs/en/memory):
  `CLAUDE.md` scope, nested loading, imports, instruction concision, and the
  recommended `@AGENTS.md` bridge for repositories shared with other agents.
- [Anthropic, Extend Claude with skills](https://code.claude.com/docs/en/skills):
  project and plugin skill locations, automatic skill selection, and the
  distinction between reference content and invoked task workflows.

These links describe current client behavior. Verify them again when loader
behavior or supported harness versions change.
