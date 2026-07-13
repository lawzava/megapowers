#!/usr/bin/env bash
set -u
w="$WORKDIR"
res="$w/res.out"; [ -f "$res" ] || { echo "no res.out"; exit 1; }
get() { sed -n "s/^$1=//p" "$res" | head -1; }

[ "$(get rc1)" = "0" ] || { echo "happy-path run failed (rc1=$(get rc1))"; exit 1; }
[ "$(get rc3)" = "0" ] || { echo "non-seeded run failed (rc3=$(get rc3))"; exit 1; }

# manifest: exactly 6 candidates, labels candidate-A..F, originals cover every input
m="$w/man1.out"
[ "$(wc -l < "$m")" -eq 6 ] || { echo "manifest is not 6 lines"; cat "$m"; exit 1; }
cut -f1 "$m" | sort > "$w/labels"
printf 'candidate-A\ncandidate-B\ncandidate-C\ncandidate-D\ncandidate-E\ncandidate-F\n' > "$w/labels.want"
diff -q "$w/labels" "$w/labels.want" >/dev/null || { echo "labels are not candidate-A..F"; cat "$m"; exit 1; }
cut -f2 "$m" | sort > "$w/origs"
printf 'alpha.txt\nbeta.txt\nbinary.bin\ncrlf.txt\ngamma\nnoeol.txt\n' > "$w/origs.want"
diff -q "$w/origs" "$w/origs.want" >/dev/null || { echo "manifest originals mismatch"; cat "$m"; exit 1; }

# determinism under --seed
diff -q "$w/man1.out" "$w/man2.out" >/dev/null || { echo "same seed produced a different manifest (not deterministic)"; exit 1; }

# markers were actually stripped from the produced candidates
[ -s "$w/residual.out" ] && { echo "an authorship marker survived in output"; cat "$w/residual.out"; exit 1; }

# refusal on a filename-marker leak: nonzero exit, no manifest, out dir removed
[ "$(get refuse_rc)" != "0" ] || { echo "filename marker leak was not refused"; exit 1; }
[ -s "$w/refuse.out" ] && { echo "refusal still emitted a manifest"; exit 1; }
[ "$(get rout)" = "ROUT_ABSENT" ] || { echo "a refused run left its out dir behind"; exit 1; }

# refusal on a content seam (deletion re-formed a marker)
[ "$(get seam_rc)" != "0" ] || { echo "content-seam marker survival was not refused"; exit 1; }
[ -s "$w/seam.out" ] && { echo "seam refusal still emitted a manifest"; exit 1; }

# binary candidates remain byte-identical and symlink candidates are refused
[ "$(get binary_preserved)" = "YES" ] || { echo "binary candidate was rewritten"; exit 1; }
[ "$(get noeol_preserved)" = "YES" ] || { echo "marker stripping changed the final-newline state"; exit 1; }
[ "$(get crlf_preserved)" = "YES" ] || { echo "marker stripping changed CRLF line endings"; exit 1; }
[ "$(get link_rc)" != "0" ] || { echo "symlink candidate was not refused"; exit 1; }
[ "$(get lout)" = "LOUT_ABSENT" ] || { echo "symlink refusal left its out dir behind"; exit 1; }

# no time-based seeding; entropy comes from /dev/urandom
[ "$(get date_hits)" = "0" ] || { echo "script references date (time-based seeding is forbidden)"; exit 1; }
[ "$(get urandom)" = "OK" ] || { echo "script does not seed from /dev/urandom"; exit 1; }

echo "ok: anonymize-candidates blinds, refuses survivors, and shuffles deterministically only under --seed"
