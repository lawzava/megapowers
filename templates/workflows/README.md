# Workflow templates

These are reference Claude Code dynamic-workflow scripts (the `ultracode`
workflow runner) that codify two megapowers shapes: `best-of-n.js` (generate N
candidates, then select one by executable oracle first and blind judge second)
and `audit-fanout.js` (fan a lensed audit out across many targets, adversarially
verify each finding, then synthesize). They are templates, not installed
components: copy one into `.claude/workflows/` (project, shared via the repo) or
`~/.claude/workflows/` (personal) and it runs as `/best-of-n` or `/audit-fanout`.
A plugin cannot ship a workflow (the plugin component list has no workflows
entry), so the marketplace distributes these as files you copy in rather than
installing them for you.
