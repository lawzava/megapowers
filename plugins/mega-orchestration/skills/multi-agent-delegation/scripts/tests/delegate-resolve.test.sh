#!/usr/bin/env bash
# Dependency-free tests for delegate-resolve config layering. Builds throwaway
# config layers under mktemp and asserts resolution output and exit codes.
# Run: plugins/mega-orchestration/skills/multi-agent-delegation/scripts/tests/delegate-resolve.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DR="$HERE/../delegate-resolve"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
unset DELEGATES_TOML
export XDG_CONFIG_HOME="$TMP/xdg"   # isolate the user layer
export HOME="$TMP/home"             # never read the real user config
mkdir -p "$TMP/xdg" "$TMP/home" "$TMP/proj"

pass=0; fail=0
check() {  # $1=desc $2=want-substring $3=got
  if printf '%s' "$3" | grep -qF "$2"; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL %s\n    want: %s\n    got:  %s\n' "$1" "$2" "$3"; fi
}
check_exit() {  # $1=desc $2=want-code $3=got-code
  if [ "$2" = "$3" ]; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL %s (want exit %s, got %s)\n' "$1" "$2" "$3"; fi
}

echo "== delegate-resolve layering tests =="

# Minimal self-contained v1 config for single-file mode.
cat > "$TMP/single.toml" <<'EOF'
[providers.alpha]
model   = "alpha-1"
vendor  = "acme"
binary  = "sh"
channel = "cli"
[defaults]
floor = "strong:low"
[roles]
code_review = "alpha"
EOF

out="$("$DR" code_review --config "$TMP/single.toml" 2>&1)"; rc=$?
check_exit "single-file --config resolves" 0 "$rc"
check "single-file MODEL" "MODEL=alpha-1" "$out"

out="$(DELEGATES_TOML="$TMP/single.toml" "$DR" code_review 2>&1)"; rc=$?
check_exit "env single-file resolves" 0 "$rc"
check "env single-file MODEL" "MODEL=alpha-1" "$out"

# Layered mode: shipped defaults plus a project override. binary=sh keeps
# resolution independent of which CLIs this machine has installed.
mkdir -p "$TMP/proj/.megapowers"
cat > "$TMP/proj/.megapowers/delegates.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.codex.tiers]
frontier = "project-override-model"
EOF
out="$(cd "$TMP/proj" && "$DR" code_review 2>&1)"; rc=$?
check_exit "project layer resolves" 0 "$rc"
check "project layer overrides model" "MODEL=project-override-model" "$out"
check "shipped layer still supplies vendor" "VENDOR=openai" "$out"

# User layer: wins over shipped, loses to project.
mkdir -p "$XDG_CONFIG_HOME/megapowers"
cat > "$XDG_CONFIG_HOME/megapowers/delegates.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.codex.tiers]
frontier = "user-override-model"
EOF
out="$(cd "$TMP/home" && "$DR" code_review 2>&1)"
check "user layer overrides shipped" "MODEL=user-override-model" "$out"
out="$(cd "$TMP/proj" && "$DR" code_review 2>&1)"
check "project layer beats user layer" "MODEL=project-override-model" "$out"

# --where lists active layers, highest priority first.
out="$(cd "$TMP/proj" && "$DR" --where 2>&1)"
check "--where lists project layer first" ".megapowers/delegates.toml" "$(printf '%s' "$out" | head -1)"

# A malformed override layer fails loudly, naming the file.
printf 'not toml at all\n' > "$TMP/proj/.megapowers/delegates.toml"
out="$(cd "$TMP/proj" && "$DR" code_review 2>&1)"; rc=$?
check_exit "broken project layer exits 2" 2 "$rc"
check "broken layer error names the file" ".megapowers/delegates.toml" "$out"

echo "== schema v2 tests =="

cat > "$TMP/v2.toml" <<'EOF'
[lead]
provider = "alpha"
tier     = "strong"
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.alpha]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
capabilities = ["code"]
[providers.alpha.tiers]
strong = "alpha-strong-1"
[providers.beta]
vendor = "bmax"
binary = "sh"
channel = "cli"
default_tier = "frontier"
capabilities = ["code", "vision"]
[providers.beta.tiers]
frontier = "beta-max-9"
[providers.gamma]
vendor = "gcorp"
binary = "sh"
channel = "cli"
default_tier = "fast"
[providers.gamma.tiers]
fast = "gamma-fast-1"
[defaults]
floor = "strong:low"
[requires]
visual_verify = ["vision"]
[roles]
code_review = "alpha"
verify = "alpha"
visual_verify = "alpha"
cheap = "gamma"
[fallbacks]
verify = ["alpha", "beta"]
visual_verify = ["alpha", "beta"]
EOF

out="$("$DR" code_review --config "$TMP/v2.toml" 2>&1)"; rc=$?
check_exit "v2 tier resolution exits 0" 0 "$rc"
check "v2 MODEL from tier map" "MODEL=alpha-strong-1" "$out"
check "v2 TIER emitted" "TIER=strong" "$out"

