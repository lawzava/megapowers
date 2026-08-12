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
[efforts]
scale = ["low", "high"]
[efforts.use]
low  = "cheap steps"
high = "hard calls"
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
printf '%s' "$out" | grep -q "efforts: low=cheap steps | high=hard calls" && ok || no "efforts line rendered"
printf '%s' "$out" | grep -q "delegate-resolve <role>" && ok || no "route pointer rendered"

# --caller names the harness that is running, which is the one in charge. The
# catalog [lead] is only the default for a session that declares nothing, so a
# declared caller takes the lead line and [lead] is demoted to the fallback it
# actually is. Without this the block asserts "lead: alpha" to a beta session.
out="$(MODELS_TOML="$TMP/cat.toml" "$R" --caller beta)"; rc=$?
[ "$rc" -eq 0 ] && ok || no "declared-caller render exit 0"
printf '%s' "$out" | grep -q "^lead: beta, the harness running this session" && ok || no "declared caller takes the lead line"
printf '%s' "$out" | grep -q "alpha frontier (alpha-max)" && ok || no "catalog lead kept as the undeclared-session default"

# A caller that IS the catalog lead has nothing to correct: same block as before.
out="$(MODELS_TOML="$TMP/cat.toml" "$R" --caller alpha)"
printf '%s' "$out" | grep -qx "lead: alpha frontier (alpha-max)" && ok || no "caller matching the catalog lead renders the plain lead line"
if printf '%s' "$out" | grep -q "harness running this session"; then no "no correction when the caller is the catalog lead"; else ok; fi

# Fail-open extends to the flag: an empty or missing value renders the default
# block rather than erroring out of a session start.
out="$(MODELS_TOML="$TMP/cat.toml" "$R" --caller "")"; rc=$?
if [ "$rc" -eq 0 ] && printf '%s' "$out" | grep -qx "lead: alpha frontier (alpha-max)"; then ok; else no "empty caller falls back to the catalog lead line"; fi
out="$(MODELS_TOML="$TMP/cat.toml" "$R" --caller)"; rc=$?
if [ "$rc" -eq 0 ] && printf '%s' "$out" | grep -qx "lead: alpha frontier (alpha-max)"; then ok; else no "valueless caller flag falls back to the catalog lead line"; fi

out="$(MODELS_TOML="$TMP/does-not-exist.toml" "$R")"; rc=$?
if [ "$rc" -eq 0 ] && [ -z "$out" ]; then ok; else no "missing catalog is silent exit 0"; fi

# A defined empty array in a higher-priority layer replaces the shipped array.
# An empty tier scale therefore suppresses the catalog block instead of falling
# through to the shipped scale.
mkdir -p "$TMP/project/.megapowers" "$TMP/home" "$TMP/xdg"
cat > "$TMP/project/.megapowers/models.toml" <<'EOF'
[tiers]
scale = []
EOF
out="$(cd "$TMP/project" && HOME="$TMP/home" XDG_CONFIG_HOME="$TMP/xdg" "$R")"; rc=$?
if [ "$rc" -eq 0 ] && [ -z "$out" ]; then ok; else no "empty project array replaces shipped catalog array"; fi

printf 'not toml at all\n' > "$TMP/bad.toml"
out="$(MODELS_TOML="$TMP/bad.toml" "$R")"; rc=$?
if [ "$rc" -eq 0 ] && [ -z "$out" ]; then ok; else no "malformed catalog is silent exit 0"; fi

if [ -f "$SHIPPED" ]; then
  out="$(MODELS_TOML="$SHIPPED" "$R")"
  n="$(printf '%s' "$out" | LC_ALL=C wc -c | tr -d '[:space:]')"
  # Raised 900 -> 980 on 2026-08-10 when the catalog went from two vendors to four
  # (qwen and moonshot earned rows on the review eval). The hard clamp in the hook is
  # 1024B, so this guard stays 44B inside it: enough that an override layer adding one
  # provider still renders, tight enough that the next addition has to justify itself
  # by shortening something rather than by moving this number again.
  if [ -n "$out" ] && [ "$n" -le 980 ]; then ok; else no "shipped catalog renders non-empty and <= 980B (got ${n}B)"; fi
  # The declared-caller block carries one extra clause on the lead line. It is the
  # variant Codex and OpenCode sessions actually get, so it needs its own guard
  # inside the hook's 1200B clamp, not an assumption that the default one covers it.
  out="$(MODELS_TOML="$SHIPPED" "$R" --caller codex)"
  n="$(printf '%s' "$out" | LC_ALL=C wc -c | tr -d '[:space:]')"
  if [ -n "$out" ] && [ "$n" -le 1100 ]; then ok; else no "declared-caller catalog renders non-empty and <= 1100B (got ${n}B)"; fi
  printf '%s' "$out" | grep -q "^lead: codex, the harness running this session" && ok || no "shipped catalog honors a declared caller"
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
