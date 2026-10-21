# Bar Guard

Run before finishing:

```bash
go run <skill-dir>/scripts/bar-guard.go
```

The default base is `HEAD`. Index and working-tree changes are checked
independently, including untracked files; identical findings appear once.
`--base origin/main` uses the merge base. `--diff FILE` reads a unified patch;
truncated patches fail. `--json` emits an array of findings.

Findings include path, line, kind, and up to 120 source characters.
Checks cover skips, suppressions, removed tests/assertions, emptied error
handlers, and loosened thresholds. Test moves retaining the same name stay
clean. Numeric comparisons require unique scalar keys in one hunk with matching
units; unparseable numbers report `threshold-changed` as unverifiable.

Exit codes are 0 clean, 1 findings, 2 unable to run. `go run` maps either
failure to 1; build the file when exact exit codes matter.

A same-line `bar-guard:allow <reason>` remains visible as waived and permits
exit 0. Fix each finding or justify it to the user.
