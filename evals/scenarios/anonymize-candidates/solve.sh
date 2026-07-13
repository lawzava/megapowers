#!/usr/bin/env bash
set -u
A="$ROOT/plugins/mega-orchestration/skills/best-of-n/scripts/anonymize-candidates"

# --- happy path: text, directory, and binary candidates; markers must be stripped ---
src="$PWD/src"; mkdir -p "$src/gamma"
printf 'solution one, authored by ModelX using VendorY tooling.\n' > "$src/alpha.txt"
printf 'a second take.\nsigned CodeName.\n'                        > "$src/beta.txt"
printf 'nested work by ModelX.\n'                                  > "$src/gamma/notes.md"
cp /bin/true "$src/binary.bin"
printf 'ModelX without final newline' > "$src/noeol.txt"
printf 'ModelX with CRLF\r\n' > "$src/crlf.txt"

out="$PWD/out"
"$A" --src "$src" --out "$out" --marker ModelX --marker VendorY --marker CodeName --seed 42 > man1.out 2> man1.err
rc1=$?

# determinism: same --seed + same inputs => byte-identical manifest
out2="$PWD/out2"
"$A" --src "$src" --out "$out2" --marker ModelX --marker VendorY --marker CodeName --seed 42 > man2.out 2> man2.err
rc2=$?

# residual scan across the produced candidates: no marker may survive
grep -rIiF -e ModelX -e VendorY -e CodeName "$out" > residual.out 2>/dev/null
binary_label="$(awk -F '\t' '$2 == "binary.bin" { print $1 }' man1.out)"
if [ -n "$binary_label" ] && cmp -s "$src/binary.bin" "$out/$binary_label"; then
  binary_preserved=YES
else
  binary_preserved=NO
fi
noeol_label="$(awk -F '\t' '$2 == "noeol.txt" { print $1 }' man1.out)"
printf ' without final newline' > noeol.want
if [ -n "$noeol_label" ] && cmp -s noeol.want "$out/$noeol_label"; then noeol_preserved=YES; else noeol_preserved=NO; fi
crlf_label="$(awk -F '\t' '$2 == "crlf.txt" { print $1 }' man1.out)"
printf ' with CRLF\r\n' > crlf.want
if [ -n "$crlf_label" ] && cmp -s crlf.want "$out/$crlf_label"; then crlf_preserved=YES; else crlf_preserved=NO; fi

# non-seeded run still succeeds (and must not seed from time)
out3="$PWD/out3"
"$A" --src "$src" --out "$out3" --marker ModelX --marker VendorY --marker CodeName > man3.out 2> man3.err
rc3=$?

# refusal: a NESTED file name carries a marker (top-level names are renamed away by the
# copy, so a leak survives only inside a candidate dir) -> abort, no manifest, out removed
rsrc="$PWD/rsrc"; mkdir -p "$rsrc/cand"
printf 'clean body\n' > "$rsrc/cand/by-ModelX-note.txt"
printf 'other body\n' > "$rsrc/plain.txt"
rout="$PWD/rout"
"$A" --src "$rsrc" --out "$rout" --marker ModelX --seed 1 > refuse.out 2> refuse.err
refuse_rc=$?

# refusal: deletion re-forms a marker at the seam -> content survival is caught
ssrc="$PWD/ssrc"; mkdir -p "$ssrc"
printf 'aabb\n' > "$ssrc/one.txt"
sout="$PWD/sout"
"$A" --src "$ssrc" --out "$sout" --marker ab --seed 1 > seam.out 2> seam.err
seam_rc=$?

# refusal: symlinks can disclose the original path and escape the copied candidate set
lsrc="$PWD/lsrc"; mkdir -p "$lsrc"
ln -s "$ROOT/README.md" "$lsrc/submission"
lout="$PWD/lout"
"$A" --src "$lsrc" --out "$lout" --marker ModelX --seed 1 > link.out 2> link.err
link_rc=$?

# static guard: no time-based seeding anywhere; entropy from /dev/urandom
dh=$(grep -cE '\bdate\b' "$A" 2>/dev/null); [ -n "$dh" ] || dh=0

{
  echo "rc1=$rc1"
  echo "rc2=$rc2"
  echo "rc3=$rc3"
  echo "refuse_rc=$refuse_rc"
  echo "seam_rc=$seam_rc"
  echo "link_rc=$link_rc"
  echo "binary_preserved=$binary_preserved"
  echo "noeol_preserved=$noeol_preserved"
  echo "crlf_preserved=$crlf_preserved"
  if [ -e "$rout" ]; then echo "rout=ROUT_EXISTS"; else echo "rout=ROUT_ABSENT"; fi
  if [ -e "$lout" ]; then echo "lout=LOUT_EXISTS"; else echo "lout=LOUT_ABSENT"; fi
  echo "date_hits=$dh"
  if grep -q '/dev/urandom' "$A" 2>/dev/null; then echo "urandom=OK"; else echo "urandom=MISSING"; fi
} > res.out
cat res.out
