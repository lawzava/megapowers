#!/usr/bin/env bash
set -u
o="$WORKDIR/out.txt"; [ -f "$o" ] || { echo "no output"; exit 1; }
awk '/=== add ===/{f=1} f&&/wrote/{w=1} f&&/rc=0/{r=1} END{exit !(w&&r)}' "$o" || { echo "mem-add failed"; exit 1; }
awk '/=== index-has-both ===/{getline; if($0=="2")ok=1} END{exit !ok}' "$o" || { echo "index missing an entry"; exit 1; }
awk '/=== dup ===/{f=1} f&&/already exists/{m=1} f&&/rc=3/{r=1} END{exit !(m&&r)}' "$o" || { echo "duplicate slug not refused (exit 3)"; exit 1; }
grep -q "DB choice (db.md) — why sqlite" "$o" || { echo "recall did not find the memory"; exit 1; }
grep -q "index-lines=1" "$o" || { echo "index drifted after a file was deleted"; exit 1; }
awk '/=== bad-slug ===/{f=1} f&&/kebab/{m=1} f&&/rc=2/{r=1} /=== bad-kebab/{f=0} END{exit !(m&&r)}' "$o" || { echo "bad slug not rejected"; exit 1; }
awk '/=== bad-kebab ===/{f=1} f&&/rc=2/{r=1} END{exit !r}' "$o" || { echo "malformed kebab (bad--slug) not rejected"; exit 1; }
grep -q "A: B (yaml.md)" "$o" || { echo "YAML injection: title with ':'/'#' not preserved/recalled"; exit 1; }
awk '/=== malformed-skip ===/{f=1} f&&/skipping/{s=1} f&&/leak=0/{l=1} END{exit !(s&&l)}' "$o" || { echo "malformed frontmatter not skipped (leaked into index)"; exit 1; }
grep -q DISCIPLINE_OK "$o" || { echo "skill missing the save/don't-save discipline"; exit 1; }
grep -q VERIFY_OK "$o" || { echo "skill missing verify-recall-before-acting rule"; exit 1; }
echo "ok: project-memory contract + discipline hold"
