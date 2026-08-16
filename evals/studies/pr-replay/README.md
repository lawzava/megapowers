# PR replay

PR replay is an optional, report-only foundation for measuring whether an
installed plugin helps an actor independently solve a historical task. It does
not gate releases. Add cases locally or in a reviewed benchmark manifest:

```json
{
  "schema_version": "1",
  "cases": [{
    "id": "issue-123",
    "enabled": true,
    "repository_id": "owner/repository",
    "repository": "/private/path/to/a-local-mirror",
    "base": "0123456789abcdef0123456789abcdef01234567",
    "head": "89abcdef0123456789abcdef0123456789abcdef",
    "task": "The original issue text, without the solution.",
    "oracle": {
      "files": ["internal/feature/hidden_test.go"],
      "command": ["go", "test", "./internal/feature"]
    }
  }]
}
```

`base` and `head` must be full immutable commit IDs. The runner exports only
the base tree into a new repository. The actor receives no source remote, head
object, gold diff, head ID, or oracle files. After the actor exits, the runner
adds only the declared oracle files from `head` and runs the correctness oracle.
The oracle must fail against the untouched base or the case is rejected.

Credential-free contracts:

```bash
go run evals/studies/pr-replay/replay.go --selftest
bash evals/studies/tests/pr-replay-contract.test.sh
```

Explicit credentialed run:

```bash
go run evals/studies/pr-replay/replay.go --run --credentialed \
  --harness claude --model claude-fable-5 --effort high \
  --sandbox-broker /usr/local/libexec/megapowers-eval-broker \
  --broker-sha256 "$BROKER_SHA256" \
  --cases private-replays.json --out results/pr-replay
```

Real actors run only through a user-reviewed, hash-pinned isolation broker. The
broker owns provider authentication outside the actor-visible filesystem. It
must attest a real OS boundary with task read access limited to the exported
base project and `plugins/megapowers`. The source mirror, head commit, oracle
overlay, sibling directories, and credentials must be unreadable to the actor.
Only the exported base project may be writable. Any missing or mismatched
attestation fails closed. Direct Claude or Codex
execution is intentionally unsupported.

The broker reads one versioned JSON request from standard input and returns one
versioned JSON object containing CLI version, result, exact plugin inventory,
exit code, duration, and an isolation attestation. The attestation explicitly
sets credential and sibling-state readability to false and repeats the exact
task read and write roots. Omitted fields, extra JSON, unrecognized boundaries,
or inventory mismatches fail.

Touched-file overlap with the historical patch is recorded as a diagnostic.
It never changes the verdict. Only the declared correctness oracle does. The
publish bundle contains schema rows and a sanitized manifest, never repository
paths, prompts, responses, transcripts, credentials, or source trees.
