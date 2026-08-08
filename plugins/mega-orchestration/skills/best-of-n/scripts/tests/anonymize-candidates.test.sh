#!/usr/bin/env bash
# Characterization tests for an all-or-nothing blind candidate set.
set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ANON="$HERE/../anonymize-candidates"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
pass=0; fail=0

ok() { printf 'ok   %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf 'FAIL %s\n' "$1"; fail=$((fail + 1)); }
expect_refusal_without_output() {
  local label="$1" out="$2" rc="$3" err="$4"
  if [ "$rc" -eq 3 ] && grep -q 'anonymize-candidates:' "$err" && [ ! -e "$out" ] && [ -z "$(stage_residue)" ]; then
    ok "$label"
  else
    bad "$label (rc=$rc, out exists=$([ -e "$out" ] && printf yes || printf no), residue=$(stage_residue))"
  fi
}
stage_residue() {
  find "$TMP" -maxdepth 1 -name '.anonymize-candidates.*' -print -quit
}
expect_refusal() {
  local label="$1" rc="$2" err="$3"
  if [ "$rc" -eq 3 ] && grep -q 'anonymize-candidates:' "$err" && [ -z "$(stage_residue)" ]; then
    ok "$label"
  else
    bad "$label (rc=$rc, residue=$(stage_residue))"
  fi
}

mkdir -p "$TMP/src"
printf 'authored by Alice\n' > "$TMP/src/first.txt"

# A copy failure must not publish an empty or partial candidate directory.
mkdir -p "$TMP/no-copy"
printf '#!/usr/bin/env bash\nexit 1\n' > "$TMP/no-copy/cp"
chmod +x "$TMP/no-copy/cp"
PATH="$TMP/no-copy:$PATH" "$ANON" --src "$TMP/src" --out "$TMP/copy-failure" --marker Alice --seed 1 > /dev/null 2> "$TMP/copy.err"
rc=$?
expect_refusal_without_output "copy error aborts without publishing candidates" "$TMP/copy-failure" "$rc" "$TMP/copy.err"

# If marker detection cannot run, publishing would let author markers reach a judge.
mkdir -p "$TMP/no-grep"
printf '#!/usr/bin/env bash\nexit 2\n' > "$TMP/no-grep/grep"
chmod +x "$TMP/no-grep/grep"
PATH="$TMP/no-grep:$PATH" "$ANON" --src "$TMP/src" --out "$TMP/scan-failure" --marker Alice --seed 1 > /dev/null 2> "$TMP/scan.err"
rc=$?
expect_refusal_without_output "marker scan error aborts without publishing candidates" "$TMP/scan-failure" "$rc" "$TMP/scan.err"

# A destination created after staging must not turn publication into a nested, successful
# publication. The externally created directory remains untouched and staging is gone.
mkdir -p "$TMP/racing-perl"
cat > "$TMP/racing-perl/perl" <<EOF
#!/usr/bin/env bash
mkdir "$TMP/late-output"
printf external > "$TMP/late-output/sentinel"
exec /usr/bin/perl "\$@"
EOF
chmod +x "$TMP/racing-perl/perl"
ANONYMIZE_PERL="$TMP/racing-perl/perl" "$ANON" --src "$TMP/src" --out "$TMP/late-output" --marker Alice --seed 1 > /dev/null 2> "$TMP/late.err"
rc=$?
expect_refusal "late destination collision refuses atomically" "$rc" "$TMP/late.err"
if [ "$(cat "$TMP/late-output/sentinel")" = external ] && [ ! -e "$TMP/late-output/candidate-A" ]; then
  ok "late destination content is not mutated"
else
  bad "late destination content is not mutated"
fi

# A symlink to a directory can make ln create a nested link in an external target.
mkdir "$TMP/external-target-a" "$TMP/external-target-b"
mkdir -p "$TMP/racing-symlink-perl"
cat > "$TMP/racing-symlink-perl/perl" <<EOF
#!/usr/bin/env bash
/bin/ln -s "$TMP/external-target-a" "$TMP/late-symlink"
rm "$TMP/late-symlink"
/bin/ln -s "$TMP/external-target-b" "$TMP/late-symlink"
exec /usr/bin/perl "\$@"
EOF
chmod +x "$TMP/racing-symlink-perl/perl"
ANONYMIZE_PERL="$TMP/racing-symlink-perl/perl" "$ANON" --src "$TMP/src" --out "$TMP/late-symlink" --marker Alice --seed 1 > /dev/null 2> "$TMP/late-symlink.err"
rc=$?
expect_refusal "late symlink collision refuses atomically" "$rc" "$TMP/late-symlink.err"
if [ -z "$(find "$TMP/external-target-a" "$TMP/external-target-b" -mindepth 1 -print -quit)" ]; then
  ok "late symlink target is not mutated"
else
  bad "late symlink target is not mutated"
fi

# Symlinks, including dangling ones, are destination aliases and must be refused.
ln -s "$TMP/missing-target" "$TMP/symlink-output"
"$ANON" --src "$TMP/src" --out "$TMP/symlink-output" --marker Alice --seed 1 > /dev/null 2> "$TMP/symlink.err"
rc=$?
expect_refusal "symlink destination refuses without staging" "$rc" "$TMP/symlink.err"

# The normal publication path must not depend on GNU `mv -T`.
mkdir -p "$TMP/no-gnu-mv"
cat > "$TMP/no-gnu-mv/mv" <<'EOF'
#!/usr/bin/env bash
for arg in "$@"; do [ "$arg" != "-T" ] || exit 99; done
exec /bin/mv "$@"
EOF
chmod +x "$TMP/no-gnu-mv/mv"
(umask 022; PATH="$TMP/no-gnu-mv:$PATH" "$ANON" --src "$TMP/src" --out "$TMP/success" --marker Alice --seed 1) > "$TMP/manifest" 2> "$TMP/success.err"
rc=$?
if [ "$rc" -eq 0 ] && [ -d "$TMP/success" ] && [ ! -L "$TMP/success" ] &&
   [ -f "$TMP/success/candidate-A" ] &&
   [ "$(find "$TMP/success" -type f | wc -l | tr -d '[:space:]')" -eq 1 ] &&
   ! grep -qiF Alice "$TMP/success/candidate-A"; then
  ok "successful publication is complete and anonymous"
else
  bad "successful publication is complete and anonymous"
fi
mode="$(perl -e 'printf "%03o\n", (stat($ARGV[0]))[2] & 0777' "$TMP/success")"
if [ "$mode" = 755 ]; then
  ok "published directory honors the caller umask"
else
  bad "published directory honors the caller umask (mode=$mode)"
fi
rm -rf "$TMP/success"
if [ -z "$(stage_residue)" ]; then
  ok "removing the published directory leaves no hidden candidate set"
else
  bad "removing the published directory leaves no hidden candidate set"
fi

# A direct child signal after staging removes the private copy rather than leaving residue.
mkdir -p "$TMP/signalling-perl"
cat > "$TMP/signalling-perl/perl" <<'EOF'
#!/usr/bin/env bash
kill -TERM "$PPID"
exit 1
EOF
chmod +x "$TMP/signalling-perl/perl"
ANONYMIZE_PERL="$TMP/signalling-perl/perl" "$ANON" --src "$TMP/src" --out "$TMP/interrupted" --marker Alice --seed 1 > /dev/null 2> "$TMP/interrupted.err"
rc=$?
expect_refusal "signal cleanup removes staged candidates" "$rc" "$TMP/interrupted.err"
if [ ! -e "$TMP/interrupted" ]; then ok "interrupted run publishes nothing"; else bad "interrupted run publishes nothing"; fi

# TERM after a successful rename cannot roll publication back. The success finalizer
# completes private manifest delivery and reports success exactly once.
mkdir -p "$TMP/post-publish-perl"
cat > "$TMP/post-publish-perl/perl" <<'EOF'
#!/usr/bin/env bash
/usr/bin/perl "$@"
rc=$?
kill -TERM "$PPID"
exit "$rc"
EOF
chmod +x "$TMP/post-publish-perl/perl"
ANONYMIZE_PERL="$TMP/post-publish-perl/perl" "$ANON" --src "$TMP/src" --out "$TMP/post-publish" --marker Alice --seed 1 > "$TMP/post-publish.manifest" 2> "$TMP/post-publish.err"
rc=$?
if [ "$rc" -eq 0 ] && grep -q 'signal arrived after publication' "$TMP/post-publish.err" &&
   cmp -s "$TMP/post-publish.manifest" <(printf 'candidate-A\tfirst.txt\n') &&
   [ -d "$TMP/post-publish" ] && [ ! -L "$TMP/post-publish" ] &&
   [ -f "$TMP/post-publish/candidate-A" ] && [ -z "$(stage_residue)" ]; then
  ok "post-publication signal preserves the committed complete output"
else
  bad "post-publication signal preserves the committed complete output"
fi

if cmp -s "$TMP/src/first.txt" <(printf 'authored by Alice\n'); then
  ok "refusals never mutate source candidates"
else
  bad "refusals never mutate source candidates"
fi

printf '== %d passed, %d failed ==\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
