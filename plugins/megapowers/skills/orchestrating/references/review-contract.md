# Reviewer return and lead decision

Require this return from each reviewer, native or external:

```text
verdict: pass | fail
scope.creep: [] # unrequested changes, drive-by refactors, new dependencies
scope.shrink: [] # stubs, TODOs, weakened/skipped/deleted tests,
                 # loosened assertions, unconfirmed acceptance items
findings:
  - severity: blocker | major | nit
    file: repo-relative path
    line: line number
    claim: defect and failing input or sequence
    evidence: observation, test, or artifact that supports the claim
```

Use explicit empty lists and state verification limits. The lead computes fail
when any blocker, major, scope.creep or scope.shrink exists, regardless of the
reviewer's stated verdict. Nits never fail. Missing fields, truncated returns
and unavailable reviews are gaps, not passes. Check findings yourself; record
concrete disproof for false positives and fix real failures before approval.

Set the correction-round cap before dispatch and retain it across fresh agents.
A spent budget leaves unresolved findings open for the user. Start a fresh
agent per correction round with the original brief plus all later directives;
review only fixes, affected boundaries and new evidence. See
[correction rounds](../../independent-review/references/correction-rounds.md).
