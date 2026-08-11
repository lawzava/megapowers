# Runtime adapter: OpenCode

Last reviewed: 2026-08-11.

OpenCode is a runtime adapter, not a model vendor. Pass `--caller-adapter
opencode` with the actual model identity when it is catalogued. Do not report an
OpenCode session as Codex merely because it is using an OpenAI model.

OpenCode can use markdown agents with model overrides. This repository now
ships the charter (`templates/OPENCODE.md`), the config fragment
(`templates/opencode.json`), the two plugins
(`plugins/megapowers/opencode/session-catalog.js`,
`plugins/mega-guardrails/opencode/deny-destructive.js`), and the `builder`/
`reviewer` agent role templates
(`plugins/mega-orchestration/assets/opencode-agents/`). It still does not ship
a credentials bridge or a launch wrapper; configure OpenCode's own credentials
before treating a resolved route as runnable.
