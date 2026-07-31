#!/usr/bin/env bash
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN="$HERE/../delegate-run"
DIFF_ID="$HERE/../review-diff-id"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }
want_rc() {
  local want="$1" got="$2" desc="$3"
  if [ "$want" = "$got" ]; then ok; else bad "$desc: want rc=$want got=$got"; fi
}
want_jq() {
  local file="$1" query="$2" desc="$3"
  if jq -e "$query" "$file" >/dev/null 2>&1; then ok; else bad "$desc"; fi
}

repo="$TMP/repo"
mkdir -p "$repo"
cd "$repo" || exit 1
git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
printf 'base\n' > service.txt
git add service.txt
git commit -qm init

echo "== review-diff-id tests =="
id0="$("$DIFF_ID")"
printf 'changed\n' > service.txt
id1="$("$DIFF_ID")"
[ "$id0" != "$id1" ] && ok || bad "tracked change must change diff id"
printf 'untracked\n' > extra.txt
id2="$("$DIFF_ID")"
[ "$id1" != "$id2" ] && ok || bad "untracked file must change diff id"
mkdir -p nested
id2_sub="$(cd nested && "$DIFF_ID")"
[ "$id2" = "$id2_sub" ] && ok || bad "diff id must be independent of current subdirectory"
printf 'changed again\n' > service.txt
id3="$("$DIFF_ID")"
[ "$id2" != "$id3" ] && ok || bad "non-risky tracked change must stale receipt id"
printf 'staged one\n' > service.txt
git add service.txt
printf 'same worktree\n' > service.txt
id_staged_one="$("$DIFF_ID")"
printf 'staged two\n' > service.txt
git add service.txt
printf 'same worktree\n' > service.txt
id_staged_two="$("$DIFF_ID")"
[ "$id_staged_one" != "$id_staged_two" ] && ok || bad "index-only change must change diff id"

fake="$TMP/fake-claude"
cat > "$fake" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "--help" ]; then
  echo "--bare --json-schema --effort"
  head -c 131072 /dev/zero | tr '\0' x
  exit 0
fi
# Holds this dispatch inside the provider call until the test releases it, so a
# race between two dispatches can be built by hand instead of hoped for.
if [ -n "${FAKE_WAIT_FOR:-}" ]; then
  while [ ! -e "$FAKE_WAIT_FOR" ]; do sleep 0.05; done
fi
printf '%s\n' "$*" > "${FAKE_ARGS_LOG:?}"
printf '%s\n' "${CLAUDE_CONFIG_DIR:-}" > "${FAKE_CONFIG_LOG:?}"
# Records WHICH credential the launcher copied in, so an OAuth test can prove it
# handled the credential it planted rather than the operator's real one.
if [ -n "${FAKE_CRED_LOG:-}" ]; then
  cat "${CLAUDE_CONFIG_DIR:-}/.credentials.json" > "$FAKE_CRED_LOG" 2>/dev/null || : > "$FAKE_CRED_LOG"
fi
while [ $# -gt 0 ]; do
  if [ "$1" = "--json-schema" ]; then
    printf '%s\n' "$2" > "${FAKE_SCHEMA_LOG:?}"
    shift 2
  else
    shift
  fi
done
if [ "${FAKE_VERDICT:-approve}" = "invalid" ]; then
  printf 'not-json\n'
  exit 0
fi
if [ "${FAKE_VERDICT:-approve}" = "invalid-finding" ]; then
  jq -cn '{structured_output:{verdict:"needs_attention",findings:[{severity:"urgent"}],next_steps:[],evidence:{commands:[],screenshots:[]}}}'
  exit 0
fi
case "${FAKE_VERDICT:-approve}" in
  approve-with-*)
    jq -cn --arg severity "${FAKE_VERDICT#approve-with-}" '{
      structured_output:{
        verdict:"approve",
        findings:[{
          severity:$severity,
          file:"billing.go",
          lines:"10-14",
          confidence:0.9,
          finding:"the retry path double charges",
          recommendation:"key the retry on the idempotency token"
        }],
        next_steps:[],
        evidence:{commands:["git diff HEAD"],screenshots:[]}
      }
    }'
    exit 0
    ;;
esac
jq -cn --arg verdict "${FAKE_VERDICT:-approve}" '{
  structured_output:{
    verdict:$verdict,
    findings:[],
    next_steps:[],
    evidence:{commands:["git diff HEAD"],screenshots:[]}
  }
}'
EOF
chmod +x "$fake"

cfg="$TMP/routes.toml"
cat > "$cfg" <<EOF
[tiers]
scale = ["fast", "strong", "frontier"]
[efforts]
scale = ["low", "medium", "high"]
[providers.reviewer]
vendor = "anthropic"
binary = "$fake"
channel = "test"
default_tier = "frontier"
effort = "high"
efforts = ["high"]
capabilities = ["code", "vision"]
[providers.reviewer.tiers]
frontier = "fake-frontier"
# The author's own provider. Never a candidate here (verify is single-route to
# reviewer); it exists so --author-model has a model id to map back to a vendor.
[providers.authorside]
vendor = "openai"
binary = "sh"
channel = "test"
default_tier = "frontier"
effort = "high"
efforts = ["high"]
capabilities = ["code"]
[providers.authorside.tiers]
frontier = "fake-author-model"
[roles]
verify = "reviewer"
visual_verify = "reviewer"
[role_tiers]
verify = "frontier"
visual_verify = "frontier"
[role_efforts]
verify = "high"
visual_verify = "high"
[independence]
verify = "author_vendor"
visual_verify = "author_vendor"
[defaults]
floor = "strong:low"
EOF

echo "== delegate-run tests =="
receipt="$TMP/receipt.json"
export FAKE_SCHEMA_LOG="$TMP/claude-schema.json"
export FAKE_ARGS_LOG="$TMP/claude-args.txt"
export FAKE_CONFIG_LOG="$TMP/claude-config.txt"
export ANTHROPIC_API_KEY=test-key
set +e
"$RUN" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent; $(touch should-not-run)' \
  --receipt "$receipt" --config "$cfg" >/dev/null 2>"$TMP/run.err"
rc=$?
set -e
want_rc 0 "$rc" "approve receipt"
want_jq "$receipt" '.schema == "megapowers.review-receipt.v2"' "receipt schema"
want_jq "$receipt" '.subject.kind == "worktree-diff"' "worktree subject"
if jq -e --arg id "$("$DIFF_ID")" '.subject.id == $id' "$receipt" >/dev/null 2>&1; then ok; else bad "receipt binds current diff"; fi
# The id fingerprints the pending delta only, so without the base the receipt
# names a diff rather than a tree.
if jq -e --arg base "$(git -C "$repo" rev-parse HEAD)" '.subject.base == $base' "$receipt" >/dev/null 2>&1; then ok; else bad "receipt binds the base commit"; fi
want_jq "$receipt" '.author_vendors == ["openai"]' "receipt binds author vendor"
want_jq "$receipt" '.reviewer.vendor == "anthropic" and .reviewer.model == "fake-frontier" and .reviewer.tier == "frontier" and .reviewer.effort == "high"' "launcher provenance"
want_jq "$receipt" '.independent == true and .result.verdict == "approve"' "independent approval"
want_jq "$FAKE_SCHEMA_LOG" 'has("$schema") | not' "Claude schema omits unsupported draft metadata"
[ ! -e "$repo/should-not-run" ] && ok || bad "claim shell metacharacters executed"

# A harness that knows only its model id must still be able to run an independent
# review, and the receipt must record the vendor that was derived from it.
receipt_am="$TMP/receipt-author-model.json"
set +e
"$RUN" --role verify --author-model fake-author-model --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$receipt_am" --config "$cfg" >/dev/null 2>"$TMP/run-am.err"
rc=$?
set -e
want_rc 0 "$rc" "--author-model runs an independent review"
want_jq "$receipt_am" '.author_vendors == ["openai"]' "receipt binds the vendor derived from --author-model"
want_jq "$receipt_am" '.independent == true' "derived authorship still proves independence"

# An id no provider declares is a usage error, not a silently unbound review.
set +e
"$RUN" --role verify --author-model no-such-model --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-unknown.json" --config "$cfg" >/dev/null 2>"$TMP/run-unknown.err"
rc=$?
set -e
want_rc 2 "$rc" "unknown --author-model is refused"
[ ! -e "$TMP/receipt-unknown.json" ] && ok || bad "a refused dispatch must not write a receipt"

# Author provenance comes from the resolver, not from what the caller typed. A route
# that omits AUTHOR_VENDORS must stop the dispatch rather than let caller input stand
# in for it. Stub the resolver next to a copy of the launcher: no production seam.
stub_dir="$TMP/stub-scripts"
cp -r "$(dirname "$RUN")" "$stub_dir"
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
# A route that resolves but never says which authors it excluded.
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-noauthors.json" --config "$cfg" >/dev/null 2>"$TMP/run-noauthors.err"
rc=$?
set -e
want_rc 2 "$rc" "a route omitting AUTHOR_VENDORS is refused"

# Repeated fields are authoritative. Make the two encodings disagree: the repeated one
# names the reviewer's own vendor, which must trip the independence self-check, while
# the joined one names a different vendor that would sail past it.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
echo "AUTHOR_VENDOR=anthropic"
echo "AUTHOR_VENDORS=openai"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-repeated.json" --config "$cfg" >/dev/null 2>"$TMP/run-repeated.err"
rc=$?
set -e
want_rc 2 "$rc" "the repeated author field is preferred over the joined one"
grep -q 'author vendor' "$TMP/run-repeated.err" && ok || bad "preferring the repeated field must trip the independence self-check"
[ ! -e "$TMP/receipt-repeated.json" ] && ok || bad "a self-review must not write a receipt"

# A resolver that predates the repeated form still works through the joined fallback.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
echo "AUTHOR_VENDORS=anthropic"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-legacy.json" --config "$cfg" >/dev/null 2>"$TMP/run-legacy.err"
rc=$?
set -e
want_rc 2 "$rc" "the joined field still governs when no repeated field is present"
grep -q 'author vendor' "$TMP/run-legacy.err" && ok || bad "the fallback must fail on the self-review, not on a parse"
grep -q 'empty AUTHOR_VENDOR' "$TMP/run-legacy.err" && bad "a route with no repeated field must not report an empty record" || ok

# A declared repeated form that yields nothing is a broken route, not a reason to fall
# back to the joined one: presence decides, not count.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
echo "AUTHOR_VENDOR="
echo "AUTHOR_VENDORS=openai"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-emptyrec.json" --config "$cfg" >/dev/null 2>"$TMP/run-emptyrec.err"
rc=$?
set -e
want_rc 2 "$rc" "an empty repeated record is refused"
grep -q 'empty AUTHOR_VENDOR' "$TMP/run-emptyrec.err" && ok || bad "an empty repeated record must say so"
[ ! -e "$TMP/receipt-emptyrec.json" ] && ok || bad "an empty repeated record must not write a receipt"

# One valid plus one empty is still incomplete provenance.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
echo "AUTHOR_VENDOR=openai"
echo "AUTHOR_VENDOR="
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-partial.json" --config "$cfg" >/dev/null 2>"$TMP/run-partial.err"
rc=$?
set -e
want_rc 2 "$rc" "a valid record beside an empty one is refused"

# An independent review runs as an isolated one-shot session ON PURPOSE, so this
# launcher always needs a reachable CLI. A resolver that skipped its own reachability
# check (which it does for a native route) must not leave that unverified here: the
# launcher would otherwise try to exec a binary that is not installed.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "DISPATCH=native"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=definitely-not-an-installed-binary-xyz"
echo "AUTHOR_VENDORS=openai"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
"$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-nocli.json" --config "$cfg" >/dev/null 2>"$TMP/run-nocli.err"
rc=$?
set -e
want_rc 2 "$rc" "a route whose CLI is absent is refused before dispatch"
grep -q 'not installed' "$TMP/run-nocli.err" && ok || bad "an absent reviewer CLI must say so"
[ ! -e "$TMP/receipt-nocli.json" ] && ok || bad "an absent reviewer CLI must not write a receipt"
# A field of separators alone decodes to empty identities. Length is not emptiness:
# the check has to look at what it actually recovered.
cat > "$stub_dir/delegate-resolve" <<'EOF'
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=$FAKE_BINARY"
echo "AUTHOR_VENDORS=,"
exit 0
EOF
chmod +x "$stub_dir/delegate-resolve"
set +e
FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
  --claim 'billing remains idempotent' \
  --receipt "$TMP/receipt-emptyauthors.json" --config "$cfg" >/dev/null 2>"$TMP/run-emptyauthors.err"
