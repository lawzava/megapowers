# recursive-multi-writer-contract

Artifact oracle for recursive SDD ownership, durable shared-run state, bounded
worktrees, lifecycle cleanup, and the Claude Code and Codex coordinator rules.
`solve.sh` also runs both shipped SDD registry and worktree script suites before
`check.sh` accepts the marker set.

## Formal guidance policy

The coordinator prompt contains one visible, versioned
`megapowers-recursive-sdd-policy:v1` block. That block is the machine-readable
oracle for exact writer-token release, forbidden agent teams, and the maximum
five task-name components beneath `/root`. The adjacent prose implements the
formal policy and remains covered by exact positive assertions.

Earlier versions attempted to infer contradictory policy from arbitrary
English. Sentence parsing produced both false positives and false negatives,
so it was abandoned. `guidance-policy.awk` now validates only the formal block:
one start and end marker, exactly the three known fields once, and their exact
version-one values. Text outside the block does not change parsed values.

The tracked `guidance-policy.test.sh` fixture covers the valid block, every
wrong value, every missing or duplicate required field, unknown fields,
duplicate blocks, missing markers, nesting, unterminated input, and ignored
outside content. Both `solve.sh` and `scripts/validate.sh` run that same fixture
test and validate the prompt with the same POSIX `awk` parser.

Run the focused fixture directly:

```bash
bash evals/scenarios/recursive-multi-writer-contract/guidance-policy.test.sh
```

Run the full artifact scenario:

```bash
evals/run.sh recursive-multi-writer-contract
```
