#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TOOL="$ROOT/plugins/megapowers/skills/independent-review/scripts/megapowers-review.go"
GO_BIN="$(command -v go)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
last_out=""
last_rc=0

ok() {
  pass=$((pass + 1))
  printf '  ok %s\n' "$1"
}

bad() {
  fail=$((fail + 1))
  printf '  FAIL %s\n' "$1"
  [ -z "$last_out" ] || printf '%s\n' "$last_out" | sed -n '1,20p'
}

must_succeed() {
  local name="$1"
  shift
  run_tool "$@"
  if [ "$last_rc" -eq 0 ]; then ok "$name"; else bad "$name (exit $last_rc)"; fi
}

must_fail_with() {
  local name="$1" needle="$2"
  shift 2
  run_tool "$@"
  if [ "$last_rc" -ne 0 ] && printf '%s\n' "$last_out" | grep -qi -- "$needle"; then
    ok "$name"
  else
    bad "$name (exit $last_rc, missing '$needle')"
  fi
}

mode_of() {
  stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"
}

json_value() {
  local key="$1"
  sed -n "s/^[[:space:]]*\"$key\": \"\([^\"]*\)\",*$/\\1/p"
}

REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.name 'Review Test'
git -C "$REPO" config user.email 'review@example.invalid'
printf 'package example\n\nfunc Value() int { return 1 }\n' > "$REPO/app.go"
git -C "$REPO" add app.go
git -C "$REPO" -c commit.gpgsign=false commit -qm initial
BASE_OID="$(git -C "$REPO" rev-parse HEAD)"
printf 'package example\n\nfunc Value() int { return 2 }\n' > "$REPO/app.go"
git -C "$REPO" add app.go
git -C "$REPO" -c commit.gpgsign=false commit -qm changed
HEAD_OID="$(git -C "$REPO" rev-parse HEAD)"

EXTERNAL_BIN="$TMP/external-bin"
mkdir -p "$EXTERNAL_BIN"
for provider in claude codex; do
  {
    printf '#!/usr/bin/env bash\n'
    printf 'env | sort > "%s/provider.env"\n' "$TMP"
    printf 'printf "%%s\\n" "$*" > "%s/provider.args"\n' "$TMP"
    printf 'printf "%%s\\n" "$0" > "%s/provider.path"\n' "$TMP"
    printf 'cat > "%s/provider.stdin"\n' "$TMP"
    printf 'printf "reviewed by fake provider\\n"\n'
  } > "$EXTERNAL_BIN/$provider"
  chmod +x "$EXTERNAL_BIN/$provider"
done

ACTIVE_PATH="$EXTERNAL_BIN:$PATH"

run_tool() {
  last_out="$(
    cd "$REPO" || exit 97
    PATH="$ACTIVE_PATH" MEGAPOWERS_TEST_LEAK='do-not-forward' \
      ANTHROPIC_API_KEY='test-anthropic-credential' OPENAI_API_KEY='test-openai-credential' \
      "$GO_BIN" run "$TOOL" "$@" 2>&1
  )"
  last_rc=$?
}

printf '== megapowers-review black-box tests ==\n'

must_fail_with 'input mode is required' 'exactly one input mode' inspect
must_fail_with 'file and range are mutually exclusive' 'exactly one input mode' \
  inspect --file app.go --base "$BASE_OID" --head "$HEAD_OID" --provider claude
must_fail_with 'range requires both endpoints' 'base and --head' \
  inspect --base "$BASE_OID" --provider claude
must_fail_with 'inspect requires a fixed provider' 'provider must be' inspect --file app.go

printf 'AWS_SECRET_ACCESS_KEY=not-part-of-the-commit\n' > "$REPO/.env"
must_succeed 'commit range ignores untracked worktree files' \
  inspect --base HEAD~1 --head HEAD --provider claude
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q "$BASE_OID" && \
   printf '%s\n' "$last_out" | grep -q "$HEAD_OID" && \
   printf '%s\n' "$last_out" | grep -q 'app.go'; then
  ok 'commit range resolves to immutable OIDs and paths'