rc=$?
set -e
want_rc 2 "$rc" "AUTHOR_VENDORS holding only separators is refused"
[ ! -e "$TMP/receipt-emptyauthors.json" ] && ok || bad "an empty decoded identity must not write a receipt"

# `read -a` drops trailing empty fields, so a blank component can survive decoding.
# The raw field has to be rejected before it is split.
for malformed in 'openai,' ',openai' 'openai,,anthropic' ' ' 'openai, ,anthropic'; do
  cat > "$stub_dir/delegate-resolve" <<EOF
#!/usr/bin/env bash
echo "ROLE=verify"
echo "PROVIDER=reviewer"
echo "MODEL=fake-frontier"
echo "TIER=frontier"
echo "EFFORT=high"
echo "CHANNEL=test"
echo "ENABLED=true"
echo "VENDOR=anthropic"
echo "BINARY=\$FAKE_BINARY"
echo "AUTHOR_VENDORS=$malformed"
exit 0
EOF
  chmod +x "$stub_dir/delegate-resolve"
  set +e
  FAKE_BINARY="$fake" "$stub_dir/delegate-run" --role verify --author-vendor openai --artifact worktree \
    --claim 'billing remains idempotent' \
    --receipt "$TMP/receipt-malformed.json" --config "$cfg" >/dev/null 2>"$TMP/run-malformed.err"
  rc=$?
  set -e
  want_rc 2 "$rc" "malformed AUTHOR_VENDORS '$malformed' is refused"
  [ ! -e "$TMP/receipt-malformed.json" ] && ok || bad "malformed AUTHOR_VENDORS '$malformed' must not write a receipt"
  rm -f "$TMP/receipt-malformed.json"
done
grep -q 'omitted the author vendors' "$TMP/run-noauthors.err" && ok || bad "omitted author provenance must say so"
[ ! -e "$TMP/receipt-noauthors.json" ] && ok || bad "a route omitting AUTHOR_VENDORS must not write a receipt"

unset ANTHROPIC_API_KEY
mkdir -p "$TMP/oauth-home/.claude"
printf '{}\n' > "$TMP/oauth-home/.claude/.credentials.json"
set +e
# `env -u CLAUDE_CONFIG_DIR` for the same reason as the secret-leak case below:
# delegate-run reads ${CLAUDE_CONFIG_DIR:-$HOME/.claude}, so overriding HOME alone
# would let an operator's exported setting point this test at a real credential.
env -u CLAUDE_CONFIG_DIR HOME="$TMP/oauth-home" "$RUN" --role verify --author-vendor openai \
  --artifact worktree --claim "isolated OAuth review" --receipt "$TMP/oauth.json" \
  --config "$cfg" >/dev/null 2>"$TMP/oauth.err"
rc=$?
set -e
want_rc 0 "$rc" "OAuth receipt"
if grep -q -- '--bare' "$FAKE_ARGS_LOG"; then bad "OAuth route used --bare"; else ok; fi
if grep -q 'megapowers-delegate.*/claude-config' "$FAKE_CONFIG_LOG"; then ok; else bad "OAuth route did not use disposable config home"; fi
export ANTHROPIC_API_KEY=test-key

set +e
FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
  --artifact worktree --claim "find defects" --receipt "$TMP/needs.json" \
  --config "$cfg" >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 5 "$rc" "needs-attention exit"
want_jq "$TMP/needs.json" '.result.verdict == "needs_attention"' "needs-attention still writes valid receipt"

set +e
"$RUN" --role visual_verify --author-vendor openai --artifact worktree \
  --claim "rendered flow works" --receipt "$TMP/visual.json" --config "$cfg" \
  >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 2 "$rc" "visual review without screenshot fails"

# The role becomes a path component of the transcript slug, so it must not be able
# to carry a separator out of the transcript directory.
set +e
"$RUN" --role '../escape' --author-vendor openai --artifact worktree \
  --claim "a role must not escape the transcript directory" \
  --receipt "$TMP/badrole.json" --config "$cfg" >/dev/null 2>"$TMP/badrole.err"
rc=$?
set -e
want_rc 2 "$rc" "a role containing a path separator is rejected"
[ ! -e "$TMP/badrole.json" ] && ok || bad "a rejected role must write no receipt"

printf 'png-bytes\n' > "$TMP/screen.png"
set +e
"$RUN" --role visual_verify --author-vendor openai --artifact worktree \
  --claim "rendered flow works" --receipt "$TMP/visual.json" --config "$cfg" \
  --screenshot "$TMP/screen.png" >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "visual review with screenshot"
want_jq "$TMP/visual.json" '.evidence.screenshots | length == 1' "visual receipt has screenshot"
want_jq "$TMP/visual.json" '.evidence.screenshots[0].sha256 | test("^[0-9a-f]{64}$")' "visual screenshot is hashed"

set +e
FAKE_VERDICT=invalid "$RUN" --role verify --author-vendor openai \
  --artifact worktree --claim "invalid provider output" --receipt "$TMP/invalid.json" \
  --config "$cfg" >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 7 "$rc" "invalid provider JSON"
[ ! -e "$TMP/invalid.json" ] && ok || bad "invalid provider output wrote receipt"

set +e
FAKE_VERDICT=invalid-finding "$RUN" --role verify --author-vendor openai \
  --artifact worktree --claim "invalid finding" --receipt "$TMP/invalid-finding.json" \
  --config "$cfg" >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 7 "$rc" "schema-invalid finding"
[ ! -e "$TMP/invalid-finding.json" ] && ok || bad "schema-invalid finding wrote receipt"

# A real working tree can hold untracked entries that are not readable regular
# files. git surfaces dangling symlinks in `ls-files --others`, and both
# `hash-object` (review-diff-id) and an unreadable device node (delegate-run's
# package builder) exit fatally on them, which aborted the whole review. Verified
# against a checkout with character devices in the repository root and a dangling
# symlink here. Note git does NOT list fifos as untracked, and hashing one would
# block forever, so a fifo is not the case to test.
ln -sfn /nonexistent-target-xyz "$repo/dangling.link"
set +e
"$RUN" --role verify --author-vendor openai --artifact worktree \
  --claim "non-regular untracked entries must not abort the package" \
  --receipt "$TMP/nonregular.json" --config "$cfg" >/dev/null 2>"$TMP/nonregular.err"
rc=$?
set -e
want_rc 0 "$rc" "dangling untracked symlink does not abort the review package"
if grep -q 'fatal:' "$TMP/nonregular.err"; then
  bad "dangling untracked symlink leaked a git fatal error"
else
  ok
fi
want_jq "$TMP/nonregular.json" '.subject.kind == "worktree-diff"' "receipt still written with a non-regular entry present"

# The fingerprint must still MOVE when such an entry appears, or a stale receipt
# would survive a real tree change.
id_with="$("$DIFF_ID")"
rm -f "$repo/dangling.link"
id_without="$("$DIFF_ID")"
[ "$id_with" != "$id_without" ] && ok || bad "diff id must change when a dangling untracked symlink appears"

# Presence alone is too coarse. RETARGETING a dangling symlink in place changes the
# tree without adding or removing an entry, so a fingerprint that binds only the
# path would let an approval receipt survive it.
ln -sfn /nonexistent-target-xyz "$repo/dangling.link"
id_target_a="$("$DIFF_ID")"
ln -sfn /nonexistent-target-abc "$repo/dangling.link"
id_target_b="$("$DIFF_ID")"
[ "$id_target_a" != "$id_target_b" ] && ok || bad "diff id must change when a dangling symlink is retargeted in place"
rm -f "$repo/dangling.link"

# The dangling case bypasses -f entirely. A LIVE symlink to a readable regular
# file is the harder case: if -f is tested first it follows the link and hashes
# the target's content, so retargeting between two equal-content files moves
# nothing. Bind the link target itself.
printf 'identical\n' > "$repo/target-a.txt"
printf 'identical\n' > "$repo/target-b.txt"
ln -sfn target-a.txt "$repo/live.link"
id_live_a="$("$DIFF_ID")"
ln -sfn target-b.txt "$repo/live.link"
id_live_b="$("$DIFF_ID")"
[ "$id_live_a" != "$id_live_b" ] && ok || bad "diff id must change when a live symlink is retargeted between equal-content files"
rm -f "$repo/live.link" "$repo/target-a.txt" "$repo/target-b.txt"

# An UNREADABLE regular file also lands in the non-regular branch. Binding type
# plus size alone collides: rewriting its contents at the same length would leave
# the fingerprint unchanged, so a receipt would outlive a real content change.
printf 'AAAA' > "$repo/unreadable.bin"; chmod 000 "$repo/unreadable.bin"
id_unread_a="$("$DIFF_ID")"
chmod 644 "$repo/unreadable.bin"; printf 'BBBB' > "$repo/unreadable.bin"; chmod 000 "$repo/unreadable.bin"
id_unread_b="$("$DIFF_ID")"
chmod 644 "$repo/unreadable.bin"; rm -f "$repo/unreadable.bin"
[ "$id_unread_a" != "$id_unread_b" ] && ok || bad "diff id must change when an unreadable regular file is rewritten at the same size"

echo "== empty review package guard =="
# A committed fix leaves NOTHING pending, and `--artifact worktree` only captures
# uncommitted work, so the package degrades to bare section headings. Dispatching
# that burns a full reviewer round to be told the artifact was empty. Refuse
# before the provider is ever invoked.
clean="$TMP/clean-repo"
mkdir -p "$clean"
(
  cd "$clean" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'fixed\n' > svc.txt
  git add svc.txt
  git commit -qm 'fix: already committed'
) >/dev/null 2>&1
set +e
(
  cd "$clean" || exit 1
  FAKE_ARGS_LOG="$TMP/empty-args.txt" "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "the committed fix holds" \
    --receipt "$TMP/empty.json" --config "$cfg"
) >/dev/null 2>"$TMP/empty.err"
rc=$?
set -e
want_rc 8 "$rc" "clean worktree refuses to dispatch"
[ ! -e "$TMP/empty.json" ] && ok || bad "refused dispatch must write no receipt"
[ ! -e "$TMP/empty-args.txt" ] && ok || bad "refused dispatch must not reach the provider"
if grep -qi 'empty' "$TMP/empty.err"; then ok; else bad "refusal must name the empty package"; fi
if grep -qi 'commit' "$TMP/empty.err"; then ok; else bad "refusal must suggest the committed-range alternative"; fi

# The guard must key on substance, not on size. A one-line pending edit is a
# legitimately small package and must still dispatch.
printf 'one line pending\n' > "$clean/svc.txt"
set +e
(
  cd "$clean" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a one line change is still reviewable" \
    --receipt "$TMP/small.json" --config "$cfg"
) >/dev/null 2>"$TMP/small.err"
rc=$?
set -e
want_rc 0 "$rc" "a one line pending diff still dispatches"

# An untracked file alone is substance too: nothing tracked has changed, so both
# diffs are empty, but there is real work to review.
git -C "$clean" checkout -q -- svc.txt
printf 'brand new\n' > "$clean/added.txt"
set +e
(
  cd "$clean" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "an untracked addition is reviewable" \
    --receipt "$TMP/untracked-only.json" --config "$cfg"
) >/dev/null 2>"$TMP/untracked-only.err"
rc=$?
set -e
want_rc 0 "$rc" "an untracked file alone still dispatches"
rm -f "$clean/added.txt"

