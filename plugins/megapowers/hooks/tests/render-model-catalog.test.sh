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

# Hostile fixture: a raw control byte (0x0B) in a catalog value must not reach
# the rendered output. escape_for_json in session-start only handles \, ", \n,
# \r, \t; anything else raw would corrupt the JSON payload downstream. Put the
# byte in a delegate's `use` value so it lands in del_lines, which is what
# actually reaches the SessionStart payload.
printf '[lead]\nprovider = "alpha"\ntier = "frontier"\n[tiers]\nscale = ["fast"]\n[tiers.use]\nfast = "ok"\n[providers.alpha]\nvendor = "acme"\ndefault_tier = "fast"\nuse = "leads"\n[providers.alpha.tiers]\nfast = "alpha-mini"\n[providers.beta]\nvendor = "bmax"\ndefault_tier = "fast"\nuse = "delegate\x0Bwith control byte"\n[providers.beta.tiers]\nfast = "beta-mini"\n[defaults]\nfloor = "fast:low"\n' > "$TMP/hostile.toml"
out="$(MODELS_TOML="$TMP/hostile.toml" "$R")"; rc=$?
[ "$rc" -eq 0 ] && ok || no "hostile control-byte fixture exits 0"
[ -n "$out" ] && ok || no "hostile control-byte fixture renders non-empty output"
stripped="$(printf '%s' "$out" | tr -d '\t\n\r')"
if printf '%s' "$stripped" | LC_ALL=C grep -q '[[:cntrl:]]'; then no "control byte leaked into rendered output"; else ok; fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