else
  bad 'commit range resolves to immutable OIDs and paths'
fi

must_fail_with 'secret-like path is rejected' 'secret-like path' \
  inspect --file .env --provider claude
printf 'api_key = "sk-abcdefghijklmnopqrstuvwxyz123456"\n' > "$REPO/config.txt"
must_fail_with 'likely secret content is rejected' 'likely secret content' \
  inspect --file config.txt --provider claude

ln -s app.go "$REPO/link.go"
must_fail_with 'explicit symlink is rejected' 'symlink' \
  inspect --file link.go --provider claude

printf 'text\000binary\n' > "$REPO/binary.dat"
must_fail_with 'binary input is rejected' 'binary' \
  inspect --file binary.dat --provider claude

dd if=/dev/zero bs=1100000 count=1 2>/dev/null | tr '\000' 'a' > "$REPO/large.txt"
must_fail_with 'oversized input is rejected' 'size limit' \
  inspect --file large.txt --provider claude

ln -s app.go "$REPO/committed-link.go"
git -C "$REPO" add committed-link.go
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add symlink'
LINK_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_fail_with 'range symlink is rejected' 'symlink' \
  inspect --base "$HEAD_OID" --head "$LINK_HEAD" --provider claude

git -C "$REPO" rm -q committed-link.go
git -C "$REPO" -c commit.gpgsign=false commit -qm 'remove symlink'
SUB_BASE="$(git -C "$REPO" rev-parse HEAD)"
mkdir -p "$REPO/vendor"
git -C "$REPO" update-index --add --cacheinfo "160000,$HEAD_OID,vendor/module"
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add gitlink'
SUB_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_fail_with 'submodule range is rejected' 'submodule' \
  inspect --base "$SUB_BASE" --head "$SUB_HEAD" --provider claude

EMPTY_BASE="$SUB_HEAD"
: > "$REPO/empty.txt"
git -C "$REPO" add empty.txt
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add empty file'
EMPTY_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_succeed 'empty file range can be inspected' \
  inspect --base "$EMPTY_BASE" --head "$EMPTY_HEAD" --provider claude
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q 'empty.txt' && \
   printf '%s\n' "$last_out" | grep -q 'e3b0c44298fc1c149afbf4c8996fb924'; then
  ok 'empty files retain a source hash'
else
  bad 'empty files retain a source hash'
fi

must_fail_with 'author cannot review own artifact' 'must differ' \
  review --file app.go --provider claude --author claude --approve-external invalid-token
must_fail_with 'external disclosure needs explicit approval' 'approve-external' \
  review --file app.go --provider claude --author codex

LOCAL_BIN="$REPO/local-bin"
mkdir -p "$LOCAL_BIN"
printf '#!/usr/bin/env bash\nprintf "must not run\\n"\n' > "$LOCAL_BIN/claude"
chmod +x "$LOCAL_BIN/claude"
ACTIVE_PATH="$LOCAL_BIN:$EXTERNAL_BIN:$PATH"
must_fail_with 'repository-local provider binary is rejected' 'inside the repository' \
  inspect --file app.go --provider claude
ACTIVE_PATH="$EXTERNAL_BIN:$LOCAL_BIN:$PATH"

must_succeed 'inspect resolves provider and creates approval token' \
  inspect --file app.go --provider claude
APPROVAL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
PROVIDER_PATH="$(cd "$EXTERNAL_BIN" && pwd -P)/claude"
if [ -n "$APPROVAL_TOKEN" ] && printf '%s\n' "$last_out" | grep -q '"package_sha256"' && \
   printf '%s\n' "$last_out" | grep -q '"binary_sha256"' && \
   printf '%s\n' "$last_out" | grep -Fq "$PROVIDER_PATH"; then
  ok 'inspect discloses resolved binary path, binary hash, package hash, and token'
