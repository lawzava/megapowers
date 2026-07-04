#!/usr/bin/env bash
# Payload-size regression for the megapowers session-start hook. The SessionStart
# injection must stay a lean nudge, not the whole skill re-pasted into every session
# (and again after every /clear and compaction). Assert the emitted additionalContext
# is under 300 words, that the dead-weight blocks are stripped (YAML frontmatter, whose
# name+description already appear in the skills listing, and the "## Platform
# Adaptation" section, which points other harnesses at their own reference files), and
# that the workflow-critical Core Rule survives. The communication-register rules must
# also survive (condensed in SKILL.md, not stripped by this hook): the maintainer
# requires them in every session. SKILL.md on disk stays complete and portable.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/../session-start"
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 2; }

pass=0; fail=0
out="$(bash "$HOOK" 2>/dev/null)"
ctx="$(printf '%s' "$out" | jq -r '.hookSpecificOutput.additionalContext' 2>/dev/null || true)"
words="$(printf '%s' "$ctx" | wc -w | tr -d ' ')"

if [ -n "$ctx" ] && [ "$ctx" != "null" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL hook emitted no additionalContext\n'; fi
if [ "${words:-0}" -le 300 ]; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL emitted payload %s words, want <=300\n' "$words"; fi
if printf '%s' "$ctx" | grep -q 'description:'; then fail=$((fail + 1)); printf '  FAIL YAML frontmatter leaked into payload\n'; else pass=$((pass + 1)); fi
if printf '%s' "$ctx" | grep -q 'Platform Adaptation'; then fail=$((fail + 1)); printf '  FAIL Platform Adaptation section not stripped\n'; else pass=$((pass + 1)); fi
if printf '%s' "$ctx" | grep -q 'Core Rule'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL workflow-critical Core Rule missing from payload\n'; fi
if printf '%s' "$ctx" | grep -qi 'no dash punctuation'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL communication-register no-dash-punctuation rule missing from payload\n'; fi
if printf '%s' "$ctx" | grep -qi 'for takeover'; then pass=$((pass + 1)); else fail=$((fail + 1)); printf '  FAIL communication-register takeover rule missing from payload\n'; fi

echo "== session-start: $pass passed, $fail failed (payload ${words} words) =="
[ "$fail" -eq 0 ]
