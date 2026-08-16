#!/usr/bin/env bash
# Dependency-free tests for delegate-resolve config layering. Builds throwaway
# config layers under mktemp and asserts resolution output and exit codes.
# Run: plugins/mega-orchestration/skills/multi-agent-delegation/scripts/tests/delegate-resolve.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DR="$HERE/../delegate-resolve"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin"
for binary in claude codex opencode; do
  printf '#!/bin/sh\nexit 0\n' > "$TMP/bin/$binary"
  chmod +x "$TMP/bin/$binary"
done
export PATH="$TMP/bin:$PATH"
unset DELEGATES_TOML
export XDG_CONFIG_HOME="$TMP/xdg"   # isolate the user layer
export HOME="$TMP/home"             # never read the real user config
mkdir -p "$TMP/xdg" "$TMP/home" "$TMP/proj"

pass=0; fail=0
check() {  # $1=desc $2=want-substring $3=got
  if printf '%s' "$3" | grep -qF -- "$2"; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL %s\n    want: %s\n    got:  %s\n' "$1" "$2" "$3"; fi
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
out="$(cd "$TMP/proj" && "$DR" code_review --author-vendor anthropic 2>&1)"; rc=$?
check_exit "project layer resolves" 0 "$rc"
check "project layer overrides model" "MODEL=project-override-model" "$out"
check "shipped layer still supplies vendor" "VENDOR=openai" "$out"

# A defined empty array replaces the shipped fallback chain. With codex excluded,
# the primary-only route must fail instead of falling through to shipped claude.
mkdir -p "$TMP/emptyfallback/.megapowers"
cat > "$TMP/emptyfallback/.megapowers/delegates.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.claude]
binary = "sh"
[roles]
code_review = "codex"
[fallbacks]
code_review = []
EOF
# The author must be a vendor the catalog declares; a placeholder would exclude
# nobody. The subject here is the empty fallback array, not the author.
out="$(cd "$TMP/emptyfallback" && "$DR" code_review --author-vendor anthropic --exclude codex 2>&1)"; rc=$?
check_exit "empty project fallback array replaces shipped chain" 3 "$rc"

# User layer: wins over shipped, loses to project.
mkdir -p "$XDG_CONFIG_HOME/megapowers"
cat > "$XDG_CONFIG_HOME/megapowers/delegates.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.codex.tiers]
frontier = "user-override-model"
EOF
out="$(cd "$TMP/home" && "$DR" code_review --author-vendor anthropic 2>&1)"
check "user layer overrides shipped" "MODEL=user-override-model" "$out"
out="$(cd "$TMP/proj" && "$DR" code_review --author-vendor anthropic 2>&1)"
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

# Runtime adapters are launch surfaces, not model vendors. A BYO OpenCode caller
# must identify its runtime without being treated as the catalog's Codex provider.
cat > "$TMP/adapters.toml" <<'EOF'
[tiers]
scale = ["strong"]
[adapters.codex]
binary = "sh"
channel = "codex-native"
[adapters.opencode]
binary = "sh"
channel = "opencode-agent"
[providers.openai]
vendor = "openai"
adapter = "codex"
binary = "sh"
channel = "legacy-codex-channel"
default_tier = "strong"
[providers.openai.tiers]
strong = "gpt-test"
[roles]
small_impl = "openai"
[role_tiers]
small_impl = "strong"
EOF
out="$("$DR" small_impl --config "$TMP/adapters.toml" --caller-adapter opencode 2>&1)"; rc=$?
check_exit "a caller runtime adapter is accepted independently of model provider" 0 "$rc"
check "a BYO OpenCode caller is not marked native to Codex" "DISPATCH=cli" "$out"
check "the resolved launch adapter is provider-keyed" "ADAPTER=codex" "$out"
check "the caller runtime is reported separately" "CALLER_ADAPTER=opencode" "$out"

# A BYO runtime can host a catalogued provider through a different adapter. The
# declared provider proves model identity; the declared adapter is the native launch
# surface. A different provider must still cross a runtime.
cat > "$TMP/byo-adapters.toml" <<'EOF'
[tiers]
scale = ["strong"]
[adapters.codex]
binary = "sh"
channel = "codex-native"
[adapters.opencode]
binary = "sh"
channel = "opencode-agent"
[providers.openai]
vendor = "openai"
adapter = "codex"
binary = "sh"
channel = "legacy-codex-channel"
default_tier = "strong"
[providers.openai.tiers]
strong = "gpt-test"
[providers.other]
vendor = "other"
adapter = "codex"
binary = "sh"
channel = "legacy-codex-channel"
default_tier = "strong"
[providers.other.tiers]
strong = "other-test"
[roles]
small_impl = "self"
cross_impl = "other"
[role_tiers]
small_impl = "strong"
cross_impl = "strong"
EOF
out="$("$DR" small_impl --config "$TMP/byo-adapters.toml" --caller-provider openai --caller-adapter opencode 2>&1)"; rc=$?
check_exit "a BYO caller can dispatch its declared provider natively" 0 "$rc"
check "a BYO self route is native by declared provider" "DISPATCH=native" "$out"
check "a BYO native route reports its running adapter" "ADAPTER=opencode" "$out"
check "a BYO native route uses its running adapter channel" "CHANNEL=opencode-agent" "$out"
out="$("$DR" cross_impl --config "$TMP/byo-adapters.toml" --caller-provider openai --caller-adapter opencode 2>&1)"; rc=$?
check_exit "a different provider still resolves" 0 "$rc"
check "a different provider remains a cli route" "DISPATCH=cli" "$out"
out="$("$DR" cross_impl --config "$TMP/byo-adapters.toml" --caller-provider openai --caller-adapter codex 2>&1)"; rc=$?
check_exit "a same-adapter different-vendor provider still resolves" 0 "$rc"
check "adapter equality cannot make a different vendor native" "DISPATCH=cli" "$out"

# A provider's adapter is an explicit contract, not an optional spelling. Bare
# reachability must probe the adapter binary too, otherwise it advertises a route
# that role resolution cannot launch.
cat > "$TMP/bad-adapter.toml" <<'EOF'
[adapters.good]
binary = "sh"
channel = "good"
[providers.alpha]
adapter = "typo"
vendor = "acme"
binary = "sh"
model = "alpha"
[roles]
small_impl = "alpha"
EOF
out="$("$DR" --check --config "$TMP/bad-adapter.toml" 2>&1)"; rc=$?
check_exit "--check rejects an unknown provider adapter" 1 "$rc"
check "unknown adapter diagnostic names the adapter" "typo" "$out"
out="$("$DR" small_impl --config "$TMP/bad-adapter.toml" 2>&1)"; rc=$?
check_exit "resolution rejects an unknown provider adapter" 2 "$rc"
check "resolution names the declared missing adapter" "missing runtime adapter 'typo'" "$out"
# Reachability probes the binary dispatch will actually use, which is the
# provider's when it declares one and the adapter's otherwise. Both directions
# are pinned below, because either one alone passes under a resolver that reads
# only its own half. Here the provider declares no binary, so the adapter's
# uninstalled one is what would launch and the vendor must not be advertised.
cat > "$TMP/adapter-binary.toml" <<'EOF'
[adapters.missing]
binary = "definitely-not-an-installed-command"
channel = "missing"
[providers.alpha]
adapter = "missing"
vendor = "acme"
model = "alpha"
[roles]
small_impl = "alpha"
EOF
out="$("$DR" --vendors --config "$TMP/adapter-binary.toml" 2>&1)"; rc=$?
check_exit "bare vendors accepts an adapter-backed catalog" 0 "$rc"
case "$out" in
  *acme*) fail=$((fail+1)); echo "  FAIL bare vendors probes the adapter binary" ;;
  *) pass=$((pass+1)) ;;
esac

# The mirror: an installed adapter binary cannot rescue a provider that names an
# uninstalled one. Without this case the resolver could ignore provider-level
# binaries entirely and still pass the case above.
cat > "$TMP/provider-binary.toml" <<'EOF'
[adapters.present]
binary = "sh"
channel = "present"
[providers.alpha]
adapter = "present"
vendor = "acme"
binary = "definitely-not-an-installed-command"
model = "alpha"
[roles]
small_impl = "alpha"
EOF
out="$("$DR" --vendors --config "$TMP/provider-binary.toml" 2>&1)"; rc=$?
check_exit "bare vendors accepts a provider-binary catalog" 0 "$rc"
case "$out" in
  *acme*) fail=$((fail+1)); echo "  FAIL bare vendors probes the provider binary" ;;
  *) pass=$((pass+1)) ;;
esac

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
out="$(cd "$TMP/home" && "$DR" code_review --author-vendor anthropic 2>&1)"
check "user catalog layer overrides shipped tier map" "MODEL=user-catalog-model" "$out"
# The same layer's `binary` has to land too, and it is the half that used to be
# dropped. The shipped provider declares an adapter, the resolver read every
# value from that adapter section, and so a provider-keyed layer was read,
# parsed, and ignored: the exact case models.toml's header promises still works.
# Nothing caught it, because on a machine with the real CLI installed the route
# still resolves, just to the binary the layer was trying to replace.
check "user catalog layer overrides the provider binary" "BINARY=sh" "$out"
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
[providers.qwen]
binary = "sh"
[providers.moonshot]
binary = "sh"
EOF
# qwen is asserted rather than claude because it sits ahead of claude in the shipped
# chain: claude is deliberately last, being the default lead's vendor. The binaries
# are pinned to sh so this asserts chain ORDER, not which CLIs this box happens to
# have. What the check is really for is that an openai-authored artifact under a
# codex lead leaves the author's vendor at all.
for r in plan_review code_review; do
  out="$(cd "$TMP/codexlead" && "$DR" "$r" --author-vendor openai 2>&1)"; rc=$?
  check_exit "$r resolves cross-vendor under codex lead" 0 "$rc"
  check "$r falls back cross-vendor under codex lead" "PROVIDER=qwen" "$out"
done

# The reverse swap: under a claude lead (the shipped default), an
# anthropic-authored artifact must walk the chain to codex rather than
# dead-ending or resolving same-vendor.
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
  out="$(cd "$TMP/claudelead" && "$DR" "$r" --author-vendor anthropic 2>&1)"; rc=$?
  check_exit "$r resolves cross-vendor under claude lead" 0 "$rc"
  check "$r falls back cross-vendor under claude lead" "PROVIDER=codex" "$out"
