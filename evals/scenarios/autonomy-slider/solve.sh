#!/usr/bin/env bash
A="$ROOT/plugins/mega-orchestration/skills/autonomous-run/scripts/autonomy-level"
{
  for l in autonomous on-the-loop in-the-loop; do echo "=== $l ==="; "$A" "$l"; done
  echo "=== bad ==="; "$A" nope 2>&1; echo "rc=$?"
} > out.txt 2>&1
cat out.txt
