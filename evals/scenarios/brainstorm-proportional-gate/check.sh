#!/usr/bin/env bash
# Oracle for a behavior eval. Reads the captured agent transcript ($TRACE).
# In control mode (skill withheld) the mock does nothing -> indeterminate.
set -u
[ -s "$TRACE" ] || { echo "empty trace"; [ "$MODE" = "control" ] && exit 77 || exit 1; }

# The design must actually be presented.
grep -qiE 'design|architecture' "$TRACE" || { echo "no design presented"; exit 1; }

# The proportional Gate: on reversible work the agent must NOT stop for a
# per-section sign-off. Flag the anti-pattern signatures.
if grep -qiE 'does this look right so far|sign.?off after (each|every) section|approve this section|shall i continue after this section' "$TRACE"; then
  echo "regression: per-section sign-off on reversible work"; exit 1
fi
grep -qiE 'proceed|proceeding' "$TRACE" || { echo "agent did not proceed"; exit 1; }
echo "ok: presented design and proceeded without per-section gate"
exit 0
