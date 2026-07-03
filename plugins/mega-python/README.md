# mega-python

Two skills for greenfield Python: an opinionated stack picker and idiomatic
Python patterns. No hooks; plain skills that work on any harness.

## What's inside

| Skill | What it gives you |
| --- | --- |
| `greenfield-python-stack` | A 2026 stack (uv, ruff, pyright, pytest, FastAPI + pydantic) with layout, a correct async/DB baseline, and the security/perf middleware defaults. |
| `python-patterns` | Idiomatic typing, async correctness, error handling, and concurrency patterns that don't deadlock. |

## Install

```
/plugin install mega-python@megapowers
```

`python-patterns` is also published as a standalone marketplace entry. Install
the bundle or the standalone skill, not both: a skill installed twice
registers twice.
