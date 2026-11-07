# Task fences and acceptance checks

Keep the existing task ID, owned files, prerequisites, consumes/produces
contracts, verification commands and disjoint ownership. Add only:

- **Fence:** repo-relative exact files, `dir/` prefixes or `*` patterns, using
  the [fence syntax](../../orchestrating/references/fences.md). Define ownership
  before dispatch, not after seeing the worker's patch.
- **Out-of-scope:** the changes this task must not make, even inside its fence.
- **Acceptance:** one exact repo-relative command per acceptance item, run from
  the repository root, with the expected observable result. Do not replace an
  unconfirmed item with a stub, TODO or a weaker test.

Before handing off, check these smells:

- An acceptance item joining more than two "and"s needs splitting.
- Overlapping fences in one parallel wave need one owner or sequential work.
- Uncheckable acceptance needs an executable oracle before implementation.
- A task that cannot merge alone and stay green needs a vertical slice or an
  explicit expand/migrate/contract prerequisite.
- A consumes contract with no producer needs an existing source or a producer
  task, not an assumption that another worker will supply it.
