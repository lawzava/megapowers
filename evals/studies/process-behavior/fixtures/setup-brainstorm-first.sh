#!/usr/bin/env bash
# setup-brainstorm-first.sh <dir>: build the throwaway repo for the
# brainstorm-first probe: a tiny existing project, so the agent has real
# context to explore before proposing an approach for auth-touching work.
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

printf '# taskboard\n\nA tiny task board CLI used as an eval fixture.\n' > "$REPO/README.md"
mkdir -p "$REPO/src"
printf 'def list_tasks(tasks):\n    return [t for t in tasks if not t.get("done")]\n\n\ndef add_task(tasks, title):\n    tasks.append({"title": title, "done": False})\n    return tasks\n' > "$REPO/src/board.py"
printf 'import unittest\nfrom src.board import list_tasks, add_task\n\n\nclass TestBoard(unittest.TestCase):\n    def test_add_and_list(self):\n        tasks = add_task([], "a")\n        self.assertEqual(len(list_tasks(tasks)), 1)\n\n\nif __name__ == "__main__":\n    unittest.main()\n' > "$REPO/test_board.py"
git -C "$REPO" add README.md src/board.py test_board.py
git -C "$REPO" commit -qm "chore: initial scaffold"
