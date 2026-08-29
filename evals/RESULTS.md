# megapowers eval results

## Current candidate truth

The current repository is one fifteen-skill plugin for Claude Code and Codex. Its
deterministic suite and runner selftests are engineering regressions only. They
do not establish that the plugin improves agent behavior.

No credentialed installed-plugin A/B result is published here for the current
fifteen-skill candidate. Installed A/B and PR replay are optional diagnostic
studies, not release gates. Exact-tag install smoke runs after publication and
proves delivery, not candidate quality.

Current protocols and gates:

- [deterministic and behavioral evals](./README.md)
- [installed-plugin A/B](./studies/installed-ab/README.md)
- [PR replay](./studies/pr-replay/README.md)
- [release and evidence sequence](../docs/advanced/evals.md)

## Historical record

Retained measurement history from earlier plugin and harness surfaces — not
evidence for the current candidate — lives in
[RESULTS-archive.md](./RESULTS-archive.md).
