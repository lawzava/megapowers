#!/usr/bin/env bash
S="$ROOT/plugins/megapowers/skills/project-memory/scripts"
export MEGAPOWERS_MEMORY_DIR="$PWD/.megapowers/memory"
{
  echo "=== add ==="; printf 'chose sqlite over postgres: no managed DB on target\n' | "$S/mem-add" db --title "DB choice" --hook "why sqlite" --type decision; echo "rc=$?"
  printf 'x\n' | "$S/mem-add" other --title "Other" --hook "second memory" >/dev/null
  echo "=== index-has-both ==="; grep -c '^- \[' "$MEGAPOWERS_MEMORY_DIR/INDEX.md"
  echo "=== dup ==="; printf 'y\n' | "$S/mem-add" db --title t --hook h 2>&1; echo "rc=$?"
  echo "=== recall ==="; "$S/mem-recall" sqlite
  echo "=== no-drift ==="; rm "$MEGAPOWERS_MEMORY_DIR/other.md"; "$S/mem-index" >/dev/null; echo "index-lines=$(grep -c '^- \[' "$MEGAPOWERS_MEMORY_DIR/INDEX.md")"
  echo "=== bad-slug ==="; printf 'x\n' | "$S/mem-add" 'Bad Slug' --title t --hook h 2>&1; echo "rc=$?"
  echo "=== bad-kebab ==="; printf 'x\n' | "$S/mem-add" 'bad--slug' --title t --hook h >/dev/null 2>&1; echo "rc=$?"
  echo "=== yaml-injection ==="; printf 'b\n' | "$S/mem-add" yaml --title 'A: B' --hook 'has # hash' >/dev/null 2>&1; "$S/mem-recall" 'A: B'
  echo "=== malformed-skip ==="; printf -- '---\ntitle: "leak"\nno-close\n' > "$MEGAPOWERS_MEMORY_DIR/mal.md"; "$S/mem-index" 2>&1 | grep -i skip; echo "leak=$(grep -c leak "$MEGAPOWERS_MEMORY_DIR/INDEX.md")"
  # discipline present in the skill?
  sk="$ROOT/plugins/megapowers/skills/project-memory/SKILL.md"
  grep -qi "Don't save" "$sk" && echo "DISCIPLINE_OK" || echo "DISCIPLINE_MISSING"
  grep -qi "verify that still exists" "$sk" && echo "VERIFY_OK" || echo "VERIFY_MISSING"
} > out.txt 2>&1
cat out.txt
