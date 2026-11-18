# Large Diffs

A reviewer can drop part of a large diff without an error. State what each
review covered: the artifact identity and the files or parts the reviewer
read.

When a diff is too large for one reviewer pass to cover, split it into parts
that can each be reviewed and reverted on their own. Order the hard-to-reverse
part (migration, data semantics, external effect) last, after the parts it
depends on are reviewed, and always review it.

Report any part that was not reviewed. An unreviewed part is a gap, never a
pass.
