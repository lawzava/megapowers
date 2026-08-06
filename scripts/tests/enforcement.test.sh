#!/usr/bin/env bash
# Tests for scripts/check-enforcement.sh.
#
# Every assertion here is a mutation test: the fixture tree is built valid, the
# checker is asserted green on it, then exactly one field is broken and the
# checker is asserted red with a message naming that field. A checker that stays
# green on a broken contract is the failure mode worth guarding, because the
# whole point of the enforcement file is that a reader can trust it.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
script="$ROOT/scripts/check-enforcement.sh"

pass=0
fail=0
ok()  { printf '  PASS %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  FAIL %s\n' "$1"; fail=$((fail + 1)); }

work="$(mktemp -d "${TMPDIR:-/tmp}/enforcement-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# A minimal but complete fixture tree: one plugin, one enforced rule, the hook
# and source it names, and the lib-toml.sh the checker parses with. Built fresh
# for every case so one mutation cannot leak into the next.
build() {
  local dir="$1"
  rm -rf "$dir"
  mkdir -p "$dir/plugins/megapowers/hooks" "$dir/plugins/megapowers/skills/demo"
  cp "$ROOT/plugins/megapowers/hooks/lib-toml.sh" "$dir/plugins/megapowers/hooks/lib-toml.sh"
  # The hook body is now irrelevant to the checker, and that is deliberate. Two
  # earlier fixtures here were themselves the bug: a lone comment naming the
  # rules file passed the first check, and this unused assignment passed the
  # second. Text can always be shaped to pass a text test, so the proof moved to
  # a named contract_test and the checker only verifies the linkage.
  printf '#!/usr/bin/env bash\necho hardcoded\n' \
    > "$dir/plugins/megapowers/hooks/demo.sh"
  mkdir -p "$dir/plugins/megapowers/hooks/tests"
  # Must name the rule id. The checker requires the link to be to THIS rule, not
  # to the rules file in general, because any sibling suite mentions that.
  printf '#!/usr/bin/env bash\n# drives demo.sh through each state of demo-rule\n' \
    > "$dir/plugins/megapowers/hooks/tests/demo.test.sh"
  printf -- '---\nname: demo\n---\n' > "$dir/plugins/megapowers/skills/demo/SKILL.md"
  cat > "$dir/plugins/megapowers/enforcement.toml" <<'TOML'
[rules.demo-rule]
state    = "enforced"
hook     = "hooks/demo.sh"
source   = "skills/demo/SKILL.md"
promoted = "2026-08-05"
contract_test = "hooks/tests/demo.test.sh"

[rules.demo-rule.scope]
keywords = ["billing", "oauth"]
TOML
}

# run <dir> -> prints output, sets RC
run() { OUT="$("$script" --root "$1" 2>&1)"; RC=$?; }

expect_green() {
  local name="$1" dir="$2"
  run "$dir"
  if [ "$RC" -eq 0 ]; then ok "$name"; else bad "$name (expected exit 0, got $RC)"; printf '%s\n' "$OUT" | sed 's/^/      /'; fi
}

expect_red() {
  local name="$1" dir="$2" needle="$3"
  run "$dir"
  if [ "$RC" -eq 0 ]; then
    bad "$name (checker stayed green on a broken contract)"
  elif printf '%s' "$OUT" | grep -q "$needle"; then
    ok "$name"
  else
    bad "$name (failed, but no message matching '$needle')"
    printf '%s\n' "$OUT" | sed 's/^/      /'
  fi
}

d="$work/t"

# Baseline. Everything below mutates exactly one thing away from this.
build "$d"
expect_green "valid contract passes" "$d"

# state: the three legal values pass, an unknown one is rejected rather than
# silently read as off by every consumer.
for s in off advisory enforced; do
  build "$d"
  # advisory and off rules date with `declared`, not `promoted`.
  if [ "$s" != "enforced" ]; then
    sed -i 's/^promoted = /declared = /' "$d/plugins/megapowers/enforcement.toml"
  fi
  sed -i "s/^state    = \"enforced\"/state    = \"$s\"/" "$d/plugins/megapowers/enforcement.toml"
  expect_green "state=$s is accepted" "$d"
