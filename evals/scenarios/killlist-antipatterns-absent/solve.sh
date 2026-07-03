#!/usr/bin/env bash
# Assert (against the shipped skill files) that removed anti-patterns stay removed
# and the intended replacements are present. Writes a report of OK/BAD lines.
S="$ROOT/plugins/megapowers/skills"
report="lint.out"; : > "$report"

# absent() PATTERN FILE LABEL  -> BAD if the pattern is present
absent() { if grep -qE "$1" "$2" 2>/dev/null; then echo "BAD $3"; else echo "OK  $3"; fi; }
# present() PATTERN FILE LABEL -> BAD if the pattern is missing
present() { if grep -qE "$1" "$2" 2>/dev/null; then echo "OK  $3"; else echo "BAD $3"; fi; }

{
  # worktree: no auto-commit of the .gitignore edit (neither prose nor quick-ref table)
  absent 'add it to \.gitignore, commit the change' "$S/using-git-worktrees/SKILL.md" "worktree: no auto-commit prose"
  absent 'Add to \.gitignore \+ commit'             "$S/using-git-worktrees/SKILL.md" "worktree: no auto-commit table row"
  # writing-skills: no baked-in commit-and-push deployment step
  absent 'Commit skill to git and push'             "$S/writing-skills/SKILL.md"      "writing-skills: no commit-and-push step"
  # brainstorming: the unconditional per-section ask is gone
  absent '^- Ask after each section whether it looks right so far$' "$S/brainstorming/SKILL.md" "brainstorming: no unconditional per-section ask"
  # replacements present
  present 'opts into per-task commits'              "$S/writing-plans/SKILL.md"       "writing-plans: discloses commit cadence"
  present 'Confirm sections proportionally'         "$S/brainstorming/SKILL.md"       "brainstorming: proportional section confirm"
} >> "$report"
cat "$report"