done

echo "== disabled-route exit codes =="

# A multi-candidate chain that is entirely disabled is "no available route"
# (exit 3); exit 4 stays reserved for a single-route role whose only provider
# is switched off.
cat > "$TMP/alldisabled.toml" <<'EOF'
[providers.alpha]
model = "alpha-1"
binary = "sh"
channel = "cli"
enabled = false
[providers.beta]
model = "beta-1"
binary = "sh"
channel = "cli"
enabled = false
[roles]
verify = "alpha"
[fallbacks]
verify = ["alpha", "beta"]
EOF
out="$("$DR" verify --config "$TMP/alldisabled.toml" 2>&1)"; rc=$?
check_exit "fully-disabled chain exits 3, not 4" 3 "$rc"

cat > "$TMP/onedisabled.toml" <<'EOF'
[providers.alpha]
model = "alpha-1"
binary = "sh"
channel = "cli"
enabled = false
[roles]
verify = "alpha"
EOF
out="$("$DR" verify --config "$TMP/onedisabled.toml" 2>&1)"; rc=$?
check_exit "single-route disabled provider exits 4" 4 "$rc"
check "single-route disabled prints ENABLED=false" "ENABLED=false" "$out"

# A one-entry chain that names a different provider than the role's primary
# must report THAT provider as the disabled one, not the primary.
cat > "$TMP/chainofone.toml" <<'EOF'
[providers.alpha]
model = "alpha-1"
binary = "sh"
channel = "cli"
[providers.beta]
model = "beta-1"
binary = "sh"
channel = "cli"
enabled = false
[roles]
verify = "alpha"
[fallbacks]
verify = ["beta"]
EOF
out="$("$DR" verify --config "$TMP/chainofone.toml" 2>&1)"; rc=$?
check_exit "one-entry disabled chain exits 4" 4 "$rc"
check "one-entry disabled chain names the sole candidate" "PROVIDER=beta" "$out"

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

# A provider whose default effort is below the configured floor is skipped.
cat > "$TMP/lowproveffort.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[efforts]
scale = ["low", "medium", "high", "xhigh", "max"]
[providers.alpha]
binary = "sh"
channel = "cli"
default_tier = "strong"
effort = "low"
[providers.alpha.tiers]
strong = "alpha-strong"
[defaults]
floor = "strong:high"
[roles]
code_review = "alpha"
EOF
out="$("$DR" code_review --config "$TMP/lowproveffort.toml" 2>&1)"; rc=$?
check_exit "provider below effort floor is skipped" 3 "$rc"

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

# Both frontier CLIs expose a real `max` rung, and the catalog treats them
# differently on purpose. Codex keeps it: no default role spends it without eval
# evidence, but the rung still buys something when one does. Claude does not,
# because Artificial Analysis measures Opus 5 at max scoring below its own xhigh
# on composite, cost, latency, and comprehension simultaneously. A rung that is
# worse on every axis than the rung beneath it is not an escalation, so the
# catalog must refuse to resolve it rather than merely decline to default to it.
sed -e 's/^binary  = "claude"$/binary  = "sh"/' \
    -e 's/^binary  = "codex"$/binary  = "sh"/' \
    "$HERE/../../../../models.toml" > "$TMP/shipped-max.toml"
cat >> "$TMP/shipped-max.toml" <<'EOF'
[roles]
claude_max = "claude"
codex_max = "codex"
[role_tiers]
claude_max = "frontier"
codex_max = "frontier"
[role_efforts]
claude_max = "max"
codex_max = "max"
EOF
: > "$TMP/no-max-catalog.toml"
MAX_CFG=(--config "$TMP/shipped-max.toml" --models "$TMP/no-max-catalog.toml")
out="$("$DR" codex_max "${MAX_CFG[@]}" 2>&1)"; rc=$?
check_exit "codex_max resolves at max" 0 "$rc"
check "codex_max preserves max effort" "EFFORT=max" "$out"

out="$("$DR" claude_max "${MAX_CFG[@]}" 2>&1)"; rc=$?
check_exit "claude refuses a max-effort role" 2 "$rc"
check "and names the effort it will not route" "does not support role 'claude_max' effort 'max'" "$out"

# The refusal has to come from the catalog, not from a role table nobody ships:
# assert the shipped efforts list itself, so an override layer restoring `max`
# is a deliberate act rather than something this test would keep quiet about.
claude_efforts_line="$(sed -n '/^\[providers.claude\]/,/^\[/p' "$HERE/../../../../models.toml" |
  sed -n 's/^efforts[[:space:]]*=.*/&/p')"
case "$claude_efforts_line" in
  *'"max"'*) fail=$((fail+1)); printf '  FAIL shipped claude provider still lists max\n    got: %s\n' "$claude_efforts_line" ;;
  *'"xhigh"'*) pass=$((pass+1)) ;;
  *) fail=$((fail+1)); printf '  FAIL could not read the shipped claude efforts list\n    got: %s\n' "$claude_efforts_line" ;;
esac

echo "== v0.5 role-policy tests =="

# The author, not the catalog lead, defines independence. With a Codex lead and
# an Anthropic-authored artifact, verification must select OpenAI.
mkdir -p "$TMP/author-policy/.megapowers"
cat > "$TMP/author-policy/.megapowers/models.toml" <<'EOF'
[providers.codex]
binary = "sh"
[providers.claude]
binary = "sh"
EOF
cat > "$TMP/author-policy/.megapowers/delegates.toml" <<'EOF'
[drivers.playwright]
binary = "sh"
EOF
out="$(cd "$TMP/author-policy" && "$DR" verify --author-vendor anthropic 2>&1)"; rc=$?
check_exit "author-vendor route resolves" 0 "$rc"
check "Anthropic author selects OpenAI verifier" "VENDOR=openai" "$out"
check "author vendor is emitted" "AUTHOR_VENDORS=anthropic" "$out"

out="$(cd "$TMP/author-policy" && "$DR" verify --exclude-lead 2>&1)"; rc=$?
check_exit "--exclude-lead alone does not satisfy author policy" 2 "$rc"
check "missing author policy is explicit" "--author-vendor" "$out"

# Every vendor in the judge chain has to be named for this to have nothing left. The
# list grew on 2026-08-10 (alibaba, moonshot), which is the point of that change: a
# judge used to disappear as soon as two vendors authored the candidates.
out="$(cd "$TMP/author-policy" && "$DR" judge --author-vendor openai --author-vendor anthropic \
  --author-vendor alibaba --author-vendor moonshot 2>&1)"; rc=$?
check_exit "all author vendors excluded leaves no judge" 3 "$rc"

# The counterpart, and the reason the chain was extended: two authors no longer
# exhaust the judges.
out="$(cd "$TMP/author-policy" && "$DR" judge --author-vendor openai --author-vendor anthropic 2>&1)"; rc=$?
check_exit "two author vendors still leave a judge" 0 "$rc"

# small_impl ships `self`, so it follows the caller instead of leaving the vendor:
# no author declared means the catalog [lead], and the role policy still applies.
out="$(cd "$TMP/author-policy" && "$DR" small_impl 2>&1)"; rc=$?
check_exit "small_impl resolves with role policy" 0 "$rc"
check "small_impl without an author follows the lead" "MODEL=claude-sonnet-5" "$out"
check "small_impl tier is strong" "TIER=strong" "$out"
# Medium, not high, and the value is the assertion. GPT-5.6 guidance makes medium
# the balanced starting point and asks for eval evidence before high; scoped
# implementation whose output the lead tests anyway does not clear that bar.
# A change back to high has to beat that argument, not just edit a table.
check "small_impl effort is medium" "EFFORT=medium" "$out"

out="$(cd "$TMP/author-policy" && "$DR" small_impl --author-model gpt-5.6-sol 2>&1)"; rc=$?
check_exit "small_impl resolves for a non-lead caller" 0 "$rc"
check "small_impl selects the caller's own strong model" "MODEL=gpt-5.6-terra" "$out"
check "small_impl stays in the caller's vendor" "VENDOR=openai" "$out"

# A v2 provider cannot bypass the ship floor by omitting or inventing a tier.
cat > "$TMP/missing-tier.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[efforts]
scale = ["low", "medium", "high"]
[providers.alpha]
vendor = "acme"
binary = "sh"
channel = "cli"
model = "alpha-legacy"
effort = "high"
[defaults]
floor = "strong:low"
[roles]
code_review = "alpha"
[role_tiers]
code_review = "strong"
[role_efforts]
code_review = "high"
EOF
out="$("$DR" code_review --config "$TMP/missing-tier.toml" 2>&1)"; rc=$?
check_exit "v2 provider without requested tier mapping fails closed" 2 "$rc"

sed 's/code_review = "strong"/code_review = "unknown"/' "$TMP/missing-tier.toml" > "$TMP/unknown-role-tier.toml"
out="$("$DR" code_review --config "$TMP/unknown-role-tier.toml" 2>&1)"; rc=$?
check_exit "unknown role tier exits 2" 2 "$rc"

# Visual verification resolves a model judge plus a separate Playwright driver.
out="$(cd "$TMP/author-policy" && "$DR" visual_verify --author-vendor openai 2>&1)"; rc=$?
check_exit "visual_verify resolves" 0 "$rc"
check "visual verifier is a model provider" "PROVIDER=claude" "$out"
check "visual verifier has a model" "MODEL=claude-opus-5" "$out"
check "visual verifier has a tier" "TIER=frontier" "$out"
# Medium for the same reason small_impl is: judging screenshot evidence is bound
# by the driver and the evidence, not by reasoning depth.
check "visual verifier effort is medium" "EFFORT=medium" "$out"
check "visual verifier carries Playwright driver" "DRIVER=playwright" "$out"
check "visual verifier carries resolved driver binary" "DRIVER_BINARY=sh" "$out"