else
  bad 'inspect discloses resolved binary path, binary hash, package hash, and token'
fi

cp "$REPO/app.go" "$TMP/app.go.original"
printf '\n// changed after approval\n' >> "$REPO/app.go"
rm -f "$TMP/provider.env" "$TMP/provider.stdin" "$TMP/provider.args"
must_fail_with 'file substitution invalidates approval token' 'approval token does not match' \
  review --file app.go --provider claude --author codex --approve-external "$APPROVAL_TOKEN"
if [ ! -e "$TMP/provider.env" ] && [ ! -e "$TMP/provider.stdin" ]; then
  ok 'file substitution is rejected before credentials or input reach provider'
else
  bad 'file substitution is rejected before credentials or input reach provider'
fi
cp "$TMP/app.go.original" "$REPO/app.go"

must_succeed 'inspect original provider before binary substitution' \
  inspect --file app.go --provider claude
BINARY_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
cp "$EXTERNAL_BIN/claude" "$TMP/claude.original"
{
  printf '#!/usr/bin/env bash\n'
  printf 'printf executed > "%s/swapped-provider-ran"\n' "$TMP"
  printf 'env > "%s/provider.env"\n' "$TMP"
  printf 'cat > "%s/provider.stdin"\n' "$TMP"
} > "$EXTERNAL_BIN/claude"
chmod +x "$EXTERNAL_BIN/claude"
rm -f "$TMP/provider.env" "$TMP/provider.stdin" "$TMP/swapped-provider-ran"
must_fail_with 'PATH binary substitution invalidates approval token' 'approval token does not match' \
  review --file app.go --provider claude --author codex --approve-external "$BINARY_TOKEN"
if [ ! -e "$TMP/swapped-provider-ran" ] && [ ! -e "$TMP/provider.env" ] && \
   [ ! -e "$TMP/provider.stdin" ]; then
  ok 'binary substitution is rejected before credentials or input reach provider'
else
  bad 'binary substitution is rejected before credentials or input reach provider'
fi
cp "$TMP/claude.original" "$EXTERNAL_BIN/claude"
chmod +x "$EXTERNAL_BIN/claude"

must_succeed 'fresh inspection creates happy-path token' \
  inspect --file app.go --provider claude
APPROVAL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"

must_fail_with 'explicit receipt output must already exist' 'already exist' \
  review --file app.go --provider claude --author codex \
  --approve-external "$APPROVAL_TOKEN" --out "$TMP/missing-review-out"

repo_mode_before="$(mode_of "$REPO")"
must_fail_with 'explicit receipt output cannot overlap the repository' 'outside the repository' \
  review --file app.go --provider claude --author codex \
  --approve-external "$APPROVAL_TOKEN" --retain-transcript --out "$REPO"
repo_mode_after="$(mode_of "$REPO")"
chmod "$repo_mode_before" "$REPO"
if [ "$repo_mode_after" = "$repo_mode_before" ]; then
  ok 'rejected receipt output leaves repository permissions unchanged'
else
  bad 'rejected receipt output leaves repository permissions unchanged'
fi

OUT="$TMP/review-out"
mkdir -m 700 "$OUT"
must_succeed 'approved different-provider review succeeds' \
  review --file app.go --provider claude --author codex \
  --approve-external "$APPROVAL_TOKEN" --out "$OUT"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q 'provider=claude' && \
   printf '%s\n' "$last_out" | grep -q 'files=1' && \
   printf '%s\n' "$last_out" | grep -q 'app.go' && \
   printf '%s\n' "$last_out" | grep -q 'reviewed by fake provider'; then
  ok 'pre-dispatch disclosure and provider output are visible'
else
  bad 'pre-dispatch disclosure and provider output are visible'
fi

