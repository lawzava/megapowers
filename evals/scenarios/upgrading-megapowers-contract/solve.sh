#!/usr/bin/env bash
set -u

skill="$ROOT/plugins/megapowers/skills/upgrading-megapowers/SKILL.md"
reference="$ROOT/plugins/megapowers/skills/upgrading-megapowers/references/channels.md"
readme="$ROOT/plugins/megapowers/README.md"
setup="$ROOT/docs/setup.md"

[ -f "$skill" ] && cp "$skill" SKILL.md
[ -f "$reference" ] && cp "$reference" channels.md
cp "$readme" plugin-readme.md
cp "$setup" setup.md