# --vendors: the reachability primitive the Stop-hook nudge reads to decide whether
# an independent cross-vendor review is achievable at all on this machine.
cat > "$TMP/vendors.toml" <<'EOF'
[tiers]
scale = ["fast", "strong", "frontier"]
[providers.alpha]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.alpha.tiers]
strong = "alpha-1"
[providers.beta]
vendor = "globex"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.beta.tiers]
strong = "beta-1"
[providers.gamma]
vendor = "acme"
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.gamma.tiers]
strong = "gamma-1"
[providers.delta]
vendor = "initech"
binary = "definitely-not-an-installed-binary-xyz"
channel = "cli"
default_tier = "strong"
[providers.delta.tiers]
strong = "delta-1"
[providers.epsilon]
vendor = "umbrella"
enabled = false
binary = "sh"
channel = "cli"
default_tier = "strong"
[providers.epsilon.tiers]
strong = "epsilon-1"
[roles]
code_review = "alpha"
EOF
: > "$TMP/empty-catalog.toml"
out="$("$DR" --vendors --config "$TMP/vendors.toml" --models "$TMP/empty-catalog.toml" 2>&1)"; rc=$?
check_exit "--vendors exits 0" 0 "$rc"
check "--vendors lists a reachable vendor" "acme" "$out"
check "--vendors lists the second reachable vendor" "globex" "$out"
n="$(printf '%s\n' "$out" | grep -c .)"
check_exit "--vendors deduplicates and drops absent/disabled providers" 2 "$n"
case "$out" in
  *initech*) fail=$((fail + 1)); echo "  FAIL --vendors must drop a provider whose CLI is absent" ;;
  *) pass=$((pass + 1)) ;;
esac
case "$out" in
  *umbrella*) fail=$((fail + 1)); echo "  FAIL --vendors must drop a disabled provider" ;;
  *) pass=$((pass + 1)) ;;
esac

# The single-vendor case is the one the nudge degrades on.
sed '/^\[providers.beta\]$/,/^strong = "beta-1"$/d' "$TMP/vendors.toml" > "$TMP/one-vendor.toml"
out="$("$DR" --vendors --config "$TMP/one-vendor.toml" --models "$TMP/empty-catalog.toml" 2>&1)"
n="$(printf '%s\n' "$out" | grep -c .)"
check_exit "one reachable vendor reports a count of 1" 1 "$n"

# --- --author-model and self-routing ------------------------------------------------
# A BYO-model harness (OpenCode, Cursor, pi) knows the model id it is running, not
# the vendor name megapowers files it under. --author-model derives the vendor so
# the harness never hardcodes one. `self` routes a non-independence role back to
# that same provider instead of leaving the vendor by default.
cat > "$TMP/self.toml" <<'EOF'
[lead]
provider = "alpha"
tier     = "frontier"
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.alpha]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.alpha.tiers]
strong   = "alpha-strong"
frontier = "alpha-frontier"
[providers.beta]
vendor  = "globex"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.beta.tiers]
strong   = "beta-strong"
frontier = "beta-frontier"
[defaults]
floor = "strong:low"
[roles]
code_review = "beta"
small_impl  = "self"
static_own  = "alpha"
council_member = "alpha"
judge = "alpha"
[fallbacks]
code_review = ["beta", "alpha"]
[role_tiers]
code_review = "frontier"
small_impl  = "strong"
static_own  = "strong"
council_member = "strong"
judge = "strong"
[role_efforts]
code_review = "high"
small_impl  = "high"
static_own  = "high"
council_member = "high"
judge = "high"
[independence]
code_review = "author_vendor"
judge = "all_author_vendors"
EOF
SELF=(--config "$TMP/self.toml" --models "$TMP/empty-catalog.toml")

out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "--author-model satisfies an independence role" 0 "$rc"
check "--author-model routes away from the derived vendor" "PROVIDER=beta" "$out"
check "--author-model reports the derived vendor" "AUTHOR_VENDORS=acme" "$out"

out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier 2>&1)"
check "--author-model derives the other vendor" "PROVIDER=alpha" "$out"

out="$("$DR" code_review "${SELF[@]}" --author-model strong-alpha-typo 2>&1)"; rc=$?
check_exit "unknown --author-model is a usage error" 2 "$rc"
check "unknown --author-model names the model" "strong-alpha-typo" "$out"

out="$("$DR" code_review "${SELF[@]}" --author-model 2>&1)"; rc=$?
check_exit "--author-model needs an argument" 2 "$rc"

# v1 configs map the bare `model` key, so derivation must cover them too. A config
# with no [independence] table predates per-role policy, so its author exclusion stays
# unconditional: the alternative would silently weaken independence on legacy tables.
out="$("$DR" code_review --config "$TMP/single.toml" --author-model alpha-1 2>&1)"; rc=$?
check_exit "--author-model derives from a v1 model key" 3 "$rc"
out="$("$DR" code_review --config "$TMP/single.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "a config with no [independence] table still excludes its author" 3 "$rc"

out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "self role resolves" 0 "$rc"
check "self role stays with the author's provider" "PROVIDER=alpha" "$out"
check "self role still applies the role tier" "MODEL=alpha-strong" "$out"

out="$("$DR" small_impl "${SELF[@]}" --author-model beta-frontier 2>&1)"
check "self role follows the author, not the lead" "PROVIDER=beta" "$out"
check "self role tier applies to the other provider too" "MODEL=beta-strong" "$out"

# A route to the caller's OWN provider must not read as "shell out to yourself". The
# harness has a native subagent for that, and spawning a fresh CLI session would
# throw away the context that makes it worth delegating in the first place.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "a self route dispatches natively" "DISPATCH=native" "$out"
out="$("$DR" small_impl "${SELF[@]}" 2>&1)"
check "a lead-fallback self route dispatches natively" "DISPATCH=native" "$out"

# Crossing to another provider is what a CLI channel is for.
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "a cross-provider route dispatches by CLI" "DISPATCH=cli" "$out"

# Council members generate candidate answers, so they have no artifact author to
# exclude. A judge ranks those answers and remains author-bound.
out="$("$DR" council_member "${SELF[@]}" 2>&1)"; rc=$?
check_exit "a council member resolves without an artifact author" 0 "$rc"
out="$("$DR" judge "${SELF[@]}" 2>&1)"; rc=$?
check_exit "a judge still requires artifact authors" 2 "$rc"

# The rule follows the resolved provider, not the `self` keyword: a statically routed
# role that lands on the caller's own provider is equally a native dispatch.
out="$("$DR" static_own "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "a static route back to the caller dispatches natively" "DISPATCH=native" "$out"
# Nativeness follows the RUNNING session, so saying a different session is running is
# what makes the same static route cross a runtime.
out="$("$DR" static_own "${SELF[@]}" --caller-model beta-frontier 2>&1)"
check "the same static route from another caller dispatches by CLI" "DISPATCH=cli" "$out"

# Declaring who you are is now the universal instruction, so it must be safe to do it
# everywhere. Author exclusion is what [independence] asks for, and a role that never
# asked must not lose its own vendor just because the caller identified itself.
out="$("$DR" static_own "${SELF[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "identifying yourself does not exclude you from a non-independence role" 0 "$rc"
check "the non-independence role still resolves to the caller" "PROVIDER=alpha" "$out"

# The independence roles are unaffected: exclusion is exactly what they declare.
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "an independence role still excludes its author" 0 "$rc"
check "the independence role routes away from the author" "PROVIDER=beta" "$out"

# --exclude stays the explicit, policy-free way to drop a backend.
out="$("$DR" static_own "${SELF[@]}" --author-model alpha-frontier --exclude alpha 2>&1)"; rc=$?
check_exit "--exclude still drops a backend for a non-independence role" 3 "$rc"

out="$("$DR" small_impl "${SELF[@]}" 2>&1)"; rc=$?
check_exit "self role without an author falls back to [lead]" 0 "$rc"
check "self role falls back to the lead provider" "PROVIDER=alpha" "$out"
check "self role lead fallback still honors the role tier" "MODEL=alpha-strong" "$out"

# An explicit --exclude is a caller decision and outranks self-routing; only the
# author-derived exclusion is suspended for a self role.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --exclude alpha 2>&1)"; rc=$?
check_exit "explicit --exclude still blocks a self role" 3 "$rc"

out="$("$DR" small_impl "${SELF[@]}" --author-vendor globex 2>&1)"; rc=$?
check_exit "--author-vendor also selects the self provider" 0 "$rc"
check "self role accepts a vendor name as the author" "PROVIDER=beta" "$out"

out="$("$DR" --vendors small_impl "${SELF[@]}" --author-model beta-frontier 2>&1)"; rc=$?
check_exit "--vendors resolves a self role" 0 "$rc"
check "--vendors on a self role reports the author's vendor" "globex" "$out"
n="$(printf '%s\n' "$out" | grep -c .)"
check_exit "a self role reaches exactly one vendor" 1 "$n"

out="$("$DR" --check "${SELF[@]}" 2>&1)"; rc=$?
check_exit "--check accepts a self role" 0 "$rc"

echo "== self-role tier fallback =="

# `self` promises the CALLER's own backend, and a BYO-model harness can be running
# any model in the catalog. Several of them carry one tier only, so a role tier the
# caller's provider does not publish is a request that cannot be honoured as asked.
# Refusing there strands the caller entirely; retiering serves it and says so.
out="$(DELEGATES_TOML="$HERE/../../delegates.toml" MODELS_TOML="$HERE/../../../../models.toml" \
  "$DR" small_impl --caller-adapter opencode --caller-model qwen3.8-max 2>&1)"; rc=$?
check_exit "a self role resolves for a caller whose provider lacks the role tier" 0 "$rc"
check "the retiered self route stays with the caller's provider" "PROVIDER=qwen" "$out"
check "the retiered self route runs the caller's only model" "MODEL=qwen3.8-max" "$out"
check "TIER reports the tier the route actually got" "TIER=frontier" "$out"
check "the retier is reported, not silent" "TIER_FALLBACK=strong->frontier" "$out"

# The other direction, and it needs its own fixture: the only strong-only provider
# the shipped catalog carries is xai, which ships enabled = false.
cat > "$TMP/onetier.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "medium", "high"]
[lead]
provider = "solo"
tier     = "strong"
[providers.solo]
vendor  = "soloco"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "medium", "high"]
default_tier = "strong"
[providers.solo.tiers]
strong = "solo-strong"
[defaults]
floor = "strong:low"
[roles]
deep_work = "self"
[role_tiers]
deep_work = "frontier"
[role_efforts]
deep_work = "high"
EOF
ONE=(--config "$TMP/onetier.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" deep_work "${ONE[@]}" 2>&1)"; rc=$?
check_exit "a self role resolves down to the caller's only tier" 0 "$rc"
check "the downward retier runs the provider's only model" "MODEL=solo-strong" "$out"
check "the downward retier reports the resolved tier" "TIER=strong" "$out"
check "the downward retier is reported too" "TIER_FALLBACK=frontier->strong" "$out"

# Absence of the field is the contract for "the role got the tier it asked for", so
# a route that was never retiered must not emit it.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier 2>&1)"
if printf '%s' "$out" | grep -q 'TIER_FALLBACK='; then
  fail=$((fail+1)); printf '  FAIL a self route that got its own tier must not report a fallback\n    got: %s\n' "$out"
else
  pass=$((pass+1))
fi

# The floor is a ship rule, not a preference, so the fallback may not rescue a
# provider under it: retiering picks the nearest tier and the floor still skips it.
cat > "$TMP/subfloor.toml" <<'EOF'
[tiers]
scale = ["cheap", "strong", "frontier"]
[efforts]
scale = ["low", "medium", "high"]
[lead]
provider = "budget"
tier     = "cheap"
[providers.budget]
vendor  = "budgetco"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "medium", "high"]
default_tier = "cheap"
[providers.budget.tiers]
cheap = "budget-cheap"
[defaults]
floor = "strong:low"
[roles]
deep_work = "self"
[role_tiers]
deep_work = "frontier"
[role_efforts]
deep_work = "high"
EOF
out="$("$DR" deep_work --config "$TMP/subfloor.toml" --models "$TMP/empty-catalog.toml" 2>&1)"; rc=$?
check_exit "the fallback cannot rescue a provider below the floor" 3 "$rc"
check "the sub-floor skip still names the floor" "below floor" "$out"

