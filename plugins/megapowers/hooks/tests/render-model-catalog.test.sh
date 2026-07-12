#!/usr/bin/env bash
# Tests for render-model-catalog: fixture rendering, fail-open behavior, and a
# byte budget on the shipped catalog.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
R="$HERE/../render-model-catalog"
SHIPPED="$HERE/../../models.toml"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
pass=0; fail=0
ok() { pass=$((pass+1)); }
no() { fail=$((fail+1)); printf '  FAIL %s\n' "$1"; }

cat > "$TMP/cat.toml" <<'EOF'
[lead]
provider = "alpha"
tier     = "frontier"
[tiers]
scale = ["fast", "frontier"]
[tiers.use]
fast     = "cheap fan-out"
frontier = "lead and judge"
[providers.alpha]
vendor = "acme"
default_tier = "frontier"
use = "leads"
[providers.alpha.tiers]
frontier = "alpha-max"
fast     = "alpha-mini"
[providers.beta]
vendor = "bmax"
use = "review delegate"
default_tier = "frontier"
[providers.beta.tiers]
frontier = "beta-9"
[providers.off]
enabled = false
use = "never shown"
[defaults]
floor = "fast:low"
EOF
out="$(MODELS_TOML="$TMP/cat.toml" "$R")"; rc=$?
[ "$rc" -eq 0 ] && ok || no "fixture render exit 0"
printf '%s' "$out" | grep -q "lead: alpha frontier (alpha-max)" && ok || no "lead line"
printf '%s' "$out" | grep -q "fast=alpha-mini (cheap fan-out)" && ok || no "tier line with use hint"
printf '%s' "$out" | grep -q "beta=review delegate" && ok || no "delegate line"
if printf '%s' "$out" | grep -q "off="; then no "disabled provider leaked into block"; else ok; fi
printf '%s' "$out" | grep -q "floor fast:low" && ok || no "floor rendered"
printf '%s' "$out" | grep -q "delegate-resolve <role>" && ok || no "route pointer rendered"

out="$(MODELS_TOML="$TMP/does-not-exist.toml" "$R")"; rc=$?
if [ "$rc" -eq 0 ] && [ -z "$out" ]; then ok; else no "missing catalog is silent exit 0"; fi

printf 'not toml at all\n' > "$TMP/bad.toml"
out="$(MODELS_TOML="$TMP/bad.toml" "$R")"; rc=$?
if [ "$rc" -eq 0 ] && [ -z "$out" ]; then ok; else no "malformed catalog is silent exit 0"; fi

if [ -f "$SHIPPED" ]; then
  out="$(MODELS_TOML="$SHIPPED" "$R")"
  n="$(printf '%s' "$out" | LC_ALL=C wc -c | tr -d '[:space:]')"
  if [ -n "$out" ] && [ "$n" -le 600 ]; then ok; else no "shipped catalog renders non-empty and <= 600B (got ${n}B)"; fi
else
  no "shipped models.toml missing at plugin root"
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
