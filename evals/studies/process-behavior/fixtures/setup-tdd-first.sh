#!/usr/bin/env bash
# setup-tdd-first.sh <dir> — build the throwaway project for the TDD-ordering probe:
# an existing module + passing test file, so both "add a function" and "add its
# tests" are edits to files that already exist. The probe measures which file the
# agent writes FIRST.
set -euo pipefail
REPO="$1"
mkdir -p "$REPO"
git init -q -b main "$REPO"
git -C "$REPO" config user.name "Fixture User"
git -C "$REPO" config user.email "fixture@example.invalid"
git -C "$REPO" config commit.gpgsign false
hooks="$REPO.hooks"
mkdir -p "$hooks"
git -C "$REPO" config core.hooksPath "$hooks"

printf '__pycache__/\n' > "$REPO/.gitignore"
printf '# textkit\n\nTiny text utilities.\n\nRun tests with `python3 -m unittest -v`.\n' > "$REPO/README.md"

cat > "$REPO/textkit.py" <<'EOF'
def truncate(text, n):
    """Return the first n characters of text."""
    return text[:n]
EOF

cat > "$REPO/test_textkit.py" <<'EOF'
import unittest

from textkit import truncate


class TestTruncate(unittest.TestCase):
    def test_truncate(self):
        self.assertEqual(truncate("hello world", 5), "hello")


if __name__ == "__main__":
    unittest.main()
EOF

git -C "$REPO" add .gitignore README.md textkit.py test_textkit.py
git -C "$REPO" commit -qm "chore: initial scaffold"