# --vendors decides whether a role can route at all, so it must not report a self
# role as unreachable on the very tier mismatch resolution now absorbs.
out="$("$DR" --vendors deep_work "${ONE[@]}" 2>&1)"; rc=$?
check_exit "--vendors resolves a retiered self role" 0 "$rc"
check "--vendors reports the retiered self role's vendor" "soloco" "$out"

# An open-weights model is reachable through more than one host, so one model id can
# legitimately appear under two providers. When those providers differ in vendor, the
# id no longer identifies an author: picking either one would exclude a vendor the
# artifact's real author may not belong to, and the actual author could then be
# selected as its own reviewer. Refuse instead of guessing.
cat > "$TMP/ambiguous.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.firsthost]
vendor  = "othervendor"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.firsthost.tiers]
frontier = "shared-weights-1"
[providers.secondhost]
vendor  = "reseller"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.secondhost.tiers]
frontier = "shared-weights-1"
[defaults]
floor = "strong:low"
[roles]
verify = "firsthost"
[fallbacks]
verify = ["firsthost", "secondhost"]
[role_tiers]
verify = "frontier"
[role_efforts]
verify = "high"
[independence]
verify = "author_vendor"
EOF
AMB=(--config "$TMP/ambiguous.toml" --models "$TMP/empty-catalog.toml")

out="$("$DR" verify "${AMB[@]}" --author-model shared-weights-1 2>&1)"; rc=$?
check_exit "a model id spanning two vendors is refused" 2 "$rc"
check "the ambiguity error names the model" "shared-weights-1" "$out"
check "the ambiguity error names both vendors" "othervendor" "$out"
check "the ambiguity error names the second vendor" "reseller" "$out"
check "the ambiguity error offers the unambiguous flag" "--author-vendor" "$out"

# --author-vendor is unambiguous by construction, so the same config still resolves.
out="$("$DR" verify "${AMB[@]}" --author-vendor othervendor 2>&1)"; rc=$?
check_exit "--author-vendor still resolves on an ambiguous catalog" 0 "$rc"
check "--author-vendor routes away from the named vendor" "VENDOR=reseller" "$out"

out="$("$DR" --check "${AMB[@]}" 2>&1)"; rc=$?
check_exit "--check reports a cross-vendor duplicate model id" 1 "$rc"
check "--check names the ambiguous model id" "shared-weights-1" "$out"

# An author's own provider can be named directly. Unlike passing a provider name to
# --author-vendor, this still contributes the provider's VENDOR to the exclusion set,
# so disambiguating self-routing cannot quietly weaken an independence role.
out="$("$DR" verify "${AMB[@]}" --author-provider firsthost 2>&1)"; rc=$?
check_exit "--author-provider resolves an independence role" 0 "$rc"
check "--author-provider excludes the provider's whole vendor" "VENDOR=reseller" "$out"
check "--author-provider reports the derived vendor" "AUTHOR_VENDORS=othervendor" "$out"

out="$("$DR" verify "${AMB[@]}" --author-provider nosuchprovider 2>&1)"; rc=$?
check_exit "unknown --author-provider is a usage error" 2 "$rc"

# Two providers, ONE vendor: independence is unaffected (the vendor is the same
# either way), but `self` no longer knows WHICH backend the caller is, and guessing
# can route to a sibling or report no route when the caller's own CLI is present.
cat > "$TMP/same-vendor-dup.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.firsthost]
vendor  = "othervendor"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.firsthost.tiers]
strong   = "shared-weights-small"
frontier = "shared-weights-1"
[providers.secondhost]
vendor  = "othervendor"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.secondhost.tiers]
strong   = "shared-weights-small"
frontier = "shared-weights-1"
[lead]
provider = "firsthost"
tier     = "frontier"
[defaults]
floor = "strong:low"
[roles]
verify     = "firsthost"
small_impl = "self"
[fallbacks]
verify = ["firsthost", "secondhost"]
[role_tiers]
verify     = "frontier"
small_impl = "strong"
[role_efforts]
verify     = "high"
small_impl = "high"
[independence]
verify = "author_vendor"
EOF
DUP=(--config "$TMP/same-vendor-dup.toml" --models "$TMP/empty-catalog.toml")

out="$("$DR" verify "${DUP[@]}" --author-model shared-weights-1 2>&1)"; rc=$?
check_exit "one vendor behind a duplicated model id still resolves independence" 3 "$rc"

out="$("$DR" small_impl "${DUP[@]}" --author-model shared-weights-1 2>&1)"; rc=$?
check_exit "an ambiguous author provider is refused for a self role" 2 "$rc"
check "the self ambiguity error names both providers" "secondhost" "$out"
check "the self ambiguity error offers the unambiguous flag" "--author-provider" "$out"

out="$("$DR" small_impl "${DUP[@]}" --author-provider secondhost 2>&1)"; rc=$?
check_exit "--author-provider disambiguates a self role" 0 "$rc"
check "--author-provider selects the caller's own backend" "PROVIDER=secondhost" "$out"

# --author-vendor accepts a provider name for convenience, but authorship is a VENDOR
# claim: excluding only the named provider would leave a sibling of the same vendor
# eligible to review its own author's work.
out="$("$DR" verify "${DUP[@]}" --author-vendor firsthost 2>&1)"; rc=$?
check_exit "a provider name as author excludes its whole vendor" 3 "$rc"

out="$("$DR" verify "${AMB[@]}" --author-vendor firsthost 2>&1)"; rc=$?
check_exit "a provider name as author still resolves across vendors" 0 "$rc"
check "a provider name as author routes to the other vendor" "VENDOR=reseller" "$out"
check "a provider name as author is recorded as its vendor" "AUTHOR_VENDORS=othervendor" "$out"

# An author identity that names nothing is a typo, not an empty exclusion. Silently
# excluding no one is the worst outcome: the review appears independent and is not.
out="$("$DR" verify "${AMB[@]}" --author-vendor nosuchvendor 2>&1)"; rc=$?
check_exit "an unknown --author-vendor is a usage error" 2 "$rc"
check "the unknown author error names the value" "nosuchvendor" "$out"

out="$("$DR" small_impl "${DUP[@]}" --author-vendor nosuchvendor 2>&1)"; rc=$?
check_exit "an unknown --author-vendor never falls back to [lead]" 2 "$rc"

# The ambiguity only matters where a role actually routes to self, so --check reports
# it there and stays quiet on a catalog that never self-routes.
out="$("$DR" --check "${DUP[@]}" 2>&1)"; rc=$?
check_exit "--check reports a self-routing ambiguity" 1 "$rc"
check "--check names the ambiguous model for self-routing" "shared-weights-1" "$out"

sed 's/vendor  = "reseller"/vendor  = "othervendor"/' "$TMP/ambiguous.toml" > "$TMP/dup-no-self.toml"
out="$("$DR" --check --config "$TMP/dup-no-self.toml" --models "$TMP/empty-catalog.toml" 2>&1)"; rc=$?
check_exit "--check accepts a same-vendor duplicate with no self role" 0 "$rc"

# A provider NAME and a vendor NAME live in one flag's namespace, so they can collide:
# resolving the provider first would silently normalize to the wrong vendor and leave
# the intended author eligible. Refuse when the two namespaces disagree.
cat > "$TMP/shadowed.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.acme]
vendor  = "reseller"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.acme.tiers]
frontier = "acme-1"
[providers.other]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.other.tiers]
frontier = "other-1"
[defaults]
floor = "strong:low"
[roles]
verify = "acme"
[fallbacks]
verify = ["acme", "other"]
[role_tiers]
verify = "frontier"
[role_efforts]
verify = "high"
[independence]
verify = "author_vendor"
EOF
out="$("$DR" verify --config "$TMP/shadowed.toml" --models "$TMP/empty-catalog.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "an author token that is both a provider and a vendor is refused" 2 "$rc"
check "the shadowing error names the token" "acme" "$out"
check "the shadowing error offers the unambiguous flag" "--author-provider" "$out"

# --author-provider is namespace-unambiguous, so it still works on that catalog.
out="$("$DR" verify --config "$TMP/shadowed.toml" --models "$TMP/empty-catalog.toml" --author-provider acme 2>&1)"; rc=$?
check_exit "--author-provider resolves a shadowed name" 0 "$rc"
check "--author-provider picks the provider namespace" "AUTHOR_VENDORS=reseller" "$out"

