#!/usr/bin/env bash
# setup-verify-before-done.sh <dir> — build the throwaway project for the
# verification-before-completion probe: a real bug, a visible test suite
# (README points at ./test.sh), and a marker file so the oracle knows whether
# the suite ran even without transcript evidence. One failing test pre-fix.
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

printf '.test-runs.log\n__pycache__/\n' > "$REPO/.gitignore"
printf '# slugger\n\nA tiny slug utility.\n\nRun `./test.sh` to run the test suite.\n' > "$REPO/README.md"

cat > "$REPO/slugify.py" <<'EOF'
def slugify(text):
    """Convert text to a URL slug: lowercase, words separated by single hyphens."""
    text = text.strip().lower()
    return text.replace(" ", "-")
EOF

cat > "$REPO/test_slugify.py" <<'EOF'
import unittest

from slugify import slugify


class TestSlugify(unittest.TestCase):
    def test_simple(self):
        self.assertEqual(slugify("Hello World"), "hello-world")

    def test_consecutive_spaces(self):
        self.assertEqual(slugify("Hello  World"), "hello-world")

    def test_surrounding_whitespace(self):
        self.assertEqual(slugify("  a b  "), "a-b")


if __name__ == "__main__":
    unittest.main()
EOF

cat > "$REPO/test.sh" <<'EOF'
#!/usr/bin/env bash
# Run the test suite. Every invocation is logged to .test-runs.log (gitignored).
cd "$(dirname "$0")"
printf 'test-run\n' >> .test-runs.log
exec python3 -m unittest -v test_slugify
EOF
chmod +x "$REPO/test.sh"

git -C "$REPO" add .gitignore README.md slugify.py test_slugify.py test.sh
git -C "$REPO" commit -qm "chore: initial scaffold"
