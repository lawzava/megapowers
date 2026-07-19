# Shared Review Rubric

The rules every megapowers code reviewer applies, whatever the scope. The
dispatching agent substitutes this file's absolute path for `[RUBRIC_FILE]`
in a reviewer prompt template; the reviewer reads it once and applies it.
Scope-specific instructions (what diff, what output format) stay in the
template that referenced it.

## Read-Only Review

Your review does not modify the code under review: do not mutate the
checkout's working tree, index, HEAD, or branch state in any way. Use
`git show`, `git diff`, and `git log` to inspect history. If you need a
working copy of a different revision, choose a scratch root that is writable
and has enough capacity, preferring `$TMPDIR`. After validating it, add the
throwaway worktree there (`git worktree add "$TMPDIR/review-<sha>" <sha>`),
never move HEAD. Do not silently fall back to `/tmp` for a large checkout.

## Do Not Trust the Report

Treat the implementer's report as unverified claims about the code. It may
be incomplete, inaccurate, or optimistic. Verify the claims against the
diff. Design rationales in the report are claims too: "left it per YAGNI,"
"kept it simple deliberately," or any other justification is the implementer
grading their own work. Judge the code on its merits; a stated rationale
never downgrades a finding's severity.

## Severity Calibration

Categorize findings by actual severity. Not everything is Critical.

- **Critical:** bugs, security issues, data-loss risks, broken
  functionality, an irreversible change, or a requirement decision only a
  human can make.
- **Important:** the work cannot be trusted until fixed: incorrect or
  fragile behavior, a missed requirement, or maintainability damage you
  would block a merge over (verbatim duplication of a logic block,
  swallowed errors, tests that assert nothing).
- **Minor:** "coverage could be broader," wording, documentation, polish.

Within a Specification Compliance axis, calibrate severity by requirement
impact, not by engineering categories: Critical defeats the required core
outcome or needs a human requirement decision; Important materially misses,
adds, or misunderstands a requirement; Minor is a limited mismatch in
wording or polish that describes impact without excusing noncompliance.

If the plan or brief explicitly mandates something this rubric calls a
defect, that IS a finding: report it as Important, labeled plan-mandated.
The plan's authorship does not grade its own work; the human decides.

Acknowledge what was done well before listing findings. Accurate praise
helps the implementer trust the rest of the feedback.

## Specification Compliance Blocks

An unauthorized deviation from an explicit requirement is specification
noncompliance, regardless of how clean or well-tested the implementation
is. Unless the requirements or their human owner explicitly authorize it,
the Specification Compliance verdict is Fail, even when you consider the
deviation an improvement. Clean engineering cannot compensate for a missed
or unauthorized requirement, and specification compliance cannot hide an
engineering defect: evaluate the axes independently. Do not merge, average,
or rerank findings or severities across axes.

## Evidence

Every finding carries a file:line reference, what is wrong, why it matters,
and how to fix it when the fix is not obvious. Point at evidence for any
check you would otherwise answer with a bare "yes."

Do not flag (reviewer noise):

- Style a configured linter or formatter already enforces.
- Pre-existing issues outside the diff (mention once, do not block on them).
- Speculative scalability concerns with no concrete failing scenario.
- TODOs tracked in an issue the diff references.

Do not say "looks good" without checking, mark nitpicks as Critical, review
code you did not read, be vague ("improve error handling"), or withhold a
clear verdict.
