# Learning loop

Use this reference when the user asks what Megapowers is doing for a project,
wants the agent to learn from past sessions, or asks to review, promote, or
retire lessons. The loop has five steps: observe, propose, prove, promote, and
retire. Each step ends with evidence the user can check. The agent never edits
an instruction file, memory, or check without the user's approval of that
change.

`SKILL_DIR` is this skill's directory.

## 1. Observe

```bash
go run "$SKILL_DIR/scripts/session-mine.go" scorecard --workspace "$PWD" --since 2026-10-01
```

The scorecard reads local transcripts and the project's
`.megapowers/run/*/journal.jsonl`. It reports:

- correction classes;
- each skill's loads, the sessions that loaded it, and the corrections that
  followed a load before the next human turn;
- megapowers gate blocks;
- `VERIFIED` and `NOT VERIFIED` reports;
- foreground `sleep` calls;
- per-run oracle, continuation, blocked, holdout, and high-risk decision
  counts.

The numbers are candidates. A correction after a skill load does not show that
the skill caused it, so read the sessions behind a number before acting on
it. Keyword corrections are sparse. In one 30-day sample, 5 of 137 human turns
matched a class, so outcome counts usually carry more signal.

## 2. Propose

Run a [retrospective](retrospective.md) over the scorecard and the
[session mining](session-mining.md) corrections. For each class seen at least
twice, choose the highest surface that works:

| Surface | Use when | Target |
|---|---|---|
| `check` | A test, lint, hook, or script can catch the mistake | The check's path or command |
| `instructions` | Only judgment prevents it in this project | The nearest `AGENTS.md`, `CLAUDE.md`, or skill reference |
| `memory` | It is a personal preference across projects | Native harness memory, audited with [memory hygiene](memory-hygiene.md) |
| `upstream` | Megapowers itself caused it in every project | A sanitized export for the maintainers |

Record each candidate with its evidence:

```bash
go run "$SKILL_DIR/scripts/lessons.go" add --id ID --class CLASS --lesson TEXT \
  --surface check --target PATH --evidence direct-observation \
  --source "session.jsonl 2026-10-10T09:02:00Z" [--source ...]
```

`--evidence` accepts only `direct-statement`, `direct-observation`,
`source-backed`, or `history-entry-only`. `--class` is a correction class,
`sleep`, or `gate:<skill>`, so review can count recurrences. The store
defaults to `$MEGAPOWERS_LESSONS_DIR`, else
`${XDG_STATE_HOME:-~/.local/state}/megapowers/lessons`. It must sit outside
the repository, like a holdout set, so implementing agents do not read the
lessons they are judged by. On a sandboxed harness, writing there needs the
user's approval or a configured `MEGAPOWERS_LESSONS_DIR`.

## 3. Prove

A check lesson must fail on the real past instance and pass now:

```bash
go run "$SKILL_DIR/scripts/lessons.go" prove --id ID --fail-ref <commit-with-the-mistake> -- <check command>
```

`prove` runs the check in a temporary detached worktree at the failing
commit, then in the current tree. It first copies check material into the old
tree: each `--check-file PATH`, and each relative file or directory named on
the command that did not exist at the old commit. A check added after the
mistake then judges the old code instead of failing because it is missing.
A path that existed at the old commit is the code under test and is never
replaced automatically. The overlay refuses to write through a symlink. It exits 3 when the check passes at the
failing commit (it cannot catch the mistake) or fails now. For an
`instructions`, `memory`, or `upstream` lesson, compare the same cases with and
without the change as the parent skill describes, then cite the result:
`prove --id ID --ab "treatment 5/5, control 1/5, <run reference>"`.

## 4. Promote

Write the approved change to its target first, then:

```bash
go run "$SKILL_DIR/scripts/lessons.go" promote --id ID [--anchor "exact text now in the target"]
```

`promote` requires a proof or at least two sources. It re-runs a check
lesson's command, and it reads `--anchor` back from an `instructions` or
`memory` target. A missing read-back exits 3. When a check makes a prose rule
redundant, delete the prose rule.

## 5. Review and retire

Run review monthly, or whenever the user asks what is working:

```bash
go run "$SKILL_DIR/scripts/session-mine.go" scorecard --workspace "$PWD" --since <date> --format json > "$TMPDIR/card.json"
go run "$SKILL_DIR/scripts/lessons.go" review --scorecard "$TMPDIR/card.json"
go run "$SKILL_DIR/scripts/lessons.go" retire --id ID --why TEXT
```

Review needs a scorecard for this project with a `--since` window. For each
promoted lesson it prints one of:

- `recurring`: the class reappeared after promotion, so the lesson is not
  working;
- `stale`: the class has been quiet for more than `max_age_days` (default
  90), so the lesson is a retirement candidate;
- `holding`: the class stayed quiet within its age limit;
- `unknown`: the window starts before the promotion, or the scorecard has no
  signal for the class.

An unpromoted lesson older than its age limit shows as `expired`. Review exits
3 when anything needs the user. Bring those lessons to the user. Retire one
only with their agreement.

## Upstream

What leaves the machine goes through two sanitized outputs:

```bash
go run "$SKILL_DIR/scripts/session-mine.go" scorecard --all-workspaces --sanitize --since <date> --format json
go run "$SKILL_DIR/scripts/lessons.go" export --id ID
```

`--sanitize` hashes workspace paths, drops run IDs, and folds every skill name
outside `megapowers:` into `other`. `export` drops the project path, sources,
and commit IDs, and replaces every check argument that contains `/` or `\`
with `<path>`. It omits the free-text lesson unless `--include-text` is
given, because free text can name a client in words no pattern catches; with
it, path tokens become `<path>` and the user reads the text before sharing.
It and adds an installed-ab case draft. Sanitizing keeps only
well-formed `megapowers:<skill>` names. Show both to the user
before sharing; sharing is an outward effect under `safe-effects`. A
maintainer who accepts a draft adds the case to
`evals/studies/installed-ab/cases.json` together with its `oracleProofs`
entry, and changes a skill only after the installed-ab comparison shows no
regression.
