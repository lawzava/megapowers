# Run Receipt

Run each check directly with the harness's normal command tools. Save full
combined output in a private disk-backed temporary location. Preserve the child
exit status before filtering the log. Never infer success from a pipeline's tail.

Record the exact command, working directory, duration, real exit status, commit,
and whether the checkout contains uncommitted changes. Include untracked files
when identifying the tested artifact. Keep the log path with the result. Read
the full output and report failures and skipped checks separately.

Reuse evidence only for the same command, directory, commit and unchanged tree,
including untracked contents. Do not reuse it when network, environment or files
outside the repository can change the result.

Block on a running check using the harness wait tool. Do not wait on guessed
sleep durations. For CI, use a blocking check watcher. Start independent work
before blocking. Keep logs local because checks can print secrets. Cite the
result and artifact identity without publishing raw logs.