if [ -s "$TMP/provider.path" ] && [ "$(cat "$TMP/provider.path")" != "$PROVIDER_PATH" ]; then
  ok 'provider executes from a verified private copy'
else
  bad 'provider executes from a verified private copy'
fi

receipt="$(find "$OUT" -type f -name receipt.json -print -quit 2>/dev/null)"
if [ -n "$receipt" ] && [ "$(mode_of "$OUT")" = 700 ] && \
   [ "$(mode_of "$(dirname "$receipt")")" = 700 ] && \
   [ "$(mode_of "$receipt")" = 600 ]; then
  ok 'receipt directory and file are private'
else
  bad 'receipt directory and file are private'
fi

if [ -n "$receipt" ] && grep -q 'megapowers.advisory-review-receipt.v1' "$receipt" && \
   grep -q '"provider": "claude"' "$receipt" && \
   grep -q '"author": "codex"' "$receipt" && \
   grep -q '"advisory": true' "$receipt" && \
   grep -q '"sha256"' "$receipt"; then
  ok 'receipt is schema-versioned, advisory, independent, and source-bound'
else
  bad 'receipt is schema-versioned, advisory, independent, and source-bound'
fi

file_count="$(find "$OUT" -type f | wc -l | tr -d ' ')"
if [ "$file_count" = 1 ]; then ok 'transcript is not retained by default'; else bad 'transcript is not retained by default'; fi

if grep -q '^MEGAPOWERS_TEST_LEAK=' "$TMP/provider.env"; then
  bad 'provider environment excludes ambient variables'
elif grep -q -- "$LOCAL_BIN" "$TMP/provider.env"; then
  bad 'provider PATH excludes repository-local entries'
elif grep -q '^HOME=' "$TMP/provider.env" && grep -q '^PATH=' "$TMP/provider.env"; then
  ok 'provider environment uses an explicit allowlist with sanitized PATH'
else
  bad 'provider environment preserves required allowlisted variables'
fi

TRANSCRIPT_OUT="$TMP/transcript-out"
mkdir -m 700 "$TRANSCRIPT_OUT"
must_succeed 'transcript retention is explicit opt-in' \
  review --file app.go --provider claude --author codex \
  --approve-external "$APPROVAL_TOKEN" \
  --retain-transcript --out "$TRANSCRIPT_OUT"
transcript_receipt="$(find "$TRANSCRIPT_OUT" -type f -name receipt.json -print -quit 2>/dev/null)"
transcript_dir="$(dirname "$transcript_receipt")"
if [ -n "$transcript_receipt" ] && [ -f "$transcript_dir/prompt.txt" ] && \
   [ -f "$transcript_dir/provider.stdout" ] && [ -f "$transcript_dir/provider.stderr" ] && \
   [ "$(mode_of "$transcript_dir/prompt.txt")" = 600 ] && \
   grep -q '"transcript_retained": true' "$transcript_receipt" && \
   grep -q '"prompt_sha256"' "$transcript_receipt"; then
  ok 'opt-in transcript is private and hash-bound in receipt'
else
  bad 'opt-in transcript is private and hash-bound in receipt'
fi

printf '#!/usr/bin/env bash\ncat >/dev/null\nexit 23\n' > "$EXTERNAL_BIN/codex"
chmod +x "$EXTERNAL_BIN/codex"
must_succeed 'inspect failing provider binary for a bound token' \
  inspect --file app.go --provider codex
CODEX_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
FAIL_OUT="$TMP/failure-out"
mkdir -m 700 "$FAIL_OUT"
must_fail_with 'nonzero provider exit fails review' 'provider exited' \
  review --file app.go --provider codex --author claude \
  --approve-external "$CODEX_TOKEN" --out "$FAIL_OUT"
if [ ! -e "$FAIL_OUT" ] || [ -z "$(find "$FAIL_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'failed provider writes no receipt'
else
  bad 'failed provider writes no receipt'
fi

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
