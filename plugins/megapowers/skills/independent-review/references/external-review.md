# External review adapter

Use this route only for a reviewer outside the harness. A native reviewer uses
the permissions and provider binding described in [the skill](../SKILL.md).

Take the exact command and real vendor family from the capability registry or
user instruction. Author and provider labels must differ. Set `review_tool`
to the installed `scripts/megapowers-review.go` beside `SKILL.md`, not a project
copy. The tool forwards no credentials. Reviewer CLIs sign in themselves; the
process gets only `PATH` and `HOME`.

For a pre-approved reviewer, inspect and dispatch in one step:

```bash
reviewer_cmd='<reviewer CLI and arguments>' # {prompt_file} and {scratch_dir} expand
intent='<intended behavior, non-goals, and acceptance boundary>'
go run "$review_tool" review --file path/to/file --intent "$intent" \
  --provider <reviewer-family> --provider-command "$reviewer_cmd" \
  --author <author-family> --approve-external configured
```

When human inspection is required or no configured reviewer exists, run
`inspect` first with `--file path/to/file` or `--base <base> --head <head>` and
the same intent, provider and command. Check disclosed paths, bytes, intent,
binary hash, command, and chunk package hashes. Pass its `approval_token` to
`review --approve-external "$approval_token"` only after approval. Any file,
intent, binary, or command change requires a new inspection and token.

`--file` accepts a repository file, a file in the current directory outside Git,
or a file under `$TMPDIR`. Keep packages and receipts under `$TMPDIR`, not in the
deliverable. `--out` must be an existing absolute directory outside the review
root. `--preflight` probes liveness before artifact bytes leave. Without Go or
a shell, send nothing through this route: the secret scan lives in the tool.

The tool checks secret-like paths and common secret patterns, rejects symlinks,
submodules, binary and oversized packages, and binds inspection to package,
command, binary and environment hashes. It is not an exhaustive secret detector
or a filesystem sandbox for the reviewer. Review the disclosure even when
standing approval permits automated dispatch. On failure, report the error;
a direct provider call skips these checks. Use `--retain-transcript` only after
a sensitive-data decision. Receipts are advisory, not approval gates.
