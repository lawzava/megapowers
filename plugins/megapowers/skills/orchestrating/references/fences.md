# Writer fences and lead-leak checks

Use normal Git and file tools to compare each worker against the immutable
commit recorded at dispatch. Record the worker root and permitted repo-relative,
case-sensitive paths before writing. Match directory fences to descendants,
not similarly named siblings. State prohibited work separately in the brief.

Fence syntax uses repo-relative exact files, `dir/` prefixes or `*` patterns.
`*` may match `/`; all other characters are literal. Quote patterns, use `/`
separators, and reject empty or absolute paths, backslashes, and `.` or `..`
path components.

Compare the union of committed, staged, unstaged and non-ignored untracked
changes. Check both rename endpoints. A revision comparison covers only its
tree; inspect a working checkout for uncommitted changes too. Reject ambiguous
revision or directory names. Quote unusual paths in findings.

Before dispatch, capture the lead HEAD, index, worktree and untracked contents
in a private baseline outside the checkout. Compare again before integration.
Existing user edits belong to that baseline. Account for the lead's own edits
before attributing drift to workers. Never discard user edits to clear a check.

Any fence violation or unexplained lead drift blocks integration. Reconcile a
needed scope change with the lead and acceptance boundary before widening a
fence. These checks do not provide an OS sandbox or detect edits restored
before inspection. Check submodule repositories separately. If Git or isolation
is unavailable, inspect paths manually and keep writing sequential.
