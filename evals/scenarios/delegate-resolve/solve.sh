#!/usr/bin/env bash
R="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/scripts/delegate-resolve"
cfg="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml"

# PATH shims so binary-availability checks are deterministic regardless of host:
# codex/claude/playwright-cli (and one fixture binary) are "present" here; any
# made-up name that gets no shim stays "absent".
shims="$PWD/shims"; mkdir -p "$shims"
for b in codex claude playwright-cli mp_present_bin; do
  printf '#!/bin/sh\nexit 0\n' > "$shims/$b"; chmod +x "$shims/$b"
done
export PATH="$shims:$PATH"

# Hermetic: user-level megapowers config layers must not leak into fixtures.
export XDG_CONFIG_HOME="$PWD/xdg-isolated"
mkdir -p "$XDG_CONFIG_HOME"

{
  # --- existing contract ---
  echo "=== code_review ==="; "$R" code_review --config "$cfg"; echo "rc=$?"
  echo "=== visual ==="; "$R" visual --config "$cfg"; echo "rc=$?"
  echo "=== browser_test ==="; "$R" browser_test --config "$cfg"; echo "rc=$?"
  echo "=== visual_verify ==="; "$R" visual_verify --config "$cfg"; echo "rc=$?"
  echo "=== unknown ==="; "$R" bogus --config "$cfg" 2>&1; echo "rc=$?"
  echo "=== disabled ==="
  dt="$PWD/dt.toml"
  printf '[providers.offbox]\nmodel = "x"\nenabled = false\n[roles]\nplan_review = "offbox"\n' > "$dt"
  "$R" plan_review --config "$dt" 2>&1; echo "rc=$?"
  echo "=== hash-in-quoted-value ==="
  ht="$PWD/hash.toml"
  printf '[providers.codex]\nmodel = "gpt-5.5"\nnotes = "curl -H x#y keep this"\n[roles]\ncode_review = "codex"\n' > "$ht"
  "$R" code_review --config "$ht" 2>&1; echo "rc=$?"
  echo "=== missing-provider-section ==="
  gt="$PWD/ghost.toml"; printf '[providers.codex]\nmodel = "x"\n[roles]\nghost_role = "ghost"\n' > "$gt"
  "$R" ghost_role --config "$gt" 2>&1; echo "rc=$?"
  echo "=== bad-args (--config with no file) ==="
  "$R" code_review --config 2>&1; echo "rc=$?"

  # --- second vendor, fallbacks, --exclude, availability, presets, parse error ---
  echo "=== verify-primary ==="; "$R" verify --config "$cfg"; echo "rc=$?"
  echo "=== verify-exclude-anthropic ==="; "$R" verify --config "$cfg" --exclude anthropic; echo "rc=$?"
  echo "=== verify-exclude-both ==="; "$R" verify --config "$cfg" --exclude openai --exclude anthropic 2>&1; echo "rc=$?"
  echo "=== fallback-skip-absent ==="
  av="$PWD/av.toml"
  printf '[providers.p_absent]\nmodel = "x"\nvendor = "va"\nbinary = "mp_absent_bin"\n[providers.p_present]\nmodel = "y"\nvendor = "vb"\nbinary = "mp_present_bin"\n[roles]\nmyrole = "p_absent"\n[fallbacks]\nmyrole = ["p_absent", "p_present"]\n' > "$av"
  "$R" myrole --config "$av"; echo "rc=$?"
  echo "=== no-available-route ==="
  "$R" myrole --config "$av" --exclude vb 2>&1; echo "rc=$?"
  echo "=== preset ==="; "$R" --preset read_only --config "$cfg"; echo "rc=$?"
  echo "=== parse-error ==="
  bt="$PWD/bad.toml"; printf '[roles]\nthis is not valid toml\n' > "$bt"
  "$R" code_review --config "$bt" 2>&1; echo "rc=$?"
} > resolve.out 2>&1
cat resolve.out
