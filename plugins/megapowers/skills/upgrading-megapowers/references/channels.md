# Current marketplace channels

Use the observed marketplace name. The commands below assume the standard
`megapowers` registration; replace that name only when inspection found a
different existing registration.

Read marketplace provenance separately from installed registration:

```bash
claude plugin marketplace list --json
claude plugin list --json

codex plugin marketplace list --json
codex plugin list --json
```

After the approved stable tag commit matches the observed marketplace head,
refresh and register the floating install:

```bash
claude plugin marketplace update megapowers
git -C <marketplace-install-location> rev-parse HEAD
claude plugin update megapowers@megapowers --scope <scope>

codex plugin marketplace upgrade megapowers --json
git -C <marketplace-install-location> rev-parse HEAD
codex plugin add megapowers@megapowers --json
```

After each marketplace refresh, require the reported `HEAD` to equal the
approved stable commit before running the following registration command. On a
mismatch, stop with the installed plugin untouched.

Claude's installed list supplies enabled state, version, scope, and
`installPath`; its marketplace list supplies the repository. Codex's installed
list supplies enabled state, version, and source. Retain `installedPath` from
the `codex plugin add megapowers@megapowers --json` result. Compare that exact
cache with the approved target ref.

Treat `.codex-marketplace-install.json` and `.in_use` as harness runtime
markers: exclude them from user-edit detection and target byte parity. The
recorded Codex `revision` may corroborate Git `HEAD`, but does not replace it.