# A self role has exactly one caller. Several declared identities pointing at
# different providers cannot say which one is asking, so refuse instead of taking
# the first or the last.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --author-model beta-frontier 2>&1)"; rc=$?
check_exit "two author models cannot identify one caller" 2 "$rc"
out="$("$DR" small_impl "${SELF[@]}" --author-provider alpha --author-provider beta 2>&1)"; rc=$?
check_exit "two author providers cannot identify one caller" 2 "$rc"
out="$("$DR" small_impl "${SELF[@]}" --author-vendor acme --author-vendor globex 2>&1)"; rc=$?
check_exit "two author vendors cannot identify one caller" 2 "$rc"

# The question a self role asks is WHICH PROVIDER is calling, so two model ids from
# one provider are not a conflict: they have exactly one correct answer. A lead and
# its subagent declaring different tiers of the same backend is the ordinary case.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-strong --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "two models from one provider identify that provider" 0 "$rc"
check "two models from one provider resolve to it" "PROVIDER=alpha" "$out"

# Identity flags constrain each other rather than override each other. Naming a
# provider is how a caller disambiguates a shared model id, which works because the
# provider is one of that id's candidates; a provider that is NOT among them
# contradicts the model claim, and precedence would silently discard one of the two.
out="$("$DR" small_impl "${SELF[@]}" --author-provider alpha --author-model beta-frontier 2>&1)"; rc=$?
check_exit "a provider contradicting a model is refused" 2 "$rc"
check "the conflict error says the identities disagree" "alpha" "$out"

out="$("$DR" small_impl "${SELF[@]}" --author-vendor acme --author-model beta-frontier 2>&1)"; rc=$?
check_exit "a vendor contradicting a model is refused" 2 "$rc"

out="$("$DR" small_impl "${SELF[@]}" --author-vendor acme --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "agreeing vendor and model identities resolve" 0 "$rc"
check "agreeing identities resolve to the shared provider" "PROVIDER=alpha" "$out"

out="$("$DR" small_impl "${DUP[@]}" --author-model shared-weights-1 --author-provider secondhost 2>&1)"; rc=$?
check_exit "a provider inside the model's candidates disambiguates it" 0 "$rc"
check "the disambiguated self role uses the named backend" "PROVIDER=secondhost" "$out"

# Every OCCURRENCE is a separate assertion about the one caller, so they all
# intersect. Two contradictory providers stay contradictory no matter what a
# narrowing flag of another type would allow.
out="$("$DR" small_impl "${SELF[@]}" --author-provider alpha --author-provider beta --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "a third flag cannot rescue two contradictory providers" 2 "$rc"

# The mirror case: two ambiguous model ids that overlap in exactly one provider do
# identify the caller, and unioning them would wrongly report ambiguity.
cat > "$TMP/overlap.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.h1]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.h1.tiers]
strong   = "h1-s"
frontier = "mA"
[providers.h2]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.h2.tiers]
strong   = "mB"
frontier = "mA"
[providers.h3]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.h3.tiers]
strong   = "mB"
frontier = "h3-f"
[defaults]
floor = "strong:low"
[roles]
small_impl = "self"
[role_tiers]
small_impl = "strong"
[role_efforts]
small_impl = "high"
EOF
out="$("$DR" small_impl --config "$TMP/overlap.toml" --models "$TMP/empty-catalog.toml" --author-model mA --author-model mB 2>&1)"; rc=$?
check_exit "two ambiguous ids overlapping in one provider identify it" 0 "$rc"
check "the overlapping identity resolves to the shared provider" "PROVIDER=h2" "$out"

out="$("$DR" small_impl --config "$TMP/overlap.toml" --models "$TMP/empty-catalog.toml" --author-model mA 2>&1)"; rc=$?
check_exit "one ambiguous id alone is still refused" 2 "$rc"

# Repeating ONE identity is not a conflict, and multiple authors stay legal for the
# independence roles that exist to collect them.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "the same author twice is not ambiguous" 0 "$rc"
check "the repeated author still resolves to itself" "PROVIDER=alpha" "$out"
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier --author-model beta-frontier 2>&1)"; rc=$?
check_exit "multiple authors remain legal for an independence role" 3 "$rc"

# A role cannot be both self-routed and independence-bound. --check reports it, but
# resolution must refuse it too: a caller that never runs --check would otherwise get
# a review routed straight back to its author.
cat > "$TMP/self-independence.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[lead]
provider = "alpha"
tier     = "frontier"
[providers.alpha]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.alpha.tiers]
strong   = "alpha-strong"
frontier = "alpha-frontier"
[defaults]
floor = "strong:low"
[roles]
code_review = "self"
[role_tiers]
code_review = "frontier"
[role_efforts]
code_review = "high"
[independence]
code_review = "author_vendor"
EOF
out="$("$DR" code_review --config "$TMP/self-independence.toml" --models "$TMP/empty-catalog.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "resolution refuses a self-routed independence role" 2 "$rc"
check "the self-independence error explains the conflict" "independence" "$out"

# Same conflict, other resolver path: --vendors must not report the author's vendor
# as reachable for a role that can never legally resolve.
out="$("$DR" --vendors code_review --config "$TMP/self-independence.toml" --models "$TMP/empty-catalog.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "--vendors refuses a self-routed independence role" 2 "$rc"

# `vendor` is optional: a provider without one is identified by its own name, and
# every identity helper has to agree on that or a self role silently reaches [lead].
cat > "$TMP/vendorless.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[lead]
provider = "leadhost"
tier     = "frontier"
[providers.leadhost]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.leadhost.tiers]
strong   = "lead-strong"
frontier = "lead-frontier"
[providers.bare]
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.bare.tiers]
strong   = "bare-strong"
frontier = "bare-frontier"
[defaults]
floor = "strong:low"
[roles]
small_impl = "self"
[role_tiers]
small_impl = "strong"
[role_efforts]
small_impl = "high"
EOF
out="$("$DR" small_impl --config "$TMP/vendorless.toml" --models "$TMP/empty-catalog.toml" --author-vendor bare 2>&1)"; rc=$?
check_exit "a vendorless provider can still identify the caller" 0 "$rc"
check "a vendorless provider resolves to itself, not the lead" "PROVIDER=bare" "$out"
check "a vendorless self route uses the role tier" "MODEL=bare-strong" "$out"

# AUTHOR_VENDORS is a comma-joined field in a line-oriented format, so a vendor whose
# name contains a comma cannot be represented in it. Consumers split that field to
# recover the identities, and a silent split would hand them two vendors that are not
# the one that was excluded. Refuse the unrepresentable value instead.
cat > "$TMP/comma-vendor.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.commahost]
vendor  = "foo,bar"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.commahost.tiers]
frontier = "comma-1"
[providers.plainhost]
vendor  = "plain"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.plainhost.tiers]
frontier = "plain-1"
[defaults]
floor = "strong:low"
[roles]
verify = "plainhost"
[fallbacks]
verify = ["plainhost", "commahost"]
[role_tiers]
verify = "frontier"
[role_efforts]
verify = "high"
[independence]
verify = "author_vendor"
EOF
CV=(--config "$TMP/comma-vendor.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" verify "${CV[@]}" --author-vendor 'foo,bar' 2>&1)"; rc=$?
check_exit "an unrepresentable vendor name is refused" 2 "$rc"
check "the unrepresentable vendor error names the field" "AUTHOR_VENDORS" "$out"

out="$("$DR" --check "${CV[@]}" 2>&1)"; rc=$?
check_exit "--check reports a comma-bearing vendor" 1 "$rc"

# A vendorless provider is identified by its own NAME, so --check tests the effective
# vendor. That path cannot be reached through the name itself: a comma is not a legal
# character in a section name, so the parse guard refuses the config outright. This
# pins that reason, which is what keeps the effective-vendor check total.
sed 's/^\[providers.commahost\]$/[providers.comma,host]/' "$TMP/comma-vendor.toml" > "$TMP/comma-name.toml"
out="$("$DR" --check --config "$TMP/comma-name.toml" --models "$TMP/empty-catalog.toml" 2>&1)"; rc=$?
check_exit "a comma in a provider name is a parse error" 2 "$rc"
check "the parse error names the offending line" "parse error" "$out"

# `binary` is the CROSS-RUNTIME entry point, so it gates a cli dispatch and nothing
# else. A native route runs on the harness's own surface, which is present by
# definition: a session IS the provider. Requiring its CLI would strand exactly the
# BYO-model harnesses this is for, since an OpenCode or Cursor session running an
# Anthropic model has no `claude` binary anywhere on PATH.
sed 's|^binary  = "sh"$|binary  = "definitely-not-an-installed-binary-xyz"|' "$TMP/self.toml" > "$TMP/self-absent.toml"
ABS=(--config "$TMP/self-absent.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" small_impl "${ABS[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "a native route does not need the provider CLI" 0 "$rc"
check "the native route still resolves the caller" "PROVIDER=alpha" "$out"
check "the native route is marked native" "DISPATCH=native" "$out"

# The same absent binary still kills a cross-runtime route, which genuinely needs it.
out="$("$DR" code_review "${ABS[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "a cli route still needs the provider CLI" 3 "$rc"

# WHO WROTE IT and WHO IS RUNNING are different questions, and they only coincide for
# a self role. Reviewing someone else's work is the ordinary multi-agent case: the
# author is excluded, and the route lands on the CALLER, which is a native dispatch.
out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier --caller-model alpha-frontier 2>&1)"; rc=$?
check_exit "reviewing another author's work resolves" 0 "$rc"
check "the review routes away from the author" "PROVIDER=alpha" "$out"
check "a route landing on the caller is native" "DISPATCH=native" "$out"

# ...and that native route must survive an absent CLI, exactly as a self route does.
out="$("$DR" code_review "${ABS[@]}" --author-model beta-frontier --caller-model alpha-frontier 2>&1)"; rc=$?
check_exit "a native review route needs no CLI" 0 "$rc"
check "the native review route still resolves" "PROVIDER=alpha" "$out"

# With no caller declared, the session is assumed to be the catalog [lead].
out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier 2>&1)"; rc=$?
check_exit "an undeclared caller falls back to the lead" 0 "$rc"
check "the lead-assumed caller yields a native route" "DISPATCH=native" "$out"

# For a SELF role the author and the caller are the same session by definition, so
# declaring them as different providers is contradictory input, not a preference.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --caller-model beta-frontier 2>&1)"; rc=$?
check_exit "a self role refuses a caller that is not the author" 2 "$rc"
check "the self mismatch error names both providers" "beta" "$out"

# ...and a declared caller must not paper over a broken author identity either.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --author-model beta-frontier --caller-model alpha-frontier 2>&1)"; rc=$?
check_exit "a declared caller does not rescue a conflicting author" 2 "$rc"

# Agreeing on the same provider is fine, including via different tiers of it.
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier --caller-model alpha-strong 2>&1)"; rc=$?
check_exit "a self role accepts a caller that is the author" 0 "$rc"
check "the agreeing self role resolves to that provider" "PROVIDER=alpha" "$out"

