#!/usr/bin/env bash
# Payload-size regression for the megapowers session-start hook. The SessionStart
# injection must stay a lean nudge, not the whole skill re-pasted into every session
# (and again after every /clear and compaction). Assert the emitted additionalContext
# is under 350 words, that the dead-weight blocks are stripped (YAML frontmatter,
# whose name+description already appear in the skills listing, and the "## Platform
# Adaptation" section, which points other harnesses at their own reference files),
# and that the workflow-critical Core Rule survives. Ceiling 350: measured 318 word
# payload (base note plus the model-catalog block) plus about 10% headroom; the
# block's byte budget is separately capped at 600B by validate.sh and the renderer
# test. The communication-register rules must also survive (condensed in SKILL.md,
# not stripped
# by this hook): the maintainer requires them in every session. SKILL.md on disk stays
# complete and portable.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../session-start"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

pass=0; fail=0
out="$(bash "$HOOK" 2>/dev/null)"
ctx="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.additionalContext' 2>/dev/null || true)"
words="$(printf '%s' "$ctx" | wc -w | tr -d ' ')"

if [ -n "$ctx" ] && [ "$ctx" != "null" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL hook emitted no additionalContext\n'; fi
if [ "${words:-0}" -le 350 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL emitted payload %s words, want <=350\n' "$words"; fi
if printf '%s' "$ctx" | grep -q 'description:'; then fail=$((fail + 1)); printf '  FAIL YAML frontmatter leaked into payload\n'; else pass=$((pass + 1)); fi
if printf '%s' "$ctx" | grep -q 'Platform Adaptation'; then fail=$((fail + 1)); printf '  FAIL Platform Adaptation section not stripped\n'; else pass=$((pass + 1)); fi
if printf '%s' "$ctx" | grep -q 'Core Rule'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL workflow-critical Core Rule missing from payload\n'; fi
if printf '%s' "$ctx" | grep -qi 'no dash punctuation'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL communication-register no-dash-punctuation rule missing from payload\n'; fi
if printf '%s' "$ctx" | grep -qi 'for takeover'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL communication-register takeover rule missing from payload\n'; fi

# Catalog block present when a catalog resolves (MODELS_TOML is honored through
# render-model-catalog, which session-start shells).
CAT_TMP="$(mktemp -d)"
trap 'rm -rf "$CAT_TMP"' EXIT
cat > "$CAT_TMP/cat.toml" <<'EOF'
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
cat_out="$(MODELS_TOML="$CAT_TMP/cat.toml" bash "$HOOK" 2>/dev/null < /dev/null)"
if printf '%s' "$cat_out" | grep -q 'Model catalog'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL catalog block missing from payload\n'; fi

# Hostile fixture: a catalog value with a raw control byte (0x0B) must still
# produce valid JSON. escape_for_json does not handle bytes outside \, ", \n,
# \r, \t; render-model-catalog must strip them before session-start embeds them.
if command -v jq >/dev/null 2>&1; then
  HOSTILE_TMP="$(mktemp -d)"
  printf '[lead]\nprovider = "alpha"\ntier = "frontier"\n[tiers]\nscale = ["fast"]\n[tiers.use]\nfast = "ok"\n[providers.alpha]\nvendor = "acme"\ndefault_tier = "fast"\nuse = "leads"\n[providers.alpha.tiers]\nfast = "alpha-mini"\n[providers.beta]\nvendor = "bmax"\ndefault_tier = "fast"\nuse = "delegate\x0Bwith control byte"\n[providers.beta.tiers]\nfast = "beta-mini"\n[defaults]\nfloor = "fast:low"\n' > "$HOSTILE_TMP/hostile.toml"
  hostile_out="$(MODELS_TOML="$HOSTILE_TMP/hostile.toml" bash "$HOOK" 2>/dev/null < /dev/null)"
  if printf '%s' "$hostile_out" | jq -e . >/dev/null 2>&1; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL hostile control-byte catalog corrupted the JSON payload\n'; fi
  rm -rf "$HOSTILE_TMP"
fi

# Payload unchanged and hook still exits 0 when no catalog exists.
none_out="$(MODELS_TOML="/nonexistent/models.toml" bash "$HOOK" < /dev/null)"; none_rc=$?
if [ "$none_rc" -eq 0 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL hook must stay exit 0 without a catalog\n'; fi
if printf '%s' "$none_out" | grep -q 'Model catalog'; then fail=$((fail + 1)); printf '  FAIL no catalog must mean no block\n'; else pass=$((pass + 1)); fi
if printf '%s' "$none_out" | grep -q 'additionalContext'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL payload structure intact\n'; fi

echo "== session-start: $pass passed, $fail failed (payload ${words} words) =="
[ "$fail" -eq 0 ]
