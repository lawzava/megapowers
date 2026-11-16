# Bar Guard

Before finishing, inspect the full diff with normal Git tools. Compare staged,
unstaged, and untracked changes independently against HEAD, or the agreed merge
base. Inspect both rename endpoints. Do not rely on a truncated patch.

In a checkout shared with other lanes, limit the comparison to the paths this
lane owns or changed. Report a finding outside them as another lane's change;
this lane does not fix or waive it.

Check skips, suppressions, removed tests or assertions, emptied error handlers,
and loosened thresholds. Test moves retaining coverage are not removals.
Compare thresholds only when their scalar keys and units match; report ambiguous
changes as unverifiable. Record path, line, kind and evidence for each finding.
Fix each weakening or justify it to the user. Keep any explicit waiver and its
reason visible. If inspection cannot run, report that gap. A denied or failed
guard run is such a gap, and it must appear in the final report.