# Bad identity input is a usage error on EVERY path. --vendors reporting it as exit 3
# would read as "no route available", which is the degraded state callers are told to
# accept by saying the check did not run, so a typo would silently skip a review.
out="$("$DR" --vendors small_impl "${SELF[@]}" --author-model alpha-frontier --caller-model beta-frontier 2>&1)"; rc=$?
check_exit "--vendors reports a caller/author mismatch as a usage error" 2 "$rc"
check "--vendors explains the mismatch" "not the declared author" "$out"
out="$("$DR" --vendors small_impl "${SELF[@]}" --author-model alpha-frontier --author-model beta-frontier 2>&1)"; rc=$?
check_exit "--vendors reports a conflicting author as a usage error" 2 "$rc"
out="$("$DR" --vendors small_impl "${DUP[@]}" --author-model shared-weights-1 2>&1)"; rc=$?
check_exit "--vendors reports an ambiguous author as a usage error" 2 "$rc"
check "--vendors offers the disambiguating flag" "--author-provider" "$out"

# The caller is one process, so its identity is unique-or-refused like the author's.
out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier --caller-provider alpha --caller-provider beta 2>&1)"; rc=$?
check_exit "two caller providers cannot identify one session" 2 "$rc"
out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier --caller-model nosuch-model 2>&1)"; rc=$?
check_exit "an unknown caller model is a usage error" 2 "$rc"
check "the unknown caller model error offers the provider flag" "--caller-provider" "$out"

# A wrong --caller-model must not refuse a session whose provider is still knowable:
# a co-declared provider wins outright, and a model id sharing a catalog family
# (grok-4.6 beside grok-4.5) retiers onto that provider with a warning instead of
# refusing. Only a model with no provider signal at all stays a usage error.
out="$("$DR" code_review "${SELF[@]}" --author-model beta-frontier --caller-model nosuch-model --caller-provider alpha 2>&1)"; rc=$?
check_exit "a declared caller provider rescues an unknown caller model" 0 "$rc"
check "the rescued route still resolves" "PROVIDER=alpha" "$out"
check "the rescue is reported" "not in the catalog" "$out"
out="$("$DR" static_own "${SELF[@]}" --caller-model beta-99 2>&1)"; rc=$?
check_exit "an unknown caller model with a unique catalog family resolves" 0 "$rc"
check "the family fallback is reported" "by model family" "$out"
check "the family fallback names the running session's provider" "DISPATCH=cli" "$out"

# Declaring the caller must not touch the exclusion set: it says where work RUNS, not
# who wrote the artifact, so it can never make a review look independent.
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier --caller-model alpha-frontier 2>&1)"; rc=$?
check_exit "caller identity does not satisfy independence" 0 "$rc"
check "the author is still excluded when it is also the caller" "PROVIDER=beta" "$out"
check "a route away from the caller is a cli dispatch" "DISPATCH=cli" "$out"
check "only the author appears in the receipt provenance" "AUTHOR_VENDORS=acme" "$out"

# The absent-CLI matrix covers normal resolution of a STATIC native route too, not
# just the sentinel, so a command -v ordering regression cannot hide behind `sh`.
out="$("$DR" static_own "${ABS[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "a static native route resolves with an absent CLI" 0 "$rc"
check "the static native route picks the caller" "PROVIDER=alpha" "$out"
check "the static native route is marked native" "DISPATCH=native" "$out"

# --vendors is the probe callers use to decide whether a review can happen at all, so
# it has to see the same routes resolution does. A native route that resolves must not
# vanish here just because the provider's CLI is missing.
out="$("$DR" --vendors small_impl "${ABS[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "--vendors sees a native route with an absent CLI" 0 "$rc"
check "--vendors reports the native route's vendor" "acme" "$out"
out="$("$DR" --vendors static_own "${ABS[@]}" --author-model alpha-frontier 2>&1)"; rc=$?
check_exit "--vendors sees a static native route with an absent CLI" 0 "$rc"
check "--vendors reports the static native route's vendor" "acme" "$out"

# ...and still drops a candidate that genuinely needs the missing CLI. --vendors is
# author-agnostic by design (the COUNT is what says whether independence is
# achievable), so the caller's own vendor is listed and the cross-runtime one is not:
# one vendor, which correctly reports that no independent review can resolve here.
out="$("$DR" --vendors code_review "${ABS[@]}" --author-model alpha-frontier 2>&1)"
check "--vendors keeps the native candidate" "acme" "$out"
case "$out" in
  *globex*) fail=$((fail + 1)); echo "  FAIL --vendors must drop a cli candidate whose CLI is absent" ;;
  *) pass=$((pass + 1)) ;;
esac
n="$(printf '%s\n' "$out" | grep -c .)"
check_exit "absent cross-runtime CLIs leave too few vendors for independence" 1 "$n"

# `self` is a sentinel, but a config that already had a provider by that name keeps
# its meaning: reinterpreting an existing route would silently change where it goes.
cat > "$TMP/provider-named-self.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.self]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.self.tiers]
strong   = "self-strong"
frontier = "self-frontier"
[providers.other]
vendor  = "globex"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.other.tiers]
strong   = "other-strong"
frontier = "other-frontier"
[defaults]
floor = "strong:low"
[roles]
small_impl = "self"
[role_tiers]
small_impl = "strong"
[role_efforts]
small_impl = "high"
EOF
PNS=(--config "$TMP/provider-named-self.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" small_impl "${PNS[@]}" --author-model other-frontier 2>&1)"; rc=$?
check_exit "a provider actually named self still routes" 0 "$rc"
check "the provider named self wins over the sentinel" "MODEL=self-strong" "$out"
check "that route crosses runtimes like any other" "DISPATCH=cli" "$out"
out="$("$DR" --check "${PNS[@]}" 2>&1)"; rc=$?
check_exit "--check reports the shadowed self sentinel" 1 "$rc"
check "--check says the sentinel is unavailable" "unavailable" "$out"
# ...and says nothing else. A shadowed route is an ordinary provider route, so it
# needs no [lead] fallback and none of the sentinel-only diagnostics apply.
case "$out" in
  *"no [lead] provider"*) fail=$((fail + 1)); echo "  FAIL a shadowed route must not require a [lead] fallback" ;;
  *) pass=$((pass + 1)) ;;
esac
case "$out" in
  *"different-vendor reviewer"*) fail=$((fail + 1)); echo "  FAIL a shadowed route must not report the sentinel's missing-independence note" ;;
  *) pass=$((pass + 1)) ;;
esac

# The shadow has to hold on EVERY path, not just normal resolution. This config has no
# [independence] table and never uses the sentinel (the value names a real provider),
# so it is a legacy table: author exclusion stays unconditional.
out="$("$DR" small_impl "${PNS[@]}" --author-vendor acme 2>&1)"; rc=$?
check_exit "a shadowed self config keeps unconditional author exclusion" 3 "$rc"

# --vendors must read the same route as resolution, not substitute the caller.
out="$("$DR" --vendors small_impl "${PNS[@]}" --author-model other-frontier 2>&1)"; rc=$?
check_exit "--vendors resolves a shadowed self route" 0 "$rc"
check "--vendors reports the shadowed provider's vendor" "acme" "$out"
n="$(printf '%s\n' "$out" | grep -c .)"
check_exit "--vendors does not add the caller to a shadowed route" 1 "$n"

# An explicit [independence] entry on a shadowed route is a normal cross-vendor
# review, not the refused self-plus-independence combination.
sed 's/^\[roles\]$/[independence]\nsmall_impl = "author_vendor"\n[roles]/' "$TMP/provider-named-self.toml" > "$TMP/shadow-independence.toml"
out="$("$DR" small_impl --config "$TMP/shadow-independence.toml" --models "$TMP/empty-catalog.toml" --author-vendor globex 2>&1)"; rc=$?
check_exit "a shadowed route with an independence entry still resolves" 0 "$rc"
check "the shadowed independence route routes away from the author" "PROVIDER=self" "$out"

# A shadowed route is an ordinary provider route, so it may declare fallbacks like
# any other. Only the sentinel forbids them, and that diagnostic must not reach here.
sed 's/^\[role_tiers\]$/[fallbacks]\nsmall_impl = ["self", "other"]\n[role_tiers]/' "$TMP/provider-named-self.toml" > "$TMP/shadow-fallback.toml"
out="$("$DR" --check --config "$TMP/shadow-fallback.toml" --models "$TMP/empty-catalog.toml" 2>&1)"
case "$out" in
  *"cannot also declare [fallbacks]"*) fail=$((fail + 1)); echo "  FAIL a shadowed route may declare fallbacks" ;;
  *) pass=$((pass + 1)) ;;
esac
check "the shadowed fixture still reports the unavailable sentinel" "unavailable" "$out"

# The backend-ambiguity diagnostic is sentinel-only too: a duplicated model under one
# vendor cannot confuse a route that names its provider outright.
sed 's/^frontier = "other-frontier"$/frontier = "self-frontier"/' "$TMP/provider-named-self.toml" > "$TMP/shadow-dup.toml"
out="$("$DR" --check --config "$TMP/shadow-dup.toml" --models "$TMP/empty-catalog.toml" 2>&1)"
case "$out" in
  *"cannot pick a backend"*) fail=$((fail + 1)); echo "  FAIL a shadowed route is never backend-ambiguous" ;;
  *) pass=$((pass + 1)) ;;
