#!/usr/bin/env bash
R="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/scripts/delegate-resolve"
cfg="$ROOT/plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml"
{
  echo "=== code_review ==="; "$R" code_review --config "$cfg"; echo "rc=$?"
  echo "=== visual ==="; "$R" visual --config "$cfg"; echo "rc=$?"
  echo "=== unknown ==="; "$R" bogus --config "$cfg" 2>&1; echo "rc=$?"
  echo "=== disabled ==="
  dt="$PWD/dt.toml"; sed 's/plan_review[[:space:]]*= "codex"/plan_review = "antigravity"/' "$cfg" > "$dt"
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
} > resolve.out 2>&1
cat resolve.out