out="$("$DR" --lead --config "$TMP/v2.toml" 2>&1)"
check "--lead provider" "LEAD_PROVIDER=alpha" "$out"
check "--lead tier" "LEAD_TIER=strong" "$out"
check "--lead model" "LEAD_MODEL=alpha-strong-1" "$out"
check "--lead vendor" "LEAD_VENDOR=acme" "$out"

out="$("$DR" verify --exclude-lead --config "$TMP/v2.toml" 2>&1)"
check "--exclude-lead walks past the lead vendor" "PROVIDER=beta" "$out"

out="$("$DR" visual_verify --config "$TMP/v2.toml" 2>&1)"
check "[requires] skips a provider missing a capability" "PROVIDER=beta" "$out"

out="$("$DR" cheap --config "$TMP/v2.toml" 2>&1)"; rc=$?
check_exit "provider below floor is skipped (no route left)" 3 "$rc"

cat > "$TMP/badfloor.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.alpha]
binary = "sh"
channel = "cli"
model = "alpha-1"
[defaults]
floor = "mega:low"
[roles]
code_review = "alpha"
EOF
out="$("$DR" code_review --config "$TMP/badfloor.toml" 2>&1)"; rc=$?
check_exit "floor tier outside scale exits 2" 2 "$rc"

# v1 back-compat: single.toml (legacy model key, no [tiers]) still resolves.
out="$("$DR" code_review --config "$TMP/single.toml" 2>&1)"; rc=$?
check_exit "v1 config still resolves" 0 "$rc"
check "v1 MODEL from legacy key" "MODEL=alpha-1" "$out"

echo "== --check tests =="
out="$("$DR" --check --config "$TMP/v2.toml" 2>&1)"; rc=$?
check_exit "--check clean config exits 0" 0 "$rc"

cat > "$TMP/broken-check.toml" <<'EOF'
[lead]
provider = "ghost"
[roles]
code_review = "missing"
EOF
out="$("$DR" --check --config "$TMP/broken-check.toml" 2>&1)"; rc=$?
check_exit "--check broken config exits 1" 1 "$rc"
check "--check names the missing role provider" "missing" "$out"
check "--check names the missing lead provider" "ghost" "$out"

cat > "$TMP/badleadtier.toml" <<'EOF'
[lead]
provider = "alpha"
tier     = "frontier"
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.alpha]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.alpha.tiers]
strong = "alpha-strong-1"
[roles]
code_review = "alpha"
EOF
out="$("$DR" --lead --config "$TMP/badleadtier.toml" 2>&1)"; rc=$?
check_exit "--lead with unmapped lead tier exits 2" 2 "$rc"
check "--lead unmapped-tier error names the tier" "frontier" "$out"
out="$("$DR" --check --config "$TMP/badleadtier.toml" 2>&1)"; rc=$?
check_exit "--check flags unmapped lead tier" 1 "$rc"

SHIPPED="$HERE/../../delegates.toml"
out="$(DELEGATES_TOML="$SHIPPED" "$DR" --check 2>&1)"; rc=$?
check_exit "--check shipped delegates.toml exits 0" 0 "$rc"

echo "== catalog (models.toml) tests =="

# Split resolution: slim routing file plus catalog file resolving together.
cat > "$TMP/routes.toml" <<'EOF'
[roles]
code_review = "beta"
[presets.read_only]
sandbox = "read-only"
EOF
cat > "$TMP/catalog.toml" <<'EOF'
[lead]
provider = "beta"
tier     = "frontier"
[tiers]
scale = ["fast", "strong", "frontier"]
[tiers.use]
frontier = "lead and judge"
[providers.beta]
vendor = "bmax"
binary = "sh"
channel = "cli"
default_tier = "frontier"
[providers.beta.tiers]
frontier = "beta-max-9"
[defaults]
floor = "strong:low"
EOF
out="$("$DR" code_review --config "$TMP/routes.toml" --models "$TMP/catalog.toml" 2>&1)"; rc=$?
check_exit "split files resolve" 0 "$rc"
check "split MODEL from catalog" "MODEL=beta-max-9" "$out"
check "split FLOOR from catalog" "FLOOR=strong:low" "$out"

out="$("$DR" --lead --config "$TMP/routes.toml" --models "$TMP/catalog.toml" 2>&1)"
check "--lead from catalog" "LEAD_MODEL=beta-max-9" "$out"

# Legacy compatibility: inline providers in the delegates file win over the catalog.
out="$("$DR" code_review --config "$TMP/v2.toml" --models "$TMP/catalog.toml" 2>&1)"
check "inline providers beat the catalog" "MODEL=alpha-strong-1" "$out"

# MODELS_TOML env behaves like --models.
out="$(MODELS_TOML="$TMP/catalog.toml" "$DR" code_review --config "$TMP/routes.toml" 2>&1)"; rc=$?
check_exit "MODELS_TOML env resolves" 0 "$rc"
check "env catalog MODEL" "MODEL=beta-max-9" "$out"