esac
out="$("$DR" small_impl --config "$TMP/shadow-fallback.toml" --models "$TMP/empty-catalog.toml" --author-model other-frontier 2>&1)"; rc=$?
check_exit "a shadowed route with fallbacks resolves" 0 "$rc"

# ...and --check must not call that combination a self-plus-independence conflict:
# the role routes to an ordinary provider that happens to be named `self`.
out="$("$DR" --check --config "$TMP/shadow-independence.toml" --models "$TMP/empty-catalog.toml" 2>&1)"
case "$out" in
  *"'self'-routed and [independence]-bound"*) fail=$((fail + 1)); echo "  FAIL a shadowed route with [independence] is not a self conflict" ;;
  *) pass=$((pass + 1)) ;;
esac

# Using the `self` sentinel proves a config is not a legacy table, so a missing
# [independence] section there means what it says: no role declares independence.
# The routing is explicit and is honored, but --check says plainly that no role in
# this config can require a different-vendor reviewer.
cat > "$TMP/legacy-self.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[lead]
provider = "alpha"
tier     = "frontier"
[providers.alpha]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.alpha.tiers]
strong   = "alpha-strong"
frontier = "alpha-frontier"
[defaults]
floor = "strong:low"
[roles]
code_review = "self"
[role_tiers]
code_review = "strong"
[role_efforts]
code_review = "high"
EOF
LS=(--config "$TMP/legacy-self.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" code_review "${LS[@]}" --author-vendor acme 2>&1)"; rc=$?
check_exit "an explicit self route is honored without an [independence] table" 0 "$rc"
check "the explicit self route stays in-vendor" "PROVIDER=alpha" "$out"
out="$("$DR" --check "${LS[@]}" 2>&1)"; rc=$?
check_exit "--check flags self-routing with no [independence] table" 1 "$rc"
check "--check explains what that config cannot require" "different-vendor" "$out"

# A config that never uses the sentinel IS a legacy table, and there a missing
# [independence] section keeps author exclusion unconditional.
sed 's/^code_review = "self"$/code_review = "alpha"/' "$TMP/legacy-self.toml" > "$TMP/legacy-plain.toml"
out="$("$DR" code_review --config "$TMP/legacy-plain.toml" --models "$TMP/empty-catalog.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "a legacy table still excludes its author unconditionally" 3 "$rc"

# --- route provenance, loop agreement, encoding, independence fragility ----------
# `native` asserts the resolved provider IS the caller. When no caller was declared
# that assertion rests on the [lead] assumption, and a session that is NOT the lead
# would run its own subagent against another vendor's model believing it was home.
# The route has to say which of the two it is.
out="$("$DR" small_impl "${SELF[@]}" --caller-model alpha-frontier 2>&1)"
check "a declared caller is reported as declared" "CALLER=declared" "$out"
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "a self role's author counts as a declared caller" "CALLER=declared" "$out"
out="$("$DR" small_impl "${SELF[@]}" 2>&1)"
check "an undeclared caller is reported as assumed" "CALLER=assumed-lead" "$out"
check "the assumed route still resolves" "PROVIDER=alpha" "$out"
err="$("$DR" small_impl "${SELF[@]}" 2>&1 >/dev/null)"
check "an assumed native route says so on stderr" "no caller declared" "$err"

# A cross-runtime route is not affected by the assumption either way.
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier --caller-model alpha-frontier 2>&1)"
check "a cli route still reports caller provenance" "CALLER=declared" "$out"

# THE invariant the two candidate loops kept breaking: --vendors must list the vendor
# that the same role actually resolves to. Every past divergence (self target, the
# CLI gate, self_target status) violated exactly this and no test caught it.
for probe in "small_impl --author-model alpha-frontier" \
             "small_impl --author-model beta-frontier" \
             "small_impl" \
             "static_own --author-model alpha-frontier" \
             "code_review --author-model alpha-frontier"; do
  # shellcheck disable=SC2086
  r_out="$("$DR" $probe "${SELF[@]}" 2>/dev/null)"; r_rc=$?
  # Every probe here is expected to resolve; skipping a failure would let the
  # agreement claim pass while resolution was broken.
  check_exit "agreement probe resolves: $probe" 0 "$r_rc"
  r_vendor="$(printf '%s\n' "$r_out" | sed -n 's/^VENDOR=//p')"
  # shellcheck disable=SC2086
  v_out="$("$DR" --vendors $probe "${SELF[@]}" 2>/dev/null)"
  if printf '%s\n' "$v_out" | grep -qFx -- "$r_vendor"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL --vendors omits the vendor "%s" resolves to (%s)\n' "$probe" "$r_vendor"
  fi
done

# Same invariant where the CLI is absent, which is where the loops last diverged.
for probe in "small_impl --author-model alpha-frontier" "static_own --author-model alpha-frontier"; do
  # shellcheck disable=SC2086
  r_vendor="$("$DR" $probe "${ABS[@]}" 2>/dev/null | sed -n 's/^VENDOR=//p')"
  # shellcheck disable=SC2086
  v_out="$("$DR" --vendors $probe "${ABS[@]}" 2>/dev/null)"
  if [ -n "$r_vendor" ] && printf '%s\n' "$v_out" | grep -qFx -- "$r_vendor"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1)); printf '  FAIL --vendors disagrees with resolution for "%s" on an absent CLI\n' "$probe"
  fi
done

# Author provenance is emitted as repeated fields, so a consumer never parses a
# delimiter. The joined field stays for compatibility.
out="$("$DR" judge "${SELF[@]}" --author-model alpha-frontier --author-model beta-frontier 2>&1)"; rc=$?
if [ "$rc" -eq 0 ]; then
  n="$(printf '%s\n' "$out" | grep -c '^AUTHOR_VENDOR=')"
  check_exit "each author vendor is its own field" 2 "$n"
  check "the joined field remains for compatibility" "AUTHOR_VENDORS=" "$out"
else
  pass=$((pass + 2))   # no third vendor to judge with here; covered on the shipped catalog
fi
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "a single author is a repeated field too" "AUTHOR_VENDOR=acme" "$out"

# Independence that hangs on ONE reachable alternate is a single point of failure.
# Say so on the route, where a caller sees it, not only in a probe nobody runs.
out="$("$DR" code_review "${SELF[@]}" --author-model alpha-frontier 2>&1)"
check "an independence route reports its alternate count" "ALTERNATES=1" "$out"
out="$("$DR" small_impl "${SELF[@]}" --author-model alpha-frontier 2>&1)"
case "$out" in
  *ALTERNATES=*) fail=$((fail + 1)); echo "  FAIL a non-independence role has no alternates to report" ;;
  *) pass=$((pass + 1)) ;;
esac

# A vendor that is reachable but cannot SERVE the role is not an alternate. Counting
# it would report a spare route that does not exist, which is the one thing this
# number must never do. `gamma` is reachable and a distinct vendor, but the role needs
# a capability it does not declare.
cat > "$TMP/alternates.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.author]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
capabilities = ["code", "vision"]
[providers.author.tiers]
frontier = "author-1"
[providers.real]
vendor  = "globex"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
capabilities = ["code", "vision"]
[providers.real.tiers]
frontier = "real-1"
[providers.gamma]
vendor  = "initech"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
capabilities = ["code"]
[providers.gamma.tiers]
frontier = "gamma-1"
[defaults]
floor = "strong:low"
[requires]
visual_verify = ["vision"]
[roles]
visual_verify = "author"
# The INELIGIBLE candidate precedes the valid one on purpose: with the good provider
# first, resolution never evaluates the rejection and the agreement claim is vacuous.
[fallbacks]
visual_verify = ["author", "gamma", "real"]
[role_tiers]
visual_verify = "frontier"
[role_efforts]
visual_verify = "high"
[independence]
visual_verify = "author_vendor"
EOF
out="$("$DR" visual_verify --config "$TMP/alternates.toml" --models "$TMP/empty-catalog.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "the capability-filtered role resolves" 0 "$rc"
check "it routes to the capable alternate" "PROVIDER=real" "$out"
check "alternates exclude a vendor that cannot serve the role" "ALTERNATES=1" "$out"

# Resolution keeps its own eligibility block because its failures are per-check fatal
# or skip, which the probes deliberately flatten to "unusable". That difference is
# intentional, but it is also a drift boundary: a rule added to one and not the others
# would let --vendors and ALTERNATES promise a route resolution rejects. Pin the
# agreement instead of assuming it.
ALT=(--config "$TMP/alternates.toml" --models "$TMP/empty-catalog.toml")
v_out="$("$DR" --vendors visual_verify "${ALT[@]}" --author-vendor acme 2>/dev/null)"
case "$v_out" in
  *initech*) fail=$((fail + 1)); echo "  FAIL --vendors counts a vendor that cannot serve the role" ;;
  *) pass=$((pass + 1)) ;;
esac
check "--vendors keeps the vendor that can serve it" "globex" "$v_out"

# Same agreement for a TIER mismatch rather than a capability one: gamma declares the
# capability but has no model at the tier the role requires.
sed 's/^capabilities = \["code"\]$/capabilities = ["code", "vision"]/' "$TMP/alternates.toml" > "$TMP/alternates-tier.toml"
python3 - "$TMP/alternates-tier.toml" <<'PY'
import sys
p = sys.argv[1]; s = open(p).read()
s = s.replace('[providers.gamma.tiers]\nfrontier = "gamma-1"',
              '[providers.gamma.tiers]\nstrong = "gamma-1"')
open(p, 'w').write(s)
PY
FLOOR_CFG=(--config "$TMP/alternates-tier.toml" --models "$TMP/empty-catalog.toml")
# KNOWN DIVERGENCE, pinned rather than assumed away. A chain candidate that has no
# model at the role's tier is FATAL in resolution (a precise config error) but merely
# unreachable to --vendors, which skips it. So the probe can report two vendors on a
# config where resolution exits 2, meaning it promises a route that cannot be taken.
# Both behaviours predate this change; this asserts what they actually are so the
# inconsistency is visible and cannot drift further unnoticed.
r_out="$("$DR" visual_verify "${FLOOR_CFG[@]}" --author-vendor acme 2>&1)"; r_rc=$?
check_exit "a tier-less chain candidate is fatal in resolution" 2 "$r_rc"
check "the fatal names the tier it cannot serve" "no model mapping" "$r_out"
v_out="$("$DR" --vendors visual_verify "${FLOOR_CFG[@]}" --author-vendor acme 2>/dev/null)"
case "$v_out" in
  *initech*) fail=$((fail + 1)); echo "  FAIL --vendors counts a tier-ineligible vendor" ;;
  *) pass=$((pass + 1)) ;;