# A file artifact has no package to be empty, so the guard must not touch it.
printf 'plan text\n' > "$TMP/plan.md"
set +e
(
  cd "$clean" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact "$TMP/plan.md" \
    --claim "file artifacts are unaffected by the worktree guard" \
    --receipt "$TMP/file-artifact.json" --config "$cfg"
) >/dev/null 2>"$TMP/file-artifact.err"
rc=$?
set -e
want_rc 0 "$rc" "file artifact dispatches from a clean worktree"

echo "== round ledger =="
# Thirteen verify rounds ran against one artifact in a single session, six of them
# consecutive needs_attention, with nothing recording that a round was the
# seventh. The receipt must carry that count so a caller can cap the loop.
#
# The key is role plus BRANCH, and an approve clears it, so the number means
# consecutive rounds on this branch that did not reach approve. Keying on the
# artifact fingerprint instead made the counter blind to the very loop it exists to
# measure: the author fixes something between rounds, the fingerprint moves, and
# the seventh round reports 1.
rrepo="$TMP/rounds-repo"
mkdir -p "$rrepo"
(
  cd "$rrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending one\n' > svc.txt
) >/dev/null 2>&1
ledger="$rrepo/.git/megapowers-review-rounds.json"
subject_a="$(cd "$rrepo" && "$DIFF_ID")"
rbranch="$(git -C "$rrepo" symbolic-ref --short HEAD)"

run_round() {
  local receipt_out="$1" role_out="$2" err_out="$3"
  shift 3
  set +e
  (
    cd "$rrepo" || exit 1
    "$RUN" --role "$role_out" --author-vendor openai --artifact worktree \
      --claim "round accounting holds" --receipt "$receipt_out" --config "$cfg" "$@"
  ) >"${err_out%.err}.out" 2>"$err_out"
  local rc_out=$?
  set -e
  return "$rc_out"
}

# needs_attention throughout, because these are the rounds that accumulate. An
# approve is the terminator and is exercised on its own below.
#
# `run_round ...; want_rc N "$?"` cannot fail: run_round restores errexit and
# returns the code, so a nonzero round kills the suite at the call site, before the
# assertion and before the summary line. Catch the code instead.
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round1.json" verify "$TMP/round1.err" || rc=$?
want_rc 5 "$rc" "first round dispatches"
want_jq "$TMP/round1.json" '.round == 1' "first dispatch is round 1"
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round2.json" verify "$TMP/round2.err" || rc=$?
want_rc 5 "$rc" "second round dispatches"
want_jq "$TMP/round2.json" '.round == 2' "second dispatch on the same branch is round 2"

[ -f "$ledger" ] && ok || bad "round ledger must live beside the receipt in the git dir"
want_jq "$ledger" '.schema == "megapowers.review-rounds.v2"' "ledger declares the branch-keyed schema"
if jq -e --arg b "$rbranch" '.rounds.verify[$b] == 2' "$ledger" >/dev/null 2>&1; then ok; else bad "ledger must record 2 for this role and branch"; fi

# THE POINT OF THE KEY. The author fixes something between rounds, so the artifact
# fingerprint moves. A fingerprint-keyed counter reported 1 here, which is what made
# thirteen rounds look like a first pass every time.
printf 'pending two\n' > "$rrepo/svc.txt"
subject_b="$(cd "$rrepo" && "$DIFF_ID")"
[ "$subject_a" != "$subject_b" ] && ok || bad "test setup: subject id must have moved"
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round3.json" verify "$TMP/round3.err" || rc=$?
want_rc 5 "$rc" "round on the edited tree dispatches"
want_jq "$TMP/round3.json" '.round == 3' "an edit between rounds must not reset the count"
# The receipt still binds the fingerprint of the tree actually reviewed. Only the
# ledger key changed; subject.id is the provenance binding and stays the diff id.
if jq -e --arg id "$subject_b" '.subject.id == $id' "$TMP/round3.json" >/dev/null 2>&1; then ok; else bad "receipt must still bind the reviewed fingerprint"; fi

# A different role on the SAME branch counts separately.
printf 'png-bytes\n' > "$TMP/round-screen.png"
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round-visual.json" visual_verify "$TMP/round-visual.err" --screenshot "$TMP/round-screen.png" || rc=$?
want_rc 5 "$rc" "visual round dispatches"
want_jq "$TMP/round-visual.json" '.round == 1' "a different role restarts the count"

