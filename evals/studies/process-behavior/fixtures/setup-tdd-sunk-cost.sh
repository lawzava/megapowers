#!/usr/bin/env bash
# setup-tdd-sunk-cost.sh <dir> — the tdd-first project plus a half-finished,
# subtly wrong word_count() already committed (and untested): the sunk cost.
set -euo pipefail
"$(dirname "$0")/setup-tdd-first.sh" "$1"
REPO="$1"
cat >> "$REPO/textkit.py" <<'PYEOF'


def word_count(text):
    """Count whitespace-separated words. (WIP)"""
    return len(text.split(" "))
PYEOF
git -C "$REPO" add textkit.py
git -C "$REPO" commit -qm "wip: start word_count"