esac
check "--vendors still reports the vendor that can serve the role" "globex" "$v_out"

# A routed provider with no section is a broken table regardless of who the author is,
# so exclusion must not be able to turn that fatal into a silent skip.
cat > "$TMP/missing-excluded.toml" <<'EOF'
[tiers]
scale = ["strong", "frontier"]
[efforts]
scale = ["low", "high"]
[providers.real]
vendor  = "globex"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["low", "high"]
default_tier = "frontier"
[providers.real.tiers]
frontier = "real-1"
[defaults]
floor = "strong:low"
[roles]
code_review = "ghost"
[fallbacks]
code_review = ["ghost", "real"]
[role_tiers]
code_review = "frontier"
[role_efforts]
code_review = "high"
[independence]
code_review = "author_vendor"
EOF
out="$("$DR" code_review --config "$TMP/missing-excluded.toml" --models "$TMP/empty-catalog.toml" --author-vendor globex --exclude ghost 2>&1)"; rc=$?
check_exit "an excluded routed provider with no section is still fatal" 2 "$rc"
check "the missing-section error names the provider" "ghost" "$out"

echo "== membership tests =="

# A value that IS in a scale must never read as absent, whatever the scheduler does.
#
# The old spelling was `printf '%s\n' "$list" | grep -qx -- "$value"`. `grep -q`
# exits at the first match, so everything after the match stays unwritten, the
# writer takes SIGPIPE, and `set -o pipefail` promotes that 141 to the pipeline's
# status: a HIT reported as a miss. It needed the reader to win a scheduling race,
# so it surfaced as a rare red in a loaded validate.sh run and never on a serial
# re-run, and it hit resolution as well as --check.
#
# This fixture deletes the race rather than hoping for it: the value is the FIRST
# entry and the rest of the scale is bigger than a pipe buffer, so the writer
# cannot possibly be finished when the reader leaves. Against the old spelling
# every assertion below fails on every run.
# Only the LENGTH matters here: 1200 entries of this width put well over a pipe
# buffer behind the matching first entry.
pad_word="$(printf '%090d' 0)"
padding=""
pad_i=0
while [ "$pad_i" -lt 1200 ]; do
  padding="$padding, \"pad-$pad_i-$pad_word\""
  pad_i=$((pad_i + 1))
done
cat > "$TMP/longscale.toml" <<EOF
[tiers]
scale = ["strong"]
[efforts]
scale = ["high"$padding]
[providers.alpha]
vendor  = "acme"
binary  = "sh"
channel = "cli"
effort  = "high"
efforts = ["high"]
default_tier = "strong"
[providers.alpha.tiers]
strong = "alpha-1"
[roles]
code_review = "alpha"
[role_tiers]
code_review = "strong"
[role_efforts]
code_review = "high"
EOF
LONG=(--config "$TMP/longscale.toml" --models "$TMP/empty-catalog.toml")
out="$("$DR" --check "${LONG[@]}" 2>&1)"; rc=$?
check_exit "--check accepts an effort early in a long [efforts] scale" 0 "$rc"
check "--check reports no phantom scale finding" "OK (" "$out"
out="$("$DR" code_review "${LONG[@]}" 2>&1)"; rc=$?
check_exit "resolution accepts an effort early in a long [efforts] scale" 0 "$rc"
check "the resolved route keeps that effort" "EFFORT=high" "$out"

# The same --check validate.sh runs, several at once against the shipped files.
# The defect above was first seen as one red among concurrently executing suites,
# so pin that concurrency itself is survivable instead of only the serial path.
#
# This one is a smoke, not the regression: against the defective spelling it fires
# only when the machine is loaded enough to lose the race, which is exactly the
# property that made the bug hard to see. The fixture above is what pins it.
conc_dir="$TMP/concurrent"
mkdir -p "$conc_dir"
conc_n=12
conc_i=1
while [ "$conc_i" -le "$conc_n" ]; do
  (
    DELEGATES_TOML="$HERE/../../delegates.toml" MODELS_TOML="$HERE/../../../../models.toml" \
      "$DR" --check >"$conc_dir/out.$conc_i" 2>&1 ||
      printf 'run %s exit %s\n' "$conc_i" "$?" > "$conc_dir/bad.$conc_i"
  ) &
  conc_i=$((conc_i + 1))
done
wait
conc_bad="$(find "$conc_dir" -name 'bad.*' | wc -l)"
if [ "$conc_bad" -eq 0 ]; then
  pass=$((pass + 1))
else
  fail=$((fail + 1))
  printf '  FAIL %s of %s concurrent --check runs failed\n' "$conc_bad" "$conc_n"
  cat "$conc_dir"/bad.* "$conc_dir"/out.* 2>/dev/null | sed 's/^/    /' | sort -u
fi

echo "== context-separation fallback tests =="

# Two vendors, both reachable, so the cross-vendor path is the one that has to keep
# winning when it can. `sh` exists everywhere, which is what makes both installed.
cat > "$TMP/cs.toml" <<'EOF'
[providers.alpha]
model   = "alpha-1"
vendor  = "acme"
binary  = "sh"
channel = "cli"
[providers.beta]
model   = "beta-1"
vendor  = "globex"
binary  = "sh"
channel = "cli"
[roles]
code_review = "beta"
judge       = "beta"
small_impl  = "alpha"
[fallbacks]
code_review = ["beta", "alpha"]
judge       = ["beta", "alpha"]
[independence]
code_review = "author_vendor"
judge       = "all_author_vendors"
EOF

out="$("$DR" code_review --config "$TMP/cs.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "cross-vendor route still resolves" 0 "$rc"
check "a cross-vendor route labels itself" "INDEPENDENCE=cross-vendor" "$out"
check "cross-vendor picks the non-author vendor" "VENDOR=globex" "$out"

# The flag must not change a route that never needed it: opting in is not the same
# as degrading, and a caller that always passes it must still get the strong tier.
out="$("$DR" code_review --config "$TMP/cs.toml" --author-vendor acme --allow-context-separation 2>&1)"; rc=$?
check_exit "the flag alone does not degrade an available cross-vendor route" 0 "$rc"
check "the flag alone keeps the cross-vendor label" "INDEPENDENCE=cross-vendor" "$out"

# Default behavior is unchanged: no flag, no route, no review.
out="$("$DR" code_review --config "$TMP/cs.toml" --author-vendor acme --exclude beta 2>&1)"; rc=$?
check_exit "without the flag an unreachable cross-vendor route still exits 3" 3 "$rc"

out="$("$DR" code_review --config "$TMP/cs.toml" --author-vendor acme --exclude beta --allow-context-separation 2>&1)"; rc=$?
check_exit "the degraded tier resolves when asked" 0 "$rc"
check "the degraded route is labeled" "INDEPENDENCE=context-separation" "$out"
check "the degraded route lands on the author's own vendor" "VENDOR=acme" "$out"
check "the degraded route says so on stderr" "Do not report this as a cross-vendor check" "$out"
# The count a caller watches has to keep meaning "vendors that could take over",
# which on a degraded route is zero. Counting the suspended author would report
# independence as still available at the exact moment it stopped being.
check "the degraded route reports no alternates" "ALTERNATES=0" "$out"

# --exclude is the caller's own policy, not an independence rule, so the retry
# must not resurrect a backend the caller ruled out. With alpha excluded too there
# is nothing left to fall back to.
out="$("$DR" code_review --config "$TMP/cs.toml" --author-vendor acme --exclude beta --exclude alpha --allow-context-separation 2>&1)"; rc=$?
check_exit "the retry does not suspend an explicit --exclude" 3 "$rc"

# Blind ranking fails on self-preference, which a fresh session does not repair.
out="$("$DR" judge --config "$TMP/cs.toml" --author-vendor acme --author-vendor globex --allow-context-separation 2>&1)"; rc=$?
check_exit "an all_author_vendors role refuses the degraded tier" 3 "$rc"
check "and says why" "cannot degrade" "$out"

# A role that claims no independence must not claim a separation either.
out="$("$DR" small_impl --config "$TMP/cs.toml" --author-vendor acme 2>&1)"; rc=$?
check_exit "a non-independence role resolves" 0 "$rc"
if printf '%s' "$out" | grep -q 'INDEPENDENCE='; then
  fail=$((fail+1)); printf '  FAIL a role with no [independence] policy must not emit INDEPENDENCE\n    got: %s\n' "$out"
else
  pass=$((pass+1))
fi

# The shipped config is the one that matters: a Claude-authored diff whose Codex
# route is gone must still reach the proven condition rather than nothing.
# Excluding codex alone no longer reaches the degraded path, which is the 2026-08-10
# change working: qwen and moonshot are real cross-vendor routes behind it. Every
# alternate has to be gone before context separation is the honest answer.
out="$(DELEGATES_TOML="$HERE/../../delegates.toml" MODELS_TOML="$HERE/../../../../models.toml" \
  "$DR" code_review --author-vendor anthropic --exclude codex --allow-context-separation 2>&1)"; rc=$?
check_exit "one vendor down still leaves a real cross-vendor route" 0 "$rc"
if printf '%s' "$out" | grep -q 'INDEPENDENCE=context-separation'; then
  fail=$((fail+1)); printf '  FAIL losing one vendor must not degrade while alternates remain\n    got: %s\n' "$out"
else
  pass=$((pass+1))
fi

out="$(DELEGATES_TOML="$HERE/../../delegates.toml" MODELS_TOML="$HERE/../../../../models.toml" \
  "$DR" code_review --author-vendor anthropic --exclude codex --exclude qwen --exclude moonshot \
  --exclude deepseek --allow-context-separation 2>&1)"; rc=$?
check_exit "the shipped config degrades rather than skipping review" 0 "$rc"
check "the shipped degraded route is labeled" "INDEPENDENCE=context-separation" "$out"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
