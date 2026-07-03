#!/usr/bin/env bash
E="$ROOT/plugins/mega-orchestration/skills/effect-broker/scripts/effect-broker"
{
  for lvl in autonomous on-the-loop in-the-loop; do
    echo "=== reversible/$lvl ==="; "$E" reversible --level "$lvl"
    echo "=== staged/$lvl ==="; "$E" staged --level "$lvl"
    echo "=== irreversible/$lvl ==="; "$E" irreversible --level "$lvl"
  done
  echo "=== bad ==="; "$E" nope 2>&1; echo "rc=$?"
  echo "=== extra-arg ==="; "$E" reversible staged 2>&1; echo "rc=$?"
} > eb.out 2>&1
cat eb.out