# Layered catalog: user models.toml overrides the shipped catalog tier map.
# Remove the user delegates override from an earlier scenario first: the delegates
# stack wins over the catalog by design, and this scenario tests the catalog layer.
rm -f "$XDG_CONFIG_HOME/megapowers/delegates.toml"
mkdir -p "$XDG_CONFIG_HOME/megapowers"
cat > "$XDG_CONFIG_HOME/megapowers/models.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.codex.tiers]
frontier = "user-catalog-model"
EOF
out="$(cd "$TMP/home" && "$DR" code_review 2>&1)"
check "user catalog layer overrides shipped tier map" "MODEL=user-catalog-model" "$out"
rm -f "$XDG_CONFIG_HOME/megapowers/models.toml"

# --check spans both stacks.
out="$("$DR" --check --config "$TMP/routes.toml" --models "$TMP/catalog.toml" 2>&1)"; rc=$?
check_exit "--check across split files exits 0" 0 "$rc"

# A malformed catalog layer fails loudly, naming the file.
printf 'not toml either\n' > "$TMP/badcat.toml"
out="$("$DR" code_review --config "$TMP/routes.toml" --models "$TMP/badcat.toml" 2>&1)"; rc=$?
check_exit "broken catalog exits 2" 2 "$rc"
check "broken catalog error names the file" "badcat.toml" "$out"


echo "== lead-swap review-role tests =="

# With a codex lead, the review roles must resolve cross-vendor through their
# shipped [fallbacks] chains. Binaries pin to sh so the result does not depend
# on which CLIs this machine has installed.
mkdir -p "$TMP/codexlead/.megapowers"
cat > "$TMP/codexlead/.megapowers/models.toml" <<'EOF'
[lead]
provider = "codex"
tier     = "frontier"
[providers.codex]
binary = "sh"
[providers.claude]
binary = "sh"
EOF
for r in plan_review code_review; do
  out="$(cd "$TMP/codexlead" && "$DR" "$r" --exclude-lead 2>&1)"; rc=$?
  check_exit "$r --exclude-lead resolves under codex lead" 0 "$rc"
  check "$r falls back cross-vendor under codex lead" "PROVIDER=claude" "$out"
done

# The reverse swap: under a claude lead, the claude-primary roles must walk
# their chains to codex rather than dead-ending or resolving same-vendor.
mkdir -p "$TMP/claudelead/.megapowers"
cat > "$TMP/claudelead/.megapowers/models.toml" <<'EOF'
[lead]
provider = "claude"
tier     = "frontier"
[providers.codex]
binary = "sh"
[providers.claude]
binary = "sh"
EOF
for r in plan_review verify; do
  out="$(cd "$TMP/claudelead" && "$DR" "$r" --exclude-lead 2>&1)"; rc=$?
  check_exit "$r --exclude-lead resolves under claude lead" 0 "$rc"
  check "$r falls back cross-vendor under claude lead" "PROVIDER=codex" "$out"
done

echo "== efforts tests =="

# Floor effort outside the [efforts] scale fails loudly.
cat > "$TMP/badflooreffort.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[efforts]
scale = ["low", "medium", "high", "xhigh", "max"]
[providers.alpha]
binary = "sh"
channel = "cli"
model = "alpha-1"
[defaults]
floor = "strong:zzz"
[roles]
code_review = "alpha"
EOF
out="$("$DR" code_review --config "$TMP/badflooreffort.toml" 2>&1)"; rc=$?
check_exit "floor effort outside scale exits 2" 2 "$rc"
check "floor effort error names the effort" "zzz" "$out"

# A valid floor effort passes; a floor with no effort half also passes.
sed 's/floor = "strong:zzz"/floor = "strong:low"/' "$TMP/badflooreffort.toml" > "$TMP/goodflooreffort.toml"
out="$("$DR" code_review --config "$TMP/goodflooreffort.toml" 2>&1)"; rc=$?
check_exit "valid floor effort resolves" 0 "$rc"
sed 's/floor = "strong:zzz"/floor = "strong"/' "$TMP/badflooreffort.toml" > "$TMP/noeffort.toml"
out="$("$DR" code_review --config "$TMP/noeffort.toml" 2>&1)"; rc=$?
check_exit "floor without effort half resolves" 0 "$rc"

# --check flags a provider default effort its own efforts list does not allow.
cat > "$TMP/badproveffort.toml" <<'EOF'
[efforts]
scale = ["low", "medium", "high", "xhigh", "max"]
[providers.alpha]
binary = "sh"
channel = "cli"
model = "alpha-1"
effort = "max"
efforts = ["low", "medium", "high", "xhigh"]
[roles]
code_review = "alpha"
EOF
out="$("$DR" --check --config "$TMP/badproveffort.toml" 2>&1)"; rc=$?
check_exit "--check flags provider effort outside its efforts list" 1 "$rc"
check "--check names the offending effort" "max" "$out"

# Shipped catalog carries the efforts scale.
out="$(DELEGATES_TOML="$HERE/../../delegates.toml" MODELS_TOML="$HERE/../../../../models.toml" "$DR" --check 2>&1)"; rc=$?
check_exit "--check shipped files with efforts exits 0" 0 "$rc"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
