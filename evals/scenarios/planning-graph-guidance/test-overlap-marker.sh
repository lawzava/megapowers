#!/usr/bin/env bash
set -euo pipefail

repo_root="${ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
solve="$repo_root/evals/scenarios/planning-graph-guidance/solve.sh"
skills="$repo_root/plugins/megapowers/skills"
tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/planning-overlap-probes.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "$tmpdir/root/plugins/megapowers/skills/writing-plans" \
  "$tmpdir/root/plugins/megapowers/skills/systematic-debugging" \
  "$tmpdir/root/plugins/megapowers/skills/project-memory" \
  "$tmpdir/work"
cp "$skills/systematic-debugging/SKILL.md" \
  "$tmpdir/root/plugins/megapowers/skills/systematic-debugging/SKILL.md"
cp "$skills/project-memory/SKILL.md" \
  "$tmpdir/root/plugins/megapowers/skills/project-memory/SKILL.md"

failures=0

probe() {
  name="$1"
  expected="$2"
  plan="$tmpdir/root/plugins/megapowers/skills/writing-plans/SKILL.md"
  cat > "$plan"

  (
    cd "$tmpdir/work"
    ROOT="$tmpdir/root" bash "$solve" >/dev/null
  )

  if ! grep -qx "$expected overlap-forces-sequential" "$tmpdir/work/out.txt"; then
    actual="$(grep 'overlap-forces-sequential' "$tmpdir/work/out.txt")"
    printf 'FAIL %s: expected "%s overlap-forces-sequential", got "%s"\n' \
      "$name" "$expected" "$actual"
    failures=$((failures + 1))
  fi
}

probe adjacent-sequential-does-not-hide-invalid MISSING <<'EOF'
1. `Parallel safety` is `Sequential`.
2. `Ownership` overlaps an active task but does not constrain concurrency.
EOF

probe shipped-required-rule OK < "$skills/writing-plans/SKILL.md"

probe stronger-prohibition OK <<'EOF'
Ownership overlap is never allowed.
EOF

probe permissive-overlap MISSING <<'EOF'
Ownership overlap is allowed.
EOF

probe contradictory-permissive-same-line MISSING <<'EOF'
Ownership overlaps an active task. Concurrent execution is allowed.
EOF

probe contradictory-permissive-separate-line MISSING <<'EOF'
Ownership overlaps an active task.
Ownership overlap is allowed for concurrent tasks.
EOF

[ "$failures" -eq 0 ] || exit 1
echo "ok: overlap marker precision probes passed"
