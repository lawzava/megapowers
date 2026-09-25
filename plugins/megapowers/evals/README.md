# Native Claude smoke evaluations

These five cases exercise tool-free formatting, launcher-error attribution, a
simulated native goal contract under a stale-note conflict, and skill
activation on a completion request and a deploy request. They use
deterministic graders and read-only tools. They do not test actual native goal
recovery, external effects, or general task quality.

With Claude Code 2.1.269 or newer, use the current authenticated launch route:

```bash
claude plugin eval plugins/megapowers --trust-plugin \
  --model MODEL --runs 1 --concurrency 1 --max-cost-usd 3 --no-publish \
  --output-dir "$TMPDIR/megapowers-native-eval"
```

Replace `MODEL` with the intended available model. Set effort through the
harness's supported configuration and record it with the result. If the operator
uses a credential router, wrap the whole command through that approved route.
Resolve `TMPDIR` to disk-backed scratch before running. Use a fresh output
directory for each run. No real MCP server or write-tool grant is needed.

The command above overrides each case's default with one run per arm, making
ten actor calls. The case files default to three runs per arm, making thirty
calls; use `--runs 3` explicitly to confirm a useful result.
The cost limit is a list-price estimate checked before
each run, so one in-flight call can exceed it. Keep reports local. Preserve run
errors and partial results separately from behavioral failures. Positive Skill
activation graders are diagnostic in paired mode. The title case's zero-Skill
requirement is an outcome constraint and explicitly scores both arms. Single-arm
mode scores activation too; compare outcomes separately across different modes.

The resume case checks obedience to a supplied native contract when a stale note
conflicts. It cannot establish recovery effectiveness or incremental plugin value.

The commit-and-pr case checks that a "commit this and open a PR" request
activates `verify-and-finish` before the actor reports completion. The
deploy-request case checks that a production deploy request activates
`safe-effects` before the actor claims authorization. Neither executes a
command or makes an external call; both are dry runs of the actor's judgment.

Run identical cases against an immutable prior plugin snapshot to compare an
instruction change. A tiny pilot can expose a regression; passing it cannot
establish safety or a useful cost reduction.

Format and isolation reference: [Claude plugin evals](https://code.claude.com/docs/en/plugin-evals).
