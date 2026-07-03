#!/usr/bin/env bash
# setup-project.sh <dir> [--bug] — seed the small project trigger tasks run in.
# A plain python mini-project: module, tests, ./test.sh, README. With --bug,
# plant a real defect (for the systematic-debugging task) so its suite fails.
set -euo pipefail
REPO="$1"; BUG="${2:-}"
mkdir -p "$REPO"
git init -q -b main "$REPO"
git -C "$REPO" config user.name "Fixture User"
git -C "$REPO" config user.email "fixture@example.invalid"
git -C "$REPO" config commit.gpgsign false
hooks="$REPO.hooks"; mkdir -p "$hooks"
git -C "$REPO" config core.hooksPath "$hooks"

printf '__pycache__/\n.test-runs.log\n' > "$REPO/.gitignore"
printf '# statkit\n\nTiny stats/text utilities.\n\nRun `./test.sh` to run the test suite.\n' > "$REPO/README.md"

if [ "$BUG" = "--bug" ]; then
  # mean() divides by len+1 — a real, findable defect
  cat > "$REPO/stats.py" <<'EOF'
def mean(nums):
    """Arithmetic mean of a non-empty list."""
    return sum(nums) / (len(nums) + 1)
EOF
else
  cat > "$REPO/stats.py" <<'EOF'
def mean(nums):
    """Arithmetic mean of a non-empty list."""
    return sum(nums) / len(nums)
EOF
fi

cat > "$REPO/test_stats.py" <<'EOF'
import unittest

from stats import mean


class TestMean(unittest.TestCase):
    def test_mean(self):
        self.assertEqual(mean([2, 4, 6]), 4)


if __name__ == "__main__":
    unittest.main()
EOF

cat > "$REPO/test.sh" <<'EOF'
#!/usr/bin/env bash
cd "$(dirname "$0")"
printf 'test-run\n' >> .test-runs.log
exec python3 -m unittest -v
EOF
chmod +x "$REPO/test.sh"

git -C "$REPO" add .gitignore README.md stats.py test_stats.py test.sh
git -C "$REPO" commit -qm "chore: initial scaffold"
