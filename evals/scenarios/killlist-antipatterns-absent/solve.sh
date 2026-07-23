#!/usr/bin/env bash
# Assert (against the shipped skill files) that removed anti-patterns stay removed
# and the intended replacements are present. Writes a report of OK/BAD lines.
S="$ROOT/plugins/megapowers/skills"
V="$ROOT/scripts/validate.sh"
CI="$ROOT/.github/workflows/ci.yml"
U="$S/using-megapowers/SKILL.md"
G="$ROOT/plugins/mega-go/skills/greenfield-go-stack/SKILL.md"
WS1="$S/writing-skills/testing-skills-with-subagents.md"
WS2="$S/writing-skills/de-prescription-rubric.md"
README="$ROOT/README.md"
CONTRIB="$ROOT/CONTRIBUTING.md"
HARNESS="$ROOT/docs/harness-support.md"
SECURITY="$ROOT/SECURITY.md"
ORCH_README="$ROOT/plugins/mega-orchestration/README.md"
CODEX_GUARD="$ROOT/plugins/mega-guardrails/hooks/codex-deny-destructive.sh"
EFFECT="$ROOT/plugins/mega-orchestration/skills/effect-broker/SKILL.md"
ORCH="$ROOT/plugins/mega-orchestration/skills/orchestrating/SKILL.md"
AUTO="$ROOT/plugins/mega-orchestration/skills/autonomous-run/SKILL.md"
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
  # validator: tracked files only, no undeclared rg, and one source of truth for
  # the SessionStart payload transformation
  absent "find plugins scripts evals -type f -name '\\*\\.sh'" "$V" "validate: no ignored-tree shell discovery"
  absent 'if rg -q'                                  "$V" "validate: no undeclared rg dependency"
  absent 'trimmed="\$\(awk'                         "$V" "validate: no duplicated SessionStart awk"
  present 'git ls-files .*plugins scripts evals'     "$V" "validate: tracked and new shell discovery"
  present 'hookSpecificOutput.additionalContext'     "$V" "validate: measures real SessionStart payload"
  # CI: a single eval run produces both the gate and scorecard input
  if [ "$(grep -c 'bash evals/run-all.sh' "$CI")" -eq 1 ]; then echo "OK  ci: eval suite runs once"; else echo "BAD ci: eval suite runs once"; fi
  absent 'Pi `references/pi-tools.md`'               "$U" "using-megapowers: no undeclared Pi harness"
  absent 'context7 before wiring'                    "$G" "greenfield-go: Context7 is not mandatory"
  present 'if (it is )?installed'                    "$G" "greenfield-go: optional docs tooling is explicit"
  absent 'check-description-freeze\.sh'             "$WS1" "writing-skills: no dead freeze enforcement claim"
  absent 'check-description-freeze\.sh'             "$WS2" "de-prescription: no dead freeze enforcement claim"
  absent 'Everything executable is plain bash|no runtime' "$README" "README: executable runtime claim is accurate"
  absent 'local Node server'                         "$README" "README: no removed Node companion claim"
  absent 'standalone entries|standalone marketplace' "$README" "README: no removed standalone distribution"
  absent 'manual Codex pilot'                        "$CONTRIB" "contributing: no stale manual Codex pilot"
  present 'mega-frontend'                            "$HARNESS" "harness support: frontend manifests disclosed"
  absent 'local Node server'                         "$HARNESS" "harness support: no removed Node companion claim"
  present 'mega-frontend'                            "$SECURITY" "security: frontend capability row disclosed"
  absent 'Stop hooks \(Claude Code only\)'          "$ORCH_README" "orchestration: Stop hook scope is current"
  present 'delegate-nudge.*Claude Code and Codex'    "$ORCH_README" "orchestration: Codex nudge disclosed"
  absent 'codex-hooks\.json'                        "$CODEX_GUARD" "guard adapter: no obsolete manual wiring"
  absent 'PreToolUse hook \(Claude Code only\)'     "$EFFECT" "effect broker: guard is not mislabeled Claude-only"
  present 'Claude Code and Codex'                    "$EFFECT" "effect broker: guard adapter scope is current"
  present 'outer workflow once'                     "$U" "workflow: one outer announcement"
  absent 'For every skill you invoke: announce'     "$U" "workflow: no per-skill announcement ritual"
  absent 'make a todo per checklist item'           "$U" "workflow: no checklist todo duplication"
  present 'skip brainstorming and planning'         "$S/brainstorming/SKILL.md" "workflow: scoped fast path"
  present 'smallest focused test'                   "$S/test-driven-development/SKILL.md" "tdd: focused red-green verification"
  present 'canonical suite.*(task|milestone|branch)' "$S/test-driven-development/SKILL.md" "tdd: canonical suite at boundaries"
  absent 'complete code'                            "$S/writing-plans/SKILL.md" "plans: no blanket full-code mandate"
  present 'exact line ranges only'                  "$S/writing-plans/SKILL.md" "plans: detail only when subtle"
  present 'Low-risk|Low risk'                       "$S/requesting-code-review/SKILL.md" "review: proportional low-risk path"
  present 'fix and re-review at three cycles'        "$S/subagent-driven-development/SKILL.md" "review: bounded cycle cap"
  present '8 to 10 completed tasks'                 "$S/subagent-driven-development/SKILL.md" "context: controller rollover"
  present 'final 20 percent'                        "$S/subagent-driven-development/SKILL.md" "context: synthesis reserve"
  present 'one report channel'                      "$S/subagent-driven-development/SKILL.md" "delegation: single report channel"
  present 'mechanical mode: one owner'              "$S/subagent-driven-development/SKILL.md" "delegation: bulk mechanical mode"
  present 'already stated the destination'          "$S/finishing-a-development-branch/SKILL.md" "finish: explicit intent bypasses menu"
  present 'Do not poll unchanged state'             "$ORCH" "monitoring: transition driven"
  present 'at most three turns'                     "$ORCH" "context: bounded fork inheritance"
  present 'Autonomous execution chooses inline work' "$AUTO" "autonomy: chooses executor proportionally"
  present 'never grants permission to commit'       "$AUTO" "git: workflow is not authorization"
} >> "$report"
cat "$report"
