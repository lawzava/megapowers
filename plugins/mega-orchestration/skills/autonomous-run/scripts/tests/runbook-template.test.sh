#!/usr/bin/env bash
# Guards the runbook template: the MUST/SHOULD split is the contract a run follows,
# and both halves are easy to erode by accident. The counts below are pinned PER
# SECTION and exactly, because a total-only floor buys free deletions: dropping one
# Evidence MUST or relabelling an Authorization MUST as advisory leaves the total
# unchanged and passes. It also pins that run-init emits this exact text, since a
# second copy of the contract is what the template exists to remove.
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
SKILL="$(cd "$HERE/.." && pwd)"
TEMPLATE="$SKILL/references/runbook-template.md"
RUN_INIT="$HERE/run-init"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
passed=0
failed=0

pass() { echo "ok   $1"; passed=$((passed + 1)); }
fail() { echo "FAIL $1"; failed=$((failed + 1)); }

# The pinned shape. Every line is <section>=<numbered items>; a section that gains
# or loses a rule has to say so here, which is the whole point of the file.
expected_counts() {
  cat <<'EOF'
Sequence=1
Ownership and freeze=7
Working a milestone=5
Journal=5
Evidence=4
Authorization=7
Closing=5
SHOULD=15
EOF
}

# Count numbered items per section. Sections are the ### headings inside MUST plus
# the single SHOULD half; continuation lines are indented, so only a line starting
# at column 0 with "<n>." opens an item.
actual_counts() {
  awk '
    function flush() { if (sec != "") printf "%s=%d\n", sec, c }
    /^## MUST/   { flush(); half = "MUST";   sec = ""; c = 0; next }
    /^## SHOULD/ { flush(); half = "SHOULD"; sec = "SHOULD"; c = 0; next }
    /^### / && half == "MUST" { flush(); sec = substr($0, 5); c = 0; next }
    half != "" && /^[0-9]+\./ { c++ }
    END { flush() }
  ' "$1"
}

# Numbering has to be one contiguous run across both halves. A gap means an item
# was removed without anyone reading the section it came from.
numbering_ok() {
  awk '
    /^[0-9]+\./ { n = $0; sub(/\..*/, "", n); want++; if (n + 0 != want) { print "numbering breaks at " n " (expected " want ")"; bad = 1 } }
    END { exit bad ? 1 : 0 }
  ' "$1"
}

# The guard itself, run against any candidate file so the mutations below can be
# checked with the same code the real template is checked with.
check_template() {
  local file="$1"
  actual_counts "$file" > "$scratch/actual"
  expected_counts > "$scratch/expected"
  diff -u "$scratch/expected" "$scratch/actual" || return 1
  numbering_ok "$file" || return 1
}

# --- mutation helpers: they edit a copy, never the tree ---

# Remove numbered item n and its indented continuation lines.
drop_item() {
  awk -v n="$2" '
    $0 ~ "^" n "\\." { drop = 1; next }
    drop && /^[ \t]/ { next }
    { drop = 0; print }
  ' "$1"
}

# Print numbered item n and its continuation lines.
extract_item() {
  awk -v n="$2" '
    $0 ~ "^" n "\\." { p = 1; print; next }
    p && /^[ \t]/ { print; next }
    p { exit }
  ' "$1"
}

# Rewrite every leading number sequentially, so a mutation that renumbers cleanly
# still has to survive the per-section counts rather than the contiguity check.
renumber() {
  awk '{ if ($0 ~ /^[0-9]+\./) { n++; sub(/^[0-9]+\./, n ".") } print }' "$1"
}

# --- the template as shipped ---

if check_template "$TEMPLATE" > "$scratch/check.out" 2>&1; then
  pass "shipped runbook template matches the pinned per-section counts"
else
  fail "shipped runbook template drifted from the pinned counts"; cat "$scratch/check.out"
fi

# --- mutation 1: delete one MUST, renumber, expect red ---

drop_item "$TEMPLATE" 22 > "$scratch/m1.raw"
renumber "$scratch/m1.raw" > "$scratch/m1.md"
if check_template "$scratch/m1.md" > /dev/null 2>&1; then
  fail "deleting Evidence MUST 22 must fail the guard (it passed)"
else
  pass "deleting one MUST fails the guard"
fi

# --- mutation 2: move one MUST into SHOULD, renumber, expect red ---

drop_item "$TEMPLATE" 23 > "$scratch/m2.raw"
extract_item "$TEMPLATE" 23 >> "$scratch/m2.raw"
renumber "$scratch/m2.raw" > "$scratch/m2.md"
if check_template "$scratch/m2.md" > /dev/null 2>&1; then
  fail "moving Authorization MUST 23 into SHOULD must fail the guard (it passed)"
else
  pass "moving one MUST into SHOULD fails the guard"
fi

# --- run-init emits the template verbatim ---

# Run from a foreign cwd with a relative --base: run-init resolves the template
# from its own directory, so neither cwd nor an installed plugin cache changes it.
(cd "$scratch" && "$RUN_INIT" tmpl-run --base runs > /dev/null 2>&1)
if diff -u "$TEMPLATE" "$scratch/runs/tmpl-run/runbook.md" > "$scratch/diff.out" 2>&1; then
  pass "run-init copies the runbook template verbatim from any cwd"
else
  fail "run-init runbook.md differs from references/runbook-template.md"; cat "$scratch/diff.out"
fi

# --- a missing template is fatal, not a silent fallback ---

mkdir -p "$scratch/orphan"
cp -R "$HERE" "$scratch/orphan/scripts"
rm -rf "$scratch/orphan/scripts/tests"
rc=0
"$scratch/orphan/scripts/run-init" orphan-run --base "$scratch/orphan/runs" > /dev/null 2> "$scratch/orphan.err" || rc=$?
if [ "$rc" -ne 0 ] && grep -q 'references/runbook-template.md' "$scratch/orphan.err"; then
  pass "a missing template fails run-init and names the path it looked for"
else
  fail "run-init must fail loudly when the template is missing (rc=$rc)"; cat "$scratch/orphan.err"
fi
if [ -e "$scratch/orphan/runs/orphan-run/charter.md" ]; then
  fail "run-init scaffolded a run without the contract (charter written despite missing template)"
else
  pass "a missing template leaves no half-scaffolded run behind"
fi

echo "== $passed passed, $failed failed =="
[ "$failed" -eq 0 ]
