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
printf '%s\n' "$*" > "${FAKE_ARGS_LOG:?}"
printf '%s\n' "${CLAUDE_CONFIG_DIR:-}" > "${FAKE_CONFIG_LOG:?}"
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
want_jq "$receipt" '.schema == "megapowers.review-receipt.v1"' "receipt schema"
want_jq "$receipt" '.subject.kind == "worktree-diff"' "worktree subject"
if jq -e --arg id "$("$DIFF_ID")" '.subject.id == $id' "$receipt" >/dev/null 2>&1; then ok; else bad "receipt binds current diff"; fi
want_jq "$receipt" '.author_vendors == ["openai"]' "receipt binds author vendor"
want_jq "$receipt" '.reviewer.vendor == "anthropic" and .reviewer.model == "fake-frontier" and .reviewer.tier == "frontier" and .reviewer.effort == "high"' "launcher provenance"
want_jq "$receipt" '.independent == true and .result.verdict == "approve"' "independent approval"
want_jq "$FAKE_SCHEMA_LOG" 'has("$schema") | not' "Claude schema omits unsupported draft metadata"
[ ! -e "$repo/should-not-run" ] && ok || bad "claim shell metacharacters executed"

unset ANTHROPIC_API_KEY
mkdir -p "$TMP/oauth-home/.claude"
printf '{}\n' > "$TMP/oauth-home/.claude/.credentials.json"
set +e
HOME="$TMP/oauth-home" "$RUN" --role verify --author-vendor openai \
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

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
