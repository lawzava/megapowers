# Runtime adapter: OpenCode

Last reviewed: 2026-08-08.

OpenCode is a runtime adapter, not a model vendor. Pass `--caller-adapter
opencode` with the actual model identity when it is catalogued. Do not report an
OpenCode session as Codex merely because it is using an OpenAI model.

OpenCode can use markdown agents with model overrides. This repository ships
portable skill markdown only: it does not ship an OpenCode plugin, launch
wrapper, or credentials contract. Configure OpenCode's own agents and
permissions before treating a resolved route as runnable.