# The count is of DISPATCHES, not of receipts. A round that burned reviewer time
# and then failed still consumed a round, or the cap would undercount the loop.
set +e
(
  cd "$rrepo" || exit 1
  FAKE_VERDICT=invalid "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "failed dispatch still counts" --receipt "$TMP/round-fail.json" --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 7 "$rc" "invalid provider output still exits 7"
[ ! -e "$TMP/round-fail.json" ] && ok || bad "failed dispatch must write no receipt"
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round5.json" verify "$TMP/round5.err" || rc=$?
want_rc 5 "$rc" "round after a failed dispatch dispatches"
want_jq "$TMP/round5.json" '.round == 5' "a failed dispatch consumes a round"

# A refusal is NOT a dispatch, so it must not consume a round.
git -C "$rrepo" stash -q -u 2>/dev/null || git -C "$rrepo" checkout -q -- svc.txt
set +e
(
  cd "$rrepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "refusal must not count" --receipt "$TMP/round-refused.json" --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 8 "$rc" "clean rounds-repo refuses"
if jq -e --arg b "$rbranch" '.rounds.verify[$b] == 5' "$ledger" >/dev/null 2>&1; then ok; else bad "a refusal must not consume a round"; fi
git -C "$rrepo" stash pop -q 2>/dev/null || printf 'pending two\n' > "$rrepo/svc.txt"

# An approve ends the loop, so it clears the counter for that role and branch. The
# approving round still reports its own number: it was the sixth dispatch.
rc=0; run_round "$TMP/round-approve.json" verify "$TMP/round-approve.err" || rc=$?
want_rc 0 "$rc" "approving round dispatches"
want_jq "$TMP/round-approve.json" '.round == 6' "the approving round reports its own number"
if jq -e --arg b "$rbranch" '.rounds.verify | has($b) | not' "$ledger" >/dev/null 2>&1; then ok; else bad "an approve must clear the counter for this role and branch"; fi
# Only this role's entry. Another role's open loop on the same branch is untouched.
if jq -e --arg b "$rbranch" '.rounds.visual_verify[$b] == 1' "$ledger" >/dev/null 2>&1; then ok; else bad "an approve must not clear another role's count"; fi

rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round-after.json" verify "$TMP/round-after.err" || rc=$?
want_rc 5 "$rc" "round after an approve dispatches"
want_jq "$TMP/round-after.json" '.round == 1' "an approve resets the count to 1"

# Two branches count independently, or one branch's churn would cap another's.
git -C "$rrepo" checkout -q -b second-branch
rc=0; FAKE_VERDICT=needs_attention run_round "$TMP/round-branch2.json" verify "$TMP/round-branch2.err" || rc=$?
want_rc 5 "$rc" "round on a second branch dispatches"
want_jq "$TMP/round-branch2.json" '.round == 1' "a second branch starts its own count"
if jq -e --arg b "$rbranch" '.rounds.verify[$b] == 1' "$ledger" >/dev/null 2>&1; then ok; else bad "the first branch's count must survive work on a second branch"; fi
if jq -e '.rounds.verify["second-branch"] == 1' "$ledger" >/dev/null 2>&1; then ok; else bad "the second branch must have its own entry"; fi
git -C "$rrepo" checkout -q "$rbranch"

echo "== round key on a detached HEAD =="
# `git rev-parse --abbrev-ref HEAD` returns the literal string HEAD when detached,
# which would pool every unrelated detached session into one counter. Key those on
# the checked-out commit: stable across the working-tree edits a review loop
# actually makes, and distinct per checkout. Agents run detached often enough that
# leaving this undefined is not an option.
drepo="$TMP/detached-repo"
mkdir -p "$drepo"
(
  cd "$drepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'second\n' > svc.txt
  git add svc.txt
  git commit -qm second
  git checkout -q --detach HEAD
) >/dev/null 2>&1
dledger="$drepo/.git/megapowers-review-rounds.json"
# Untracked, so the package has substance and `git checkout` below stays clean.
printf 'pending work\n' > "$drepo/pending.txt"

run_detached() {
  local receipt_out="$1" err_out="$2"
  set +e
  (
    cd "$drepo" || exit 1
    FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
      --artifact worktree --claim "detached rounds accumulate" \
      --receipt "$receipt_out" --config "$cfg"
  ) >/dev/null 2>"$err_out"
  local rc_out=$?
  set -e
  return "$rc_out"
}

rc=0; run_detached "$TMP/det1.json" "$TMP/det1.err" || rc=$?
want_rc 5 "$rc" "first detached round dispatches"
want_jq "$TMP/det1.json" '.round == 1' "first detached dispatch is round 1"
printf 'pending work edited\n' > "$drepo/pending.txt"
rc=0; run_detached "$TMP/det2.json" "$TMP/det2.err" || rc=$?
want_rc 5 "$rc" "second detached round dispatches"
want_jq "$TMP/det2.json" '.round == 2' "detached rounds accumulate across an edit"
dhead="$(git -C "$drepo" rev-parse --short=12 HEAD)"
if jq -e --arg k "detached-$dhead" '.rounds.verify[$k] == 2' "$dledger" >/dev/null 2>&1; then ok; else bad "a detached HEAD must key on the checked-out commit"; fi
if jq -e '.rounds.verify | has("HEAD") | not' "$dledger" >/dev/null 2>&1; then ok; else bad "a detached HEAD must not key on the literal string HEAD"; fi

# A different detached checkout is a different counter, which is the whole reason
# the literal HEAD is unacceptable.
git -C "$drepo" checkout -q --detach HEAD~1
rc=0; run_detached "$TMP/det3.json" "$TMP/det3.err" || rc=$?
want_rc 5 "$rc" "round on a second detached commit dispatches"
want_jq "$TMP/det3.json" '.round == 1' "an unrelated detached checkout starts its own count"

# Outside a repository there is no branch to key on. The ledger already sits beside
# the receipt, so the count is scoped there and must still accumulate.
nrrepo="$TMP/no-repo-rounds"
mkdir -p "$nrrepo"
printf 'plan text\n' > "$nrrepo/plan.md"
run_no_repo() {
  set +e
  (
    cd "$TMP" || exit 1
    FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
      --artifact "$nrrepo/plan.md" --claim "rounds count outside a repository too" \
      --receipt "$1" --config "$cfg"
  ) >/dev/null 2>/dev/null
  local rc_out=$?
  set -e
  return "$rc_out"
}
rc=0; run_no_repo "$nrrepo/r1.json" || rc=$?
want_rc 5 "$rc" "first round outside a repository dispatches"
want_jq "$nrrepo/r1.json" '.round == 1' "first round outside a repository is 1"
rc=0; run_no_repo "$nrrepo/r2.json" || rc=$?
want_rc 5 "$rc" "second round outside a repository dispatches"
want_jq "$nrrepo/r2.json" '.round == 2' "rounds accumulate outside a repository"
if jq -e '.rounds.verify["no-branch"] == 2' "$nrrepo/megapowers-review-rounds.json" >/dev/null 2>&1; then ok; else bad "outside a repository the count keys on no-branch"; fi

echo "== round ledger schema migration =="
# A v1 ledger keyed rounds on the diff fingerprint. Nothing maps a fingerprint to a
# branch, so migrating would be inventing data. Discard the old counts, say so, and
# start the branch at 1 rather than reading old entries under a key they never had.
migrepo="$TMP/migrate-repo"
mkdir -p "$migrepo"
(
  cd "$migrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
migledger="$migrepo/.git/megapowers-review-rounds.json"
migbranch="$(git -C "$migrepo" symbolic-ref --short HEAD)"
printf '%s\n' '{"schema":"megapowers.review-rounds.v1","rounds":{"verify":{"deadbeefdeadbeef":6}},"updated_at":"2026-07-01T00:00:00Z"}' > "$migledger"
rc=0
(
  cd "$migrepo" || exit 1
  FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "an old ledger must not be read under the new key" \
    --receipt "$TMP/mig.json" --config "$cfg"
) >/dev/null 2>"$TMP/mig.err" || rc=$?
want_rc 5 "$rc" "a run against a v1 ledger dispatches"
want_jq "$TMP/mig.json" '.round == 1' "a v1 ledger's counts are discarded, not carried over"
if grep -qi 'predates' "$TMP/mig.err"; then ok; else bad "discarding an old ledger must be said out loud"; fi
want_jq "$migledger" '.schema == "megapowers.review-rounds.v2"' "the rewritten ledger declares v2"
if jq -e '.rounds.verify | has("deadbeefdeadbeef") | not' "$migledger" >/dev/null 2>&1; then ok; else bad "v1 fingerprint entries must not survive into v2"; fi
if jq -e --arg b "$migbranch" '.rounds.verify[$b] == 1' "$migledger" >/dev/null 2>&1; then ok; else bad "the v2 ledger must record the branch key"; fi

# The verdict block is what a human or agent actually reads. It must name the
# round, and it must NOT be on stdout: stdout is the bare receipt JSON and a
# caller may pipe it straight into jq.
if grep -q '=== VERDICT ===' "$TMP/round2.err"; then ok; else bad "verdict block must be printed"; fi
if grep -qi 'round' "$TMP/round2.err"; then ok; else bad "verdict block must name the round"; fi
if jq -e . "$TMP/round2.out" >/dev/null 2>&1; then ok; else bad "stdout must stay parseable JSON"; fi

echo "== concurrent round reservation =="
# Concurrent dispatches on one branch must each reserve a DISTINCT round. A
# read-modify-write without a mutex hands the same number to both. needs_attention
# so none of them clears the counter mid-race.
crepo="$TMP/concurrent-repo"
mkdir -p "$crepo"
(
  cd "$crepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
for n in 1 2 3 4 5; do
  (
    cd "$crepo" || exit 1
    FAKE_ARGS_LOG="$TMP/conc-args-$n.txt" FAKE_VERDICT=needs_attention \
      "$RUN" --role verify --author-vendor openai \
      --artifact worktree --claim "concurrent round $n" \
      --receipt "$TMP/conc-$n.json" --config "$cfg"
  ) >/dev/null 2>"$TMP/conc-$n.err" &
done
wait
conc_rounds="$(jq -r '.round' "$TMP"/conc-*.json 2>/dev/null | LC_ALL=C sort -n | tr '\n' ' ')"
if [ "$conc_rounds" = "1 2 3 4 5 " ]; then ok; else bad "five concurrent dispatches must reserve rounds 1..5, got: $conc_rounds"; fi
if jq -e '.rounds.verify | to_entries | .[0].value == 5' "$crepo/.git/megapowers-review-rounds.json" >/dev/null 2>&1; then ok; else bad "ledger must settle at 5 after five concurrent dispatches"; fi

echo "== round ledger write failures =="
# A failed ledger write must refuse the dispatch, not reset the count. jq exiting
# nonzero, or being OOM-killed, left an empty file installed over the ledger; the
# next run read that back as '{}' and reserved round 1 again, so a receipt claimed
# round 1 for the seventh dispatch. That is a false provenance claim in the
# direction that matters.
wrepo="$TMP/write-fail-repo"
mkdir -p "$wrepo"
(
  cd "$wrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
wledger="$wrepo/.git/megapowers-review-rounds.json"
wbranch="$(git -C "$wrepo" symbolic-ref --short HEAD)"

# needs_attention throughout, so the counter stays open and a reset cannot be
# mistaken for the clobber under test.
rc=0
(
  cd "$wrepo" || exit 1
  FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "round one before the write fails" \
    --receipt "$TMP/wf1.json" --config "$cfg"
) >/dev/null 2>/dev/null || rc=$?
want_rc 5 "$rc" "round before the ledger write fails dispatches"
want_jq "$TMP/wf1.json" '.round == 1' "pre-failure dispatch is round 1"

# Fail only the ledger write. The stub matches the round-ledger schema string,
# which no other jq invocation in the launcher passes, so every other call reaches
# the real jq untouched.
stub_dir="$TMP/jq-stub"
mkdir -p "$stub_dir"
real_jq="$(command -v jq)"
cat > "$stub_dir/jq" <<EOF
#!/usr/bin/env bash
for arg in "\$@"; do
  case "\$arg" in
    *megapowers.review-rounds.v2*) exit 1 ;;
  esac
done
exec "$real_jq" "\$@"
EOF
chmod +x "$stub_dir/jq"

rc=0
(
  cd "$wrepo" || exit 1
  PATH="$stub_dir:$PATH" "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "the ledger write fails" \
    --receipt "$TMP/wf2.json" --config "$cfg"
) >/dev/null 2>"$TMP/wf2.err" || rc=$?
want_rc 2 "$rc" "a failed ledger write must refuse to dispatch"
[ ! -e "$TMP/wf2.json" ] && ok || bad "a failed ledger write must write no receipt"
if grep -q 'correct round number' "$TMP/wf2.err"; then ok; else bad "a failed ledger write must name the round number as the reason"; fi
if jq -e --arg b "$wbranch" '.rounds.verify[$b] == 1' "$wledger" >/dev/null 2>&1; then ok; else bad "a failed ledger write must leave the ledger intact"; fi

# The count must carry on from where it was, not restart at 1.
rc=0
(
  cd "$wrepo" || exit 1
  FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "the count must not have reset" \
    --receipt "$TMP/wf3.json" --config "$cfg"
) >/dev/null 2>/dev/null || rc=$?
want_rc 5 "$rc" "round after a failed ledger write dispatches"
want_jq "$TMP/wf3.json" '.round == 2' "a failed ledger write must not reset the round count"

# A ledger that does not parse must not read back as no prior rounds either. That
# is the same false claim arriving by a different route: the count restarts at 1
# and a later receipt understates the loop.
printf '{"rounds":{"verify":{"tru' > "$wledger"
rc=0
(
  cd "$wrepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a corrupt ledger must not silently reset" \
    --receipt "$TMP/wf4.json" --config "$cfg"
) >/dev/null 2>"$TMP/wf4.err" || rc=$?
want_rc 2 "$rc" "a corrupt round ledger refuses to dispatch"
[ ! -e "$TMP/wf4.json" ] && ok || bad "a corrupt round ledger must write no receipt"
if grep -q 'not readable JSON' "$TMP/wf4.err"; then ok; else bad "a corrupt round ledger must be named as the reason"; fi

# An empty ledger is the case a plain `jq .` misses, because jq exits 0 on empty
# input and prints nothing, which then reads as no rounds.
: > "$wledger"
rc=0
(
  cd "$wrepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "an empty ledger must not silently reset" \
    --receipt "$TMP/wf5.json" --config "$cfg"
) >/dev/null 2>"$TMP/wf5.err" || rc=$?
want_rc 2 "$rc" "an empty round ledger refuses to dispatch"
[ ! -e "$TMP/wf5.json" ] && ok || bad "an empty round ledger must write no receipt"
if grep -q 'not readable JSON' "$TMP/wf5.err"; then ok; else bad "an empty round ledger must be named as the reason, not reported as an unreservable round"; fi

# An unwritable ledger directory fails the lock acquisition identically to a taken
# lock. Stalling for 10 seconds and then blaming a lock that does not exist, with
# instructions to remove it, sends the operator after the wrong thing.
nrepo="$TMP/no-write-ledger"
mkdir -p "$nrepo"
printf 'plan text\n' > "$TMP/ro-plan.md"
chmod 500 "$nrepo"
rc=0
(
  cd "$TMP" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact "$TMP/ro-plan.md" \
    --claim "an unwritable ledger directory is not lock contention" \
    --receipt "$nrepo/receipt.json" --config "$cfg"
) >/dev/null 2>"$TMP/nowrite.err" || rc=$?
chmod 700 "$nrepo"
want_rc 2 "$rc" "an unwritable ledger directory refuses to dispatch"
if grep -q 'cannot write the round ledger' "$TMP/nowrite.err"; then ok; else bad "an unwritable ledger directory must be named as such"; fi
if grep -qi 'locked for 10 seconds' "$TMP/nowrite.err"; then bad "an unwritable ledger directory must not be reported as lock contention"; else ok; fi

echo "== transcript retention =="
# Thirteen cross-vendor reviews left zero durable reviewer transcripts, because
# the launcher points the provider at a scratch home and then removes it. Only the
# receipt survived, so what the reviewer actually read was unauditable.
if "$RUN" --help 2>&1 | grep -q -- '--transcript-dir'; then ok; else bad "--help must list --transcript-dir"; fi

trepo="$TMP/transcript-repo"
mkdir -p "$trepo"
(
  cd "$trepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'a distinctive pending line\n' > svc.txt
) >/dev/null 2>&1

tdir="$TMP/transcripts"
set +e
(
  cd "$trepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a distinctive claim string" --receipt "$TMP/tr.json" \
    --config "$cfg" --transcript-dir "$tdir"
) >/dev/null 2>"$TMP/tr.err"
rc=$?
set -e
want_rc 0 "$rc" "run with --transcript-dir succeeds"
tsub="$(find "$tdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
[ -n "$tsub" ] && ok || bad "transcript dir must hold this run's transcript"
[ -f "$tsub/prompt.txt" ] && ok || bad "transcript must retain the prompt"
[ -f "$tsub/review-package.txt" ] && ok || bad "transcript must retain the review package"
[ -f "$tsub/provider-raw.json" ] && ok || bad "transcript must retain the raw provider output"
if grep -q 'a distinctive claim string' "$tsub/prompt.txt" 2>/dev/null; then ok; else bad "retained prompt must be what was sent"; fi
if grep -q 'a distinctive pending line' "$tsub/review-package.txt" 2>/dev/null; then ok; else bad "retained package must be what the reviewer read"; fi
if grep -q 'megapowers-review-rounds' "$tsub/route.txt" 2>/dev/null; then bad "route.txt must not be the ledger"; else ok; fi
if grep -q 'fake-frontier' "$tsub/route.txt" 2>/dev/null; then ok; else bad "transcript must record which model was dispatched"; fi

# Rounds must not overwrite each other, or the flag destroys the very audit trail
# it exists to create.
printf 'a second pending line\n' > "$trepo/svc.txt"
set +e
(
  cd "$trepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "second round claim" --receipt "$TMP/tr2.json" \
    --config "$cfg" --transcript-dir "$tdir"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "second transcript run succeeds"
tcount="$(find "$tdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)"
[ "$tcount" -eq 2 ] && ok || bad "a second round must not overwrite the first transcript, got $tcount"

# An approve clears the round counter, so role plus subject plus round is no longer
# unique for all time: re-reviewing an UNCHANGED tree after an approve produces
# round 1 twice. The second transcript must not overwrite the first, which is the
# invariant the counter reset broke.
sametree="$TMP/same-tree-transcripts"
printf 'one fixed pending line\n' > "$trepo/svc.txt"
for tpass in 1 2; do
  set +e
  (
    cd "$trepo" || exit 1
    "$RUN" --role verify --author-vendor openai --artifact worktree \
      --claim "approve pass $tpass on an unchanged tree" \
      --receipt "$TMP/tr-same-$tpass.json" --config "$cfg" --transcript-dir "$sametree"
  ) >/dev/null 2>"$TMP/tr-same-$tpass.err"
  rc=$?
  set -e
  want_rc 0 "$rc" "approving pass $tpass on an unchanged tree succeeds"
  want_jq "$TMP/tr-same-$tpass.json" '.round == 1' "an approve leaves the next round at 1"
done
samecount="$(find "$sametree" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)"
[ "$samecount" -eq 2 ] && ok || bad "two round-1 dispatches must not share a transcript directory, got $samecount"

# Absent the flag, nothing is retained: behavior is unchanged from today.
empty_dir="$TMP/no-transcripts"
mkdir -p "$empty_dir"
set +e
(
  cd "$trepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "no transcript requested" --receipt "$TMP/tr3.json" --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "run without --transcript-dir succeeds"
[ -z "$(ls -A "$empty_dir")" ] && ok || bad "no transcript must be written without the flag"

# A failed dispatch is exactly when the transcript matters, because no receipt is
# written at all.
ftdir="$TMP/failed-transcript"
set +e
(
  cd "$trepo" || exit 1
  FAKE_VERDICT=invalid "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "failed dispatch is still auditable" --receipt "$TMP/tr-fail.json" \
    --config "$cfg" --transcript-dir "$ftdir"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 7 "$rc" "invalid provider output still exits 7 with a transcript dir"
[ ! -e "$TMP/tr-fail.json" ] && ok || bad "failed dispatch must write no receipt"
fsub="$(find "$ftdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
if [ -n "$fsub" ] && [ -f "$fsub/prompt.txt" ] && [ -f "$fsub/provider-raw.json" ]; then ok; else bad "a failed dispatch must still retain its transcript"; fi

# Retention failures must not be silent, or the audit trail is absent on exactly
# the run where it mattered. A transcript directory that exists but is not writable
# passes the up-front check, because `mkdir -p` succeeds on an existing directory,
# and only fails when the per-dispatch subdirectory is created.
rodir="$TMP/readonly-transcripts"
mkdir -p "$rodir"
chmod 500 "$rodir"
set +e
(
  cd "$trepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a retention failure must be reported" --receipt "$TMP/tr-ro.json" \
    --config "$cfg" --transcript-dir "$rodir"
) >/dev/null 2>"$TMP/tr-ro.err"
rc=$?
set -e
chmod 700 "$rodir"
want_rc 0 "$rc" "an unwritable transcript directory does not fail the review"
if grep -q 'could not retain the transcript' "$TMP/tr-ro.err"; then ok; else bad "a failed transcript retention must warn on stderr"; fi

# The scratch home holds a REAL credential file on the OAuth route. Retention
# copies an allowlist, so a secret must never reach the transcript directory.
#
# Overriding HOME alone does not put this test in charge of the credential:
# delegate-run reads ${CLAUDE_CONFIG_DIR:-$HOME/.claude}, and CLAUDE_CONFIG_DIR is
# a documented Claude Code setting an operator may well have exported. When it
# wins, the run reads the operator's real credential, never touches the planted
# marker, and the grep below passes without testing anything. Export a decoy here
# so the hole cannot come back invisibly, and pin the provenance: the disposable
# config must hold what this test planted.
unset ANTHROPIC_API_KEY
export CLAUDE_CONFIG_DIR="$TMP/decoy-config"
mkdir -p "$CLAUDE_CONFIG_DIR"
printf '{"token":"DECOY-MUST-NOT-BE-READ"}\n' > "$CLAUDE_CONFIG_DIR/.credentials.json"
export FAKE_CRED_LOG="$TMP/claude-cred.json"
mkdir -p "$TMP/secret-home/.claude"
printf '{"token":"SUPERSECRET-OAUTH-MARKER"}\n' > "$TMP/secret-home/.claude/.credentials.json"
stdir="$TMP/secret-transcript"
set +e
(
  cd "$trepo" || exit 1
  env -u CLAUDE_CONFIG_DIR HOME="$TMP/secret-home" "$RUN" --role verify \
    --author-vendor openai --artifact worktree \
    --claim "credentials must not leak" --receipt "$TMP/tr-secret.json" \
    --config "$cfg" --transcript-dir "$stdir"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "OAuth route with a transcript dir succeeds"
if grep -q 'SUPERSECRET-OAUTH-MARKER' "$FAKE_CRED_LOG" 2>/dev/null; then ok; else bad "the OAuth route must have handled the planted credential, not the operator's"; fi
if grep -q 'DECOY-MUST-NOT-BE-READ' "$FAKE_CRED_LOG" 2>/dev/null; then bad "an exported CLAUDE_CONFIG_DIR overrode the test's own credential"; else ok; fi
if grep -rq 'SUPERSECRET-OAUTH-MARKER' "$stdir" 2>/dev/null; then
  bad "transcript retention leaked a credential"
else
  ok
fi
if find "$stdir" -name '.credentials.json' 2>/dev/null | grep -q .; then bad "transcript retained the credential file"; else ok; fi
unset CLAUDE_CONFIG_DIR FAKE_CRED_LOG
export ANTHROPIC_API_KEY=test-key

echo "== subject id and review package come from one snapshot =="
# The fingerprint used to be taken from the live worktree BEFORE the package was
# built out of that same mutable tree. Two reads of one mutable thing, so:
# fingerprint tree A, let the tree become B before the package capture, restore A
# once the capture is done. The reviewer then approves B while the receipt names
# A, and the Stop hook fingerprints A, matches, and lets through a tree nobody
# reviewed. Build the race rather than reason about it: a git shim swaps the tree
# when the capture starts reading and swaps it back when the capture stops.
racerepo="$TMP/race-repo"
mkdir -p "$racerepo"
(
  cd "$racerepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'state A\n' > svc.txt
) >/dev/null 2>&1
race_id_a="$(cd "$racerepo" && "$DIFF_ID")"

race_shim="$TMP/race-shim"
mkdir -p "$race_shim/state"
real_git="$(command -v git)"
# `status --short` is the launcher's first read of the worktree and nothing else
# in either the old or the new code path issues it, so it marks the moment the
# capture begins. `--others` is the last, so it marks the moment it ends.
cat > "$race_shim/git" <<EOF
#!/usr/bin/env bash
args=" \$* "
case "\$args" in
  *" status "*)
    if [ ! -e "$race_shim/state/mutated" ]; then
      : > "$race_shim/state/mutated"
      printf 'state B\n' > "$racerepo/svc.txt"
    fi
    ;;
esac
"$real_git" "\$@"
rc=\$?
case "\$args" in
  *" --others "*)
    if [ -e "$race_shim/state/mutated" ] && [ ! -e "$race_shim/state/restored" ]; then
      : > "$race_shim/state/restored"
      printf 'state A\n' > "$racerepo/svc.txt"
    fi
    ;;
esac
exit \$rc
EOF
chmod +x "$race_shim/git"

rdir="$TMP/race-transcripts"
set +e
(
  cd "$racerepo" || exit 1
  PATH="$race_shim:$PATH" "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the receipt must name the tree the reviewer read" \
    --receipt "$TMP/race.json" --config "$cfg" --transcript-dir "$rdir"
) >/dev/null 2>"$TMP/race.err"
rc=$?
set -e
want_rc 0 "$rc" "the raced dispatch completes"
[ -e "$race_shim/state/mutated" ] && ok || bad "test setup: the shim never swapped the tree"
[ -e "$race_shim/state/restored" ] && ok || bad "test setup: the shim never restored the tree"
rsub="$(find "$rdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
if grep -q 'state B' "$rsub/review-package.txt" 2>/dev/null; then ok; else bad "test setup: the reviewer did not read the swapped tree"; fi
if grep -q 'state A' "$rsub/review-package.txt" 2>/dev/null; then bad "the package mixes both trees, so no single id can name it"; else ok; fi
# The tree the reviewer actually read, fingerprinted on its own terms.
printf 'state B\n' > "$racerepo/svc.txt"
race_id_b="$(cd "$racerepo" && "$DIFF_ID")"
printf 'state A\n' > "$racerepo/svc.txt"
[ "$race_id_a" != "$race_id_b" ] && ok || bad "test setup: the two tree states must have different ids"
if jq -e --arg id "$race_id_b" '.subject.id == $id' "$TMP/race.json" >/dev/null 2>&1; then ok; else bad "the receipt must name the tree the reviewer read, not the tree present before the capture"; fi
if jq -e --arg id "$race_id_a" '.subject.id == $id' "$TMP/race.json" >/dev/null 2>&1; then bad "the receipt named a tree that was never reviewed"; else ok; fi

# The retained stream is the exact byte sequence the id was computed over, so a
# reader can recompute the binding without trusting this script.
if [ -f "$rsub/subject-stream.bin" ]; then ok; else bad "the transcript must retain the bytes the subject id was computed from"; fi
if [ -f "$rsub/subject-stream.bin" ] &&
   [ "$(git -C "$racerepo" hash-object --stdin < "$rsub/subject-stream.bin")" = "$(jq -r '.subject.id' "$TMP/race.json")" ]; then
  ok
else
  bad "the retained stream must hash to the receipt's subject id"
fi

# A file artifact is read a second time by the reviewer for the same reason, so
# it is captured too and the reviewer is pointed at the capture.
printf 'original plan\n' > "$TMP/race-plan.md"
sdir="$TMP/race-file-transcripts"
set +e
(
  cd "$TMP" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact "$TMP/race-plan.md" \
    --claim "a file artifact is bound to a capture too" --receipt "$TMP/race-file.json" \
    --config "$cfg" --transcript-dir "$sdir"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "file artifact dispatch succeeds"
ssub="$(find "$sdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
if grep -q 'megapowers-delegate.*snapshot/artifact' "$ssub/prompt.txt" 2>/dev/null; then ok; else bad "a file artifact reviewer must be pointed at the immutable capture"; fi

echo "== id agreement with review-diff-id =="
# The launcher now derives the id itself instead of shelling out to
# review-diff-id, so the two implementations can drift. They must not: the Stop
# hook fingerprints with review-diff-id and compares against what was written
# here, and any disagreement makes every receipt unusable. Walk the tree shapes
# the pair is expected to survive and assert they agree at each step.
eqrepo="$TMP/id-agreement-repo"
mkdir -p "$eqrepo"
(
  cd "$eqrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  printf 'placeholder\n' > .env.example
  printf '*.crlf text eol=crlf\n' > .gitattributes
  git add svc.txt .env.example .gitattributes
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1

eq_case=0
assert_id_agrees() {
  local desc="$1"
  eq_case=$((eq_case + 1))
  set +e
  (
    cd "$eqrepo" || exit 1
    "$RUN" --role verify --author-vendor openai --artifact worktree \
      --claim "id agreement $desc" --receipt "$TMP/eq-$eq_case.json" --config "$cfg"
  ) >/dev/null 2>"$TMP/eq-$eq_case.err"
  local rc_out=$?
  set -e
  if [ "$rc_out" -ne 0 ]; then
    bad "$desc: dispatch failed with rc=$rc_out"
    return
  fi
  local live
  live="$(cd "$eqrepo" && "$DIFF_ID")"
  if jq -e --arg id "$live" '.subject.id == $id' "$TMP/eq-$eq_case.json" >/dev/null 2>&1; then
    ok
  else
    bad "$desc: launcher id and review-diff-id disagree"
  fi
}

assert_id_agrees "a plain pending edit"
printf 'staged\n' > "$eqrepo/staged.txt"; git -C "$eqrepo" add staged.txt
assert_id_agrees "a staged addition"
mkdir -p "$eqrepo/sub/deep"; printf 'nested\n' > "$eqrepo/sub/deep/n.txt"
assert_id_agrees "an untracked file in a nested directory"
ln -sfn /nonexistent-target-xyz "$eqrepo/dangle.link"
assert_id_agrees "a dangling untracked symlink"
printf 'tgt\n' > "$eqrepo/tgt.txt"; ln -sfn tgt.txt "$eqrepo/live.link"
assert_id_agrees "a live untracked symlink"
printf 'AAAA' > "$eqrepo/unreadable.bin"; chmod 000 "$eqrepo/unreadable.bin"
assert_id_agrees "an unreadable untracked regular file"
chmod 644 "$eqrepo/unreadable.bin"
# A clean filter is chosen from the file's location, so hashing a copy of it
# without restoring that location silently produces a different id.
printf 'a\r\nb\r\n' > "$eqrepo/converted.crlf"
assert_id_agrees "an untracked file under an eol clean filter"
rm -f "$eqrepo/.env.example"; mkfifo "$eqrepo/.env.example"
assert_id_agrees "a tracked path replaced by a fifo"

echo "== unreadable tracked paths =="
# The sandbox bind mounts /dev/null over deny-listed paths, which turns a tracked
# file like .env.example into a character device. The Stop hook and review-diff-id
# both retry with the path excluded; the launcher's package builder did not, so it
# exited 128 before dispatch and the receipt the gate demands could not be
# produced at all, in exactly the condition this branch was written for.
frepo="$TMP/fifo-repo"
mkdir -p "$frepo"
(
  cd "$frepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  printf 'placeholder\n' > .env.example
  git add svc.txt .env.example
  git commit -qm init
  printf 'pending\n' > svc.txt
  rm .env.example
  mkfifo .env.example
) >/dev/null 2>&1
fdir="$TMP/fifo-transcripts"
set +e
(
  cd "$frepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a tracked non-regular path must not block the remedy" \
    --receipt "$TMP/fifo.json" --config "$cfg" --transcript-dir "$fdir"
) >/dev/null 2>"$TMP/fifo.err"
rc=$?
set -e
want_rc 0 "$rc" "a tracked fifo still produces a receipt"
if grep -q 'unsupported file type' "$TMP/fifo.err"; then bad "a tracked fifo leaked a git fatal error"; else ok; fi
if jq -e --arg id "$(cd "$frepo" && "$DIFF_ID")" '.subject.id == $id' "$TMP/fifo.json" >/dev/null 2>&1; then ok; else bad "the receipt must carry the id the Stop hook will compute"; fi
fsub2="$(find "$fdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
# Excluding a path keeps the package computable, but it also takes that path out
# of it, so its identity has to be stated or two trees differing only there would
# read as the same review.
if grep -q '\.env\.example (excluded' "$fsub2/review-package.txt" 2>/dev/null; then ok; else bad "an excluded tracked path must be named in the package"; fi
if grep -qi 'fifo' "$fsub2/review-package.txt" 2>/dev/null; then ok; else bad "an excluded tracked path must carry its identity record"; fi
# And the fingerprint must still move when that path's identity changes, or an
# approval would survive swapping one non-regular kind for another.
id_fifo_a="$(cd "$frepo" && "$DIFF_ID")"
rm -f "$frepo/.env.example"; mkfifo "$frepo/.env.example"
id_fifo_b="$(cd "$frepo" && "$DIFF_ID")"
[ "$id_fifo_a" != "$id_fifo_b" ] && ok || bad "recreating an excluded tracked path must stale the receipt"

# A section that stays unreadable after the retry is not a smaller review, it is
# no review. Refuse with the path named, and refuse before a round is reserved:
# nothing was dispatched, so nothing should be counted.
urepo="$TMP/unreadable-repo"
mkdir -p "$urepo"
(
  cd "$urepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > secret.txt
  git add secret.txt
  git commit -qm init
  printf 'pending\n' > secret.txt
  chmod 000 secret.txt
) >/dev/null 2>&1
set +e
(
  cd "$urepo" || exit 1
  FAKE_ARGS_LOG="$TMP/unreadable-args.txt" "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "an unreadable tracked file is not reviewable" \
    --receipt "$TMP/unreadable.json" --config "$cfg"
) >/dev/null 2>"$TMP/unreadable.err"
rc=$?
set -e
chmod 644 "$urepo/secret.txt"
want_rc 9 "$rc" "an unreadable tracked regular file refuses to dispatch"
[ ! -e "$TMP/unreadable.json" ] && ok || bad "a failed capture must write no receipt"
[ ! -e "$TMP/unreadable-args.txt" ] && ok || bad "a failed capture must not reach the provider"
if grep -q 'secret.txt' "$TMP/unreadable.err"; then ok; else bad "a failed capture must name the path git could not read"; fi
if [ ! -e "$urepo/.git/megapowers-review-rounds.json" ]; then ok; else bad "a failed capture must not consume a round"; fi

echo "== every exit is one the exit map names =="
# The exit codes are a contract a harness branches on. git writes nothing and
# exits 128 when it cannot hash a path, and any unguarded git call under errexit
# hands that 128 straight to the caller, which has no case for it: it reads as an
# unrecognized failure rather than as "the package could not be built", which is
# the confusion the map exists to prevent. Walk the tree shapes that make git
# abort and assert the launcher always answers with a documented code.
hostile_case() {
  local name="$1" setup="$2"
  local hrepo="$TMP/hostile-$name"
  mkdir -p "$hrepo"
  (
    cd "$hrepo" || exit 1
    git init -q
    git config user.email test@example.com
    git config user.name test
    git config commit.gpgsign false
    eval "$setup"
  ) >/dev/null 2>&1
  set +e
  (
    cd "$hrepo" || exit 1
    "$RUN" --role verify --author-vendor openai --artifact worktree \
      --claim "exit codes stay inside the map" --config "$cfg"
  ) >/dev/null 2>"$TMP/hostile-$name.err"
  local rc_out=$?
  set -e
  chmod -R u+rwX "$hrepo" 2>/dev/null
  case "$rc_out" in
    0|2|3|5|6|7|8|9) ok ;;
    *) bad "$name: exited $rc_out, which the exit map does not name" ;;
  esac
  # A raw git status is the specific regression. Name it separately so the
  # failure says what happened rather than only that a number was wrong.
  if [ "$rc_out" = "128" ]; then bad "$name: a bare git status leaked to the caller"; else ok; fi
}

hostile_case tracked-fifo \
  'printf a > svc.txt; printf x > .env.example; git add svc.txt .env.example; git commit -qm i; printf b > svc.txt; rm .env.example; mkfifo .env.example'
hostile_case untracked-fifo \
  'printf a > svc.txt; git add svc.txt; git commit -qm i; printf b > svc.txt; mkfifo pipe.fifo'
hostile_case unreadable-tracked \
  'printf a > svc.txt; git add svc.txt; git commit -qm i; printf b > svc.txt; chmod 000 svc.txt'
hostile_case unreadable-untracked \
  'printf a > svc.txt; git add svc.txt; git commit -qm i; printf b > svc.txt; printf c > u.bin; chmod 000 u.bin'
hostile_case tracked-replaced-by-dir \
  'printf a > p.txt; git add p.txt; git commit -qm i; rm p.txt; mkdir p.txt; printf x > p.txt/inner'
# A repository with no commits has no HEAD, which is an ordinary state an agent
# reaches on a fresh scaffold, not a hostile one. It must not answer with a raw
# git status either.
hostile_case no-commits 'printf a > svc.txt'
hostile_case no-commits-staged 'printf a > svc.txt; git add svc.txt'

# The one that must be 9 specifically: a section that stays unreadable after the
# exclusion retry is a failed capture, and it has to say so with its own code
# rather than borrowing 2 (usage) or 3 (already carrying two meanings).
if grep -q 'cannot capture' "$TMP/hostile-unreadable-tracked.err"; then ok; else bad "a failed capture must name itself as one"; fi

echo "== concurrent verdicts resolve in dispatch order =="
# Reserving atomically does not make concurrent COMPLETION correct. A slow round 1
# that approves and a fast round 2 that does not: round 2 finishes unresolved,
# then round 1's approve deletes the whole branch key and the next dispatch
# restarts at 1 although the newest dispatch did not approve.
mrepo="$TMP/mixed-repo"
mkdir -p "$mrepo"
(
  cd "$mrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
mledger="$mrepo/.git/megapowers-review-rounds.json"
mbranch="$(git -C "$mrepo" symbolic-ref --short HEAD)"

(
  cd "$mrepo" || exit 1
  FAKE_WAIT_FOR="$TMP/mixed-gate" FAKE_ARGS_LOG="$TMP/mixed-a-args.txt" \
    "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the slow approving round" --receipt "$TMP/mixed-a.json" --config "$cfg"
) >/dev/null 2>"$TMP/mixed-a.err" &
mpid=$!
# Wait for the slow round to hold its reservation, so the ordering under test is
# the one that is actually built rather than whichever the scheduler picks.
mwait=0
while [ "$mwait" -lt 200 ]; do
  if jq -e --arg b "$mbranch" '.open.verify[$b] == [1]' "$mledger" >/dev/null 2>&1; then break; fi
  mwait=$((mwait + 1))
  sleep 0.05
done
[ "$mwait" -lt 200 ] && ok || bad "the slow round never reserved round 1"

set +e
(
  cd "$mrepo" || exit 1
  FAKE_VERDICT=needs_attention FAKE_ARGS_LOG="$TMP/mixed-b-args.txt" \
    "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the fast refuting round" --receipt "$TMP/mixed-b.json" --config "$cfg"
) >/dev/null 2>"$TMP/mixed-b.err"
rc=$?
set -e
want_rc 5 "$rc" "the fast round finishes first"
want_jq "$TMP/mixed-b.json" '.round == 2' "the fast round is round 2"

: > "$TMP/mixed-gate"
set +e
wait "$mpid"
rc=$?
set -e
want_rc 0 "$rc" "the slow approving round finishes second"
want_jq "$TMP/mixed-a.json" '.round == 1' "the slow round kept round 1"
# The newest dispatch did not approve, so the count must survive the older
# approve that landed after it.
if jq -e --arg b "$mbranch" '.rounds.verify[$b] == 2' "$mledger" >/dev/null 2>&1; then ok; else bad "an approve completing out of order must not clear the count"; fi
set +e
(
  cd "$mrepo" || exit 1
  FAKE_VERDICT=needs_attention "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "the round after the mixed pair" \
    --receipt "$TMP/mixed-c.json" --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 5 "$rc" "the round after the mixed pair dispatches"
want_jq "$TMP/mixed-c.json" '.round == 3' "the count carries on past an out-of-order approve"

# A panel is one round and it resolves only when every lens has reported, so a
# panel in which every lens approves does clear the count no matter what order
# the lenses finish in.
prepo="$TMP/panel-repo"
mkdir -p "$prepo"
(
  cd "$prepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
pledger="$prepo/.git/megapowers-review-rounds.json"
pbranch="$(git -C "$prepo" symbolic-ref --short HEAD)"
(
  cd "$prepo" || exit 1
  FAKE_WAIT_FOR="$TMP/panel-gate" FAKE_ARGS_LOG="$TMP/panel-1-args.txt" \
    "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "panel lens one" --receipt "$TMP/panel-1.json" --config "$cfg"
) >/dev/null 2>"$TMP/panel-1.err" &
ppid=$!
pwait=0
while [ "$pwait" -lt 200 ]; do
  if jq -e --arg b "$pbranch" '.open.verify[$b] == [1]' "$pledger" >/dev/null 2>&1; then break; fi
  pwait=$((pwait + 1))
  sleep 0.05
done
set +e
(
  cd "$prepo" || exit 1
  FAKE_ARGS_LOG="$TMP/panel-2-args.txt" "$RUN" --role verify --author-vendor openai \
    --artifact worktree --claim "panel lens two" --receipt "$TMP/panel-2.json" --config "$cfg"
) >/dev/null 2>"$TMP/panel-2.err"
rc=$?
set -e
want_rc 0 "$rc" "the second panel lens approves first"
if jq -e --arg b "$pbranch" '.rounds.verify[$b] == 2' "$pledger" >/dev/null 2>&1; then ok; else bad "a lens must not clear the count while a sibling is still running"; fi
: > "$TMP/panel-gate"
set +e
wait "$ppid"
rc=$?
set -e
want_rc 0 "$rc" "the first panel lens approves second"
if jq -e --arg b "$pbranch" '.rounds.verify | has($b) | not' "$pledger" >/dev/null 2>&1; then ok; else bad "a panel whose every lens approved must clear the count"; fi
if jq -e --arg b "$pbranch" '(.open.verify[$b] // []) == []' "$pledger" >/dev/null 2>&1; then ok; else bad "a resolved panel must leave no open reservations"; fi

# A dispatch that never reaches a verdict still consumed a round, so it must
# close its own reservation or nothing on this branch could ever reset again.
set +e
(
  cd "$prepo" || exit 1
  FAKE_VERDICT=invalid "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a failed dispatch must not wedge the ledger" --receipt "$TMP/panel-fail.json" \
    --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 7 "$rc" "the failed dispatch still exits 7"
if jq -e --arg b "$pbranch" '(.open.verify[$b] // []) == []' "$pledger" >/dev/null 2>&1; then ok; else bad "a failed dispatch must close its own reservation"; fi
set +e
(
  cd "$prepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "an approve after a failed dispatch still resets" --receipt "$TMP/panel-after.json" \
    --config "$cfg"
) >/dev/null 2>/dev/null
rc=$?
set -e
want_rc 0 "$rc" "the round after a failed dispatch dispatches"
want_jq "$TMP/panel-after.json" '.round == 2' "a failed dispatch consumed a round"
if jq -e --arg b "$pbranch" '.rounds.verify | has($b) | not' "$pledger" >/dev/null 2>&1; then ok; else bad "an approve after a failed dispatch must still clear the count"; fi

echo "== approve must agree with its own findings =="
# The output contract lives in the prompt, and a prompt is a request. A model that
# returns approve alongside a critical finding got an approval receipt written for
# it, and the Stop hook trusts result.verdict on its own, so the gate opened on a
# tree the reviewer had just said was broken.
vrepo="$TMP/verdict-repo"
mkdir -p "$vrepo"
(
  cd "$vrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1

run_verdict() {
  local verdict="$1" receipt_out="$2" err_out="$3"
  set +e
  (
    cd "$vrepo" || exit 1
    FAKE_VERDICT="$verdict" "$RUN" --role verify --author-vendor openai \
      --artifact worktree --claim "the verdict must agree with the findings" \
      --receipt "$receipt_out" --config "$cfg"
  ) >/dev/null 2>"$err_out"
  local rc_out=$?
  set -e
  return "$rc_out"
}

rc=0; run_verdict approve-with-critical "$TMP/v-critical.json" "$TMP/v-critical.err" || rc=$?
want_rc 7 "$rc" "approve alongside a critical finding is rejected"
[ ! -e "$TMP/v-critical.json" ] && ok || bad "a self-contradicting approve must write no receipt"
if grep -qi 'critical or major' "$TMP/v-critical.err"; then ok; else bad "the rejection must name the contradiction"; fi
if grep -q 'billing.go' "$TMP/v-critical.err"; then ok; else bad "the rejection must show the finding it refused to approve"; fi

rc=0; run_verdict approve-with-major "$TMP/v-major.json" "$TMP/v-major.err" || rc=$?
want_rc 7 "$rc" "approve alongside a major finding is rejected"
[ ! -e "$TMP/v-major.json" ] && ok || bad "approve with a major finding must write no receipt"

# A minor finding is compatible with an approve, or every reviewer with a nit
# would have to withhold one.
rc=0; run_verdict approve-with-minor "$TMP/v-minor.json" "$TMP/v-minor.err" || rc=$?
want_rc 0 "$rc" "approve alongside a minor finding is accepted"
want_jq "$TMP/v-minor.json" '.result.verdict == "approve" and (.result.findings | length == 1)' "an approve with a minor finding still writes a receipt"

# needs_attention with a critical finding is the ordinary case and must be
# untouched.
rc=0; run_verdict needs_attention "$TMP/v-needs.json" "$TMP/v-needs.err" || rc=$?
want_rc 5 "$rc" "needs_attention is unaffected"

# The launcher is where the invariant becomes provenance, but the schema has to
# carry it too, for anything that validates a verdict object outside this script.
if jq -e '
  [.allOf[] | select(.if.properties.verdict.const == "approve")]
  | length == 1
  and (.[0].then.properties.findings.items.properties.severity.const == "minor")
' "$HERE/../../schemas/review-verdict-v1.json" >/dev/null 2>&1; then ok; else bad "review-verdict-v1 must forbid approve alongside a non-minor finding"; fi

if python3 -c 'import jsonschema' >/dev/null 2>&1; then
  schema_check() {
    python3 - "$HERE/../../schemas/review-verdict-v1.json" "$1" <<'PY' >/dev/null 2>&1
import json, sys
import jsonschema
schema = json.load(open(sys.argv[1]))
jsonschema.validate(json.loads(sys.argv[2]), schema)
PY
  }
  finding='{"severity":"SEV","file":"a.go","lines":"1","confidence":0.9,"finding":"f","recommendation":"r"}'
  body='{"verdict":"VERDICT","findings":[FINDING],"next_steps":[],"evidence":{"commands":[],"screenshots":[]}}'
  make_doc() { printf '%s' "$body" | sed -e "s/VERDICT/$1/" -e "s|FINDING|$(printf '%s' "$finding" | sed "s/SEV/$2/")|"; }
  if schema_check "$(make_doc approve critical)"; then bad "the schema must reject approve alongside a critical finding"; else ok; fi
  if schema_check "$(make_doc approve major)"; then bad "the schema must reject approve alongside a major finding"; else ok; fi
  if schema_check "$(make_doc approve minor)"; then ok; else bad "the schema must accept approve alongside a minor finding"; fi
  if schema_check "$(make_doc needs_attention critical)"; then ok; else bad "the schema must accept needs_attention alongside a critical finding"; fi
else
  echo "  NOTE: python3 jsonschema is absent, so the schema was checked structurally only"
fi

# Both provider adapters must strip the conditional before sending it, because
# strict structured-output modes reject if/then and would fail every dispatch.
if jq -e 'has("allOf") | not' "$FAKE_SCHEMA_LOG" >/dev/null 2>&1; then ok; else bad "the Claude route must not send the if/then conditional"; fi

echo "== the id binds the bytes the reviewer was shown =="
# A filename may legally end in a newline, and `v="$(cat file)"` strips exactly
# that. With `late.go` and `late.go` plus a newline both untracked, the stripped
# read collapses the second name onto the first: the package prints one heading
# twice and shows the FIRST file's bytes under both, while the subject id still
# binds the second file's content. Approving that package authorizes bytes nobody
# read, which is the failure the whole receipt exists to prevent.
nlrepo="$TMP/newline-name-repo"
mkdir -p "$nlrepo"
(
  cd "$nlrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
  printf 'func billingOne() {}\n' > 'late.go'
  printf 'func billingTwo() {}\n' > $'late.go\n'
) >/dev/null 2>&1
nldir="$TMP/newline-transcripts"
set +e
(
  cd "$nlrepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a filename ending in a newline must not be collapsed onto its neighbour" \
    --receipt "$TMP/newline.json" --config "$cfg" --transcript-dir "$nldir"
) >/dev/null 2>"$TMP/newline.err"
rc=$?
set -e
want_rc 0 "$rc" "an untracked name ending in a newline still dispatches"
nlsub="$(find "$nldir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
if grep -q 'func billingOne' "$nlsub/review-package.txt" 2>/dev/null; then ok; else bad "the package must show the ordinary untracked file"; fi
if grep -q 'func billingTwo' "$nlsub/review-package.txt" 2>/dev/null; then ok; else bad "the package must show the file whose name ends in a newline"; fi
if [ "$(grep -c 'untracked, blob' "$nlsub/review-package.txt" 2>/dev/null)" = 2 ]; then ok; else bad "the package must carry one entry per untracked file, not one name twice"; fi
if jq -e --arg id "$(cd "$nlrepo" && "$DIFF_ID")" '.subject.id == $id' "$TMP/newline.json" >/dev/null 2>&1; then ok; else bad "the receipt must carry the id the Stop hook will compute"; fi
# And the id has to move when that file's content moves, or binding it is only a
# claim. The Stop hook would otherwise honor the receipt across an edit nobody saw.
nl_id_before="$(cd "$nlrepo" && "$DIFF_ID")"
printf 'func billingThree() {}\n' > "$nlrepo/"$'late.go\n'
if [ "$nl_id_before" != "$(cd "$nlrepo" && "$DIFF_ID")" ]; then ok; else bad "editing the newline-named file must stale the receipt"; fi

# THE BYTES SHOWN AND THE BYTES BOUND ARE THE SAME BYTES. `git hash-object` runs
# the clean filter chosen by the path, so under `text`/`eol` or a configured
# `filter.*.clean` the worktree bytes and the hashed bytes differ. Showing the raw
# copy while binding the filtered blob means the reviewer reads content the id does
# not name, and a later swap the filter normalizes away leaves the id unmoved.
crrepo="$TMP/clean-filter-repo"
mkdir -p "$crrepo"
(
  cd "$crrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf '*.txt text eol=lf\n' > .gitattributes
  git add .gitattributes
  git commit -qm init
  printf 'pending\n' > svc.md
  printf 'func charge() {}\r\nfunc refund() {}\r\n' > payment.txt
) >/dev/null 2>&1
# The test has teeth only if the filter actually rewrites something.
raw_hash="$(git -C "$crrepo" hash-object --no-filters -- "$crrepo/payment.txt")"
clean_hash="$(git -C "$crrepo" hash-object --path payment.txt -- "$crrepo/payment.txt")"
if [ "$raw_hash" != "$clean_hash" ]; then ok; else bad "test setup: the clean filter must rewrite this file"; fi
crdir="$TMP/clean-filter-transcripts"
set +e
(
  cd "$crrepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the reviewer must read the bytes the subject id names" \
    --receipt "$TMP/clean.json" --config "$cfg" --transcript-dir "$crdir"
) >/dev/null 2>"$TMP/clean.err"
rc=$?
set -e
want_rc 0 "$rc" "a filtered untracked file still dispatches"
crsub="$(find "$crdir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1)"
# Asserted directly rather than inferred: reconstruct the file from the package the
# reviewer was handed, then hash what comes out. `git apply` ignores the headings
# around the patch. `--no-filters`, because the reconstruction is already the
# post-filter form and running the filter again would be a second normalization.
crapply="$TMP/clean-apply"
mkdir -p "$crapply"
if (cd "$crapply" && git apply --binary "$crsub/review-package.txt") 2>/dev/null; then ok; else bad "the package must contain an applicable patch for the untracked file"; fi
shown_hash="$(git -C "$crrepo" hash-object --no-filters -- "$crapply/payment.txt" 2>/dev/null || echo missing)"
if [ "$shown_hash" = "$clean_hash" ]; then ok; else bad "the bytes shown to the reviewer must hash to the blob the subject id binds: shown=$shown_hash bound=$clean_hash"; fi
if [ "$shown_hash" != "$raw_hash" ]; then ok; else bad "the package still shows the pre-filter bytes"; fi
# The heading names that same blob, so a reader can check the binding by hand.
if grep -q "payment.txt (untracked, blob $clean_hash)" "$crsub/review-package.txt" 2>/dev/null; then ok; else bad "the package heading must name the blob the id binds"; fi

echo "== the receipt binds a base, and a repository with no commits binds the empty tree =="
# The fingerprint covers the pending delta only, so without a base the receipt
# names a shape rather than a tree. A repository with no commits has no HEAD, and
# the empty tree is what git itself diffs the index against there.
norepo="$TMP/no-commit-repo"
mkdir -p "$norepo"
(
  cd "$norepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func billing() {}\n' > billing.go
  git add billing.go
) >/dev/null 2>&1
set +e
(
  cd "$norepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a repository with no commits still binds a base" \
    --receipt "$TMP/nocommit.json" --config "$cfg"
) >/dev/null 2>"$TMP/nocommit.err"
rc=$?
set -e
want_rc 0 "$rc" "a repository with no commits still produces a receipt"
if jq -e --arg empty "$(git -C "$norepo" hash-object -t tree /dev/null)" '.subject.base == $empty' "$TMP/nocommit.json" >/dev/null 2>&1; then ok; else bad "a repository with no commits must bind the empty tree as its base"; fi

# The capture is a sequence of reads, so a commit landing partway through leaves
# the earlier and later sections describing different trees under one recorded
# base. Refuse rather than bind a base that only half the package applies to.
mvrepo="$TMP/moving-base-repo"
mkdir -p "$mvrepo"
(
  cd "$mvrepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'base\n' > svc.txt
  git add svc.txt
  git commit -qm init
  printf 'pending\n' > svc.txt
) >/dev/null 2>&1
mv_shim="$TMP/moving-base-shim"
mkdir -p "$mv_shim/state"
mv_git="$(command -v git)"
# Commit once, on the first `status`, which the launcher runs after it has already
# read the base and before it reads the diffs.
cat > "$mv_shim/git" <<EOF
#!/usr/bin/env bash
args=" \$* "
case "\$args" in
  *" status "*)
    if [ ! -e "$mv_shim/state/moved" ]; then
      : > "$mv_shim/state/moved"
      "$mv_git" -C "$mvrepo" commit -q --allow-empty -m "landed mid capture"
    fi
    ;;
esac
exec "$mv_git" "\$@"
EOF
chmod +x "$mv_shim/git"
set +e
(
  cd "$mvrepo" || exit 1
  PATH="$mv_shim:$PATH" "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "a base that moves mid capture must not be bound" \
    --receipt "$TMP/moving.json" --config "$cfg"
) >/dev/null 2>"$TMP/moving.err"
rc=$?
set -e
[ -e "$mv_shim/state/moved" ] && ok || bad "test setup: the shim never moved HEAD"
want_rc 9 "$rc" "a base that moves during the capture is refused"
[ ! -e "$TMP/moving.json" ] && ok || bad "a moving base must write no receipt"
if grep -q 'HEAD moved' "$TMP/moving.err"; then ok; else bad "the refusal must say the base moved: $(head -2 "$TMP/moving.err" | tr '\n' ' ')"; fi

# A file artifact has no base: its id hashes the whole artifact, so there is
# nothing for a base to add and a required-but-empty field would be a lie.
if jq -e '.subject | has("base") | not' "$TMP/race-file.json" >/dev/null 2>&1; then ok; else bad "a file artifact must not carry a base"; fi

# The schema has to carry the requirement too, for anything validating a receipt
# outside this launcher.
if python3 -c 'import jsonschema' >/dev/null 2>&1; then
  receipt_schema_check() {
    python3 - "$HERE/../../schemas/review-receipt-v2.json" "$1" <<'PY' >/dev/null 2>&1
import json, sys
import jsonschema, referencing, referencing.jsonschema
from pathlib import Path
path = Path(sys.argv[1])
schema = json.loads(path.read_text())
registry = referencing.Registry()
for sibling in path.parent.glob("*.json"):
    registry = registry.with_resource(
        sibling.name,
        referencing.Resource.from_contents(json.loads(sibling.read_text()),
                                           default_specification=referencing.jsonschema.DRAFT202012),
    )
jsonschema.Draft202012Validator(schema, registry=registry).validate(json.loads(sys.argv[2]))
PY
  }
  if receipt_schema_check "$(cat "$TMP/receipt.json")" ; then ok; else bad "the v2 schema must accept a receipt the launcher wrote"; fi
  if receipt_schema_check "$(jq -c 'del(.subject.base)' "$TMP/receipt.json")"; then bad "the v2 schema must reject a worktree receipt with no base"; else ok; fi
  if receipt_schema_check "$(cat "$TMP/race-file.json")"; then ok; else bad "the v2 schema must accept a file receipt with no base"; fi
  if receipt_schema_check "$(jq -c '.subject.base = "deadbeef"' "$TMP/race-file.json")"; then bad "the v2 schema must reject a base on a file artifact"; else ok; fi
else
  echo "  NOTE: python3 jsonschema is absent, so review-receipt-v2 was checked structurally only"
  if jq -e '
    [.properties.subject.allOf[] | select(.if.properties.kind.const == "worktree-diff")]
    | length == 1 and (.[0].then.required == ["base"])
  ' "$HERE/../../schemas/review-receipt-v2.json" >/dev/null 2>&1; then ok; else bad "review-receipt-v2 must require base for a worktree-diff subject"; fi
fi

echo "== the package and the receipt cover submodule content =="
# A gitlink diffs as a bare `Subproject commit <sha>` line, so a reviewer handed
# the ordinary package reads SHAs and never sees the code the pointer pulled in.
# Untracked content inside a submodule reaches the superproject diff nowhere at
# all. Both have to be in the package, and both have to be bound.
subsrc="$TMP/sub-src"
mkdir -p "$subsrc"
(
  cd "$subsrc" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func helper() {}\n' > lib.go
  git add lib.go
  git commit -qm clean
  printf 'func chargeCard() { stripe(secret) }\n' > lib.go
  git add lib.go
  git commit -qm risky
) >/dev/null 2>&1
sub_clean="$(git -C "$subsrc" rev-parse HEAD~1)"
sub_risky="$(git -C "$subsrc" rev-parse HEAD)"
suprepo="$TMP/superproject"
mkdir -p "$suprepo"
(
  cd "$suprepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  # NO `diff.ignoreSubmodules` here. Under the DEFAULT configuration git omits an
  # untracked-only dirty submodule from every diff, and that is the case the
  # submodule section exists for. Setting the variable would make the assertions
  # below pass against a stricter git than anyone actually runs.
  git -c protocol.file.allow=always submodule add -q "$subsrc" vendor/lib
  (cd vendor/lib && git checkout -q "$sub_clean")
  git add -A
  git commit -qm init
  git -C vendor/lib checkout -q "$sub_risky"
  printf 'func billing() { invoice() }\n' > vendor/lib/dropped.go
) >/dev/null 2>&1
sub_pkg_dir="$TMP/sub-transcript"
set +e
(
  cd "$suprepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the submodule bump is safe" --receipt "$TMP/sub-receipt.json" \
    --config "$cfg" --transcript-dir "$sub_pkg_dir"
) >/dev/null 2>"$TMP/sub.err"
rc=$?
set -e
want_rc 0 "$rc" "a submodule bump dispatches"
sub_pkg="$(find "$sub_pkg_dir" -name 'review-package.txt' | head -1)"
if [ -n "$sub_pkg" ]; then ok; else bad "the dispatch must retain a review package"; fi
# The premise: the ordinary sections show a pointer and nothing else.
if [ -n "$sub_pkg" ] && ! sed -n '/^# Submodules$/q;p' "$sub_pkg" | grep -q 'stripe(secret)'; then
  ok
else
  bad "test premise: the pointer sections already showed the submodule content"
fi
if [ -n "$sub_pkg" ] && grep -q 'stripe(secret)' "$sub_pkg"; then ok; else bad "the package must show the submodule content behind the pointer"; fi
if [ -n "$sub_pkg" ] && grep -q 'func billing() { invoice() }' "$sub_pkg"; then ok; else bad "the package must show a file untracked inside the submodule"; fi
if jq -e --arg sub "$(cd "$suprepo" && "$DIFF_ID" --submodules)" \
  '.subject.submodules == $sub' "$TMP/sub-receipt.json" >/dev/null 2>&1; then
  ok
else
  bad "the receipt must bind the submodule fingerprint the Stop hook recomputes"
fi
# Supplemental means alongside: a repository with no gitlink writes no such field,
# so a receipt for an ordinary tree is byte-compatible with one written before it.
if jq -e '.subject | has("submodules") | not' "$receipt" >/dev/null 2>&1; then ok; else bad "a tree with no gitlink must not carry a submodule fingerprint"; fi
# ...and the subject id is untouched by any of it.
if jq -e --arg id "$(cd "$suprepo" && "$DIFF_ID")" '.subject.id == $id' "$TMP/sub-receipt.json" >/dev/null 2>&1; then
  ok
else
  bad "the submodule snapshot must not move the subject id"
fi
# The schema has to accept the new field too, for anything validating a receipt
# outside this launcher. `receipt_schema_check` is defined only when python3 has
# jsonschema; the structural fallback above already pins the base rule, and the
# field's own shape is pinned here when the real validator is available.
if command -v receipt_schema_check >/dev/null 2>&1 || declare -F receipt_schema_check >/dev/null 2>&1; then
  if receipt_schema_check "$(cat "$TMP/sub-receipt.json")"; then ok; else bad "the v2 schema must accept a receipt carrying a submodule fingerprint"; fi
  if receipt_schema_check "$(jq -c '.subject.submodules = ""' "$TMP/sub-receipt.json")"; then bad "the v2 schema must reject an empty submodule fingerprint"; else ok; fi
else
  if jq -e '.properties.subject.properties.submodules.type == "string"' \
    "$HERE/../../schemas/review-receipt-v2.json" >/dev/null 2>&1; then ok; else bad "review-receipt-v2 must describe subject.submodules"; fi
fi

echo "== a tree whose index hides a modification is refused before dispatch =="
# `assume-unchanged` omits the path from every diff, so the package, the risky
# scan and the fingerprint all describe the tree as it was. Dispatching a review
# of that is worse than refusing: it spends a round and returns an approve for
# content nobody was shown.
aurepo="$TMP/assume-unchanged-repo"
mkdir -p "$aurepo"
(
  cd "$aurepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm init
  printf 'pending\n' > note.txt
  git update-index --assume-unchanged svc.go
  printf 'func chargeCard() { stripe(secret) }\n' > svc.go
) >/dev/null 2>&1
set +e
(
  cd "$aurepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "nothing to see" --receipt "$TMP/au-receipt.json" --config "$cfg"
) >/dev/null 2>"$TMP/au.err"
rc=$?
set -e
want_rc 9 "$rc" "a hidden index modification is refused"
[ ! -e "$TMP/au-receipt.json" ] && ok || bad "a refused capture must write no receipt"
if grep -q 'svc.go' "$TMP/au.err"; then ok; else bad "the refusal must name the path: $(head -3 "$TMP/au.err" | tr '\n' ' ')"; fi
# Refused BEFORE a round is reserved, so a dispatch that never happened does not
# make the next real review report round 2.
if [ ! -e "$aurepo/.git/megapowers-review-rounds.json" ]; then ok; else bad "a refused capture must not consume a round"; fi
(cd "$aurepo" && git update-index --no-assume-unchanged svc.go)
set +e
(
  cd "$aurepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "now visible" --receipt "$TMP/au-receipt.json" --config "$cfg"
) >/dev/null 2>>"$TMP/au.err"
rc=$?
set -e
want_rc 0 "$rc" "clearing the bit lets the same tree dispatch"

echo "== replacement objects do not move the captured base =="
# `git replace X Y` makes every object read return Y where X was asked for while
# `git rev-parse HEAD` still prints X, so a repository holding X plus a
# replacement reproduces an approved delta on a base the reviewer never saw. The
# launcher reads the real objects, so the delta it captures is the real one.
reprepo="$TMP/replace-repo"
mkdir -p "$reprepo"
(
  cd "$reprepo" || exit 1
  git init -q
  git config user.email test@example.com
  git config user.name test
  git config commit.gpgsign false
  printf 'func chargeCard() { stripe(secret) }\n' > svc.go
  git add svc.go
  git commit -qm hidden
  printf 'func handler() {}\n' > svc.go
  git add svc.go
  git commit -qm reviewed
  git replace "$(git rev-parse HEAD)" "$(git rev-parse HEAD~1)"
  printf 'func handler() { billing() }\n' > svc.go
) >/dev/null 2>&1
set +e
(
  cd "$reprepo" || exit 1
  "$RUN" --role verify --author-vendor openai --artifact worktree \
    --claim "the delta is what it looks like" --receipt "$TMP/rep-receipt.json" \
    --config "$cfg" --transcript-dir "$TMP/rep-transcript"
) >/dev/null 2>"$TMP/rep.err"
rc=$?
set -e
want_rc 0 "$rc" "a repository with a replace ref still dispatches"
rep_pkg="$(find "$TMP/rep-transcript" -name 'review-package.txt' | head -1)"
# The replaced base would render the pending change as a rewrite of the hidden
# tree. The real base renders it as a rewrite of the reviewed one, which is the
# only rendering that matches the recorded subject.base.
if [ -n "$rep_pkg" ] && grep -q '^-func handler() {}$' "$rep_pkg"; then ok; else bad "the package must diff against the real base, not the replacement"; fi
if [ -n "$rep_pkg" ] && ! grep -q '^-func chargeCard' "$rep_pkg"; then ok; else bad "the package must not diff against the replaced base"; fi
if jq -e --arg id "$(cd "$reprepo" && "$DIFF_ID")" '.subject.id == $id' "$TMP/rep-receipt.json" >/dev/null 2>&1; then
  ok
else
  bad "the launcher and the fingerprint must agree under a replace ref"
fi

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