done

build "$d"
sed -i 's/^state    = "enforced"/state    = "enfoced"/' "$d/plugins/megapowers/enforcement.toml"
expect_red "misspelled state is rejected" "$d" "none of off, advisory, enforced"

build "$d"
sed -i '/^state    =/d' "$d/plugins/megapowers/enforcement.toml"
expect_red "missing state is rejected" "$d" "has no state"

# hook: must exist. Whether it HONORS its declared state is proven by the rule's
# contract_test, not here, for the reason recorded in check-enforcement.sh.
build "$d"
rm "$d/plugins/megapowers/hooks/demo.sh"
expect_red "hook that does not exist is rejected" "$d" "does not exist"

# A rule with no contract_test has nothing proving its declared state is what
# runs. That is the whole guarantee, so its absence is a failure, not a warning.
build "$d"
sed -i '/^contract_test/d' "$d/plugins/megapowers/enforcement.toml"
expect_red "rule with no contract_test is rejected" "$d" "names no contract_test"

build "$d"
rm "$d/plugins/megapowers/hooks/tests/demo.test.sh"
expect_red "contract_test that does not exist is rejected" "$d" "does not exist"

# A test file that never touches the rule is a link to nowhere. This is the
# shape both earlier versions of this check failed to catch, in their own way.
build "$d"
printf '#!/usr/bin/env bash\necho unrelated\n' > "$d/plugins/megapowers/hooks/tests/demo.test.sh"
expect_red "contract_test that never names the rule is rejected" "$d" "nothing ties it to this rule"

build "$d"
sed -i '/^hook     =/d' "$d/plugins/megapowers/enforcement.toml"
expect_red "missing hook is rejected" "$d" "names no hook"

# source: the skill a reader goes to for the reasoning.
build "$d"
rm "$d/plugins/megapowers/skills/demo/SKILL.md"
expect_red "dangling source is rejected" "$d" "does not exist"

# dates: an enforced rule dates its promotion, an advisory rule its declaration.
build "$d"
sed -i 's/^promoted = "2026-08-05"/promoted = "soon"/' "$d/plugins/megapowers/enforcement.toml"
expect_red "unparseable promoted date is rejected" "$d" "no parseable promoted date"

build "$d"
sed -i 's/^state    = "enforced"/state    = "advisory"/' "$d/plugins/megapowers/enforcement.toml"
expect_red "advisory rule without a declared date is rejected" "$d" "no parseable declared date"

# scope: an empty keyword list matches nothing while looking configured, which
# is precisely the quiet failure a security scan must not have.
build "$d"
sed -i 's/^keywords = .*/keywords = []/' "$d/plugins/megapowers/enforcement.toml"
expect_red "empty keyword list is rejected" "$d" "no keywords"

# A scope subsection must not be mistaken for a rule of its own; if it were, it
# would report as a rule with no state and the checker would fail on a valid
# file. The baseline green above already covers it, so assert the rule count.
build "$d"
run "$d"
if [ "$(printf '%s' "$OUT" | grep -c 'megapowers/demo-rule state=')" -eq 1 ]; then
  ok "scope subsection is not counted as a rule"
else
  bad "scope subsection leaked into the rule list"
fi

# No rules file at all is a broken install, not an empty configuration.
build "$d"
rm "$d/plugins/megapowers/enforcement.toml"
expect_red "missing enforcement.toml is rejected" "$d" "no plugins/\*/enforcement.toml found"

# A file that declares no rules is the same silence by another route.
build "$d"
printf '# nothing here\n' > "$d/plugins/megapowers/enforcement.toml"
expect_red "rules file with no rules is rejected" "$d" "declares no"

echo "== enforcement checker: $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
