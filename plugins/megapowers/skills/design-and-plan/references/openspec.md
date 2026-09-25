# OpenSpec conventions

Load this only when the repository contains an `openspec/` directory. A prompt
that mentions OpenSpec does not establish a repository convention; detect the
baseline (`openspec/specs/`) and any active change (`openspec/changes/<name>/`)
from the files and repository instructions.

Read whichever artifacts exist and preserve their requirement IDs and format.
Describe behavior as Added, Modified, or Removed deltas against the baseline.
After verification, reconcile deltas into the baseline. Archive a change only
under an existing convention and authority.

Do not run or add an OpenSpec CLI or Node dependency. Use plain Markdown in the
existing layout.
