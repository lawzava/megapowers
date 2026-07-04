#!/usr/bin/env bash
# Regression for auto-format.sh: it must not spawn npx (node startup is ~0.6-2s even
# when it resolves nothing) on a doc/config edit when no prettier is installed. The
# guarded hook walks up from the edited file for node_modules/.bin/prettier, else
# command -v prettier, and only then invokes prettier directly. We put a fake `npx`
# earlier on PATH that writes a sentinel; after the hook runs on a .md file with no
# prettier up-tree, the sentinel must NOT exist and the hook must still exit 0.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../auto-format.sh"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
sentinel="$work/npx-was-called"
{
  printf '#!/usr/bin/env bash\n'
  printf 'touch "%s"\n' "$sentinel"
  printf 'exit 0\n'
} > "$work/bin/npx"
chmod +x "$work/bin/npx"

md="$work/doc.md"
printf '# title\n\ntext\n' > "$md"

pass=0; fail=0
rc=0
printf '{"tool_input":{"file_path":"%s"}}' "$md" | PATH="$work/bin:$PATH" bash "$HOOK" >/dev/null 2>&1 || rc=$?

if [ ! -e "$sentinel" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL npx was spawned (sentinel exists)\n'; fi
if [ "$rc" -eq 0 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL hook exit code %s, want 0\n' "$rc"; fi

echo "== auto-format: $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
