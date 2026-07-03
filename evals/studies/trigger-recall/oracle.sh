#!/usr/bin/env bash
# oracle.sh <results-dir> — score trigger-recall runs into a markdown scorecard.
#
# Per on-topic task: recall = runs whose Skill invocations include the expected
# skill / non-error runs. Per negative task: domain false-fire = runs invoking
# any megapowers DOMAIN skill (dispatcher invocations — using-megapowers — are
# counted separately: firing the dispatcher on any task is designed behavior;
# what must stay quiet on off-topic work is the domain skills).
set -uo pipefail
DIR="${1:?usage: oracle.sh <results-dir>}"
rows="$(mktemp)"; trap 'rm -f "$rows"' EXIT

for meta in "$DIR"/*/run-*/meta.json; do
  [ -f "$meta" ] || continue
  rundir="$(dirname "$meta")"
  task="$(jq -r .task "$meta")"; expected="$(jq -r .expected "$meta")"
  rc="$(jq -r .rc "$meta")"
  inv="$rundir/skills-invoked.txt"
  # triggering happens early in a run, so a turn-capped/nonzero-exit run still
  # carries valid trigger evidence — indeterminate only when nothing ran at all
  if [ ! -s "$rundir/transcript.jsonl" ]; then verdict=INDETERMINATE; detail="no-transcript(rc=$rc)"
  elif [ "$expected" != "-" ]; then
    if grep -q "$expected" "$inv" 2>/dev/null; then verdict=HIT; detail="$expected"
    else
      other="$(grep -v 'using-megapowers' "$inv" 2>/dev/null | paste -sd, -)"
      verdict=MISS; detail="${other:-none}"
    fi
  else
    domain="$(grep -v 'using-megapowers' "$inv" 2>/dev/null | paste -sd, -)"
    if [ -n "$domain" ]; then verdict=FALSE_FIRE; detail="$domain"
    else
      dc="$(grep -c 'using-megapowers' "$inv" 2>/dev/null | head -1)"
      verdict=QUIET; detail="${dc:-0}-dispatcher"
    fi
  fi
  printf '%s\t%s\t%s\t%s\n' "$task" "$expected" "$verdict" "$detail" >> "$rows"
done
[ -s "$rows" ] || { echo "no runs found under $DIR" >&2; exit 1; }

echo "# megapowers trigger-recall study (installed plugin, organic tasks)"
echo
echo "Tasks never name a skill. HIT = the expected skill was invoked via the Skill"
echo "tool; FALSE_FIRE = a domain skill fired on an off-topic task. Dispatcher"
echo "invocations (using-megapowers) are excluded from false-fire by design."
echo
echo "| task | expected skill | recall / quiet-rate (n) |"
echo "|---|---|---|"
for task in $(cut -f1 "$rows" | sort -u); do
  exp="$(awk -F'\t' -v t="$task" '$1==t{print $2; exit}' "$rows")"
  n=$(awk -F'\t' -v t="$task" '$1==t && $3!="INDETERMINATE"{c++} END{print c+0}' "$rows")
  if [ "$exp" != "-" ]; then
    hits=$(awk -F'\t' -v t="$task" '$1==t && $3=="HIT"{c++} END{print c+0}' "$rows")
    awk -v t="$task" -v e="$exp" -v h="$hits" -v n="$n" \
      'BEGIN{printf "| %s | %s | %.0f%% recall (%d/%d) |\n", t, e, (n?h/n*100:0), h, n}'
  else
    quiet=$(awk -F'\t' -v t="$task" '$1==t && $3=="QUIET"{c++} END{print c+0}' "$rows")
    awk -v t="$task" -v q="$quiet" -v n="$n" \
      'BEGIN{printf "| %s | (should stay quiet) | %.0f%% quiet (%d/%d) |\n", t, (n?q/n*100:0), q, n}'
  fi
done
echo
echo "## Per-run detail (verdict: skills seen)"
echo '```'
awk -F'\t' '{k=$1" "$3" ["$4"]"} {c[k]++} END{for(x in c) print c[x]"\t"x}' "$rows" | sort -k2
echo '```'
ind=$(awk -F'\t' '$3=="INDETERMINATE"{c++} END{print c+0}' "$rows")
echo
echo "_indeterminate runs excluded: ${ind}_"
