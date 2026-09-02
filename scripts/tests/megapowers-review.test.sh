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

# Every case runs an operator-supplied command whose argv[0] is a fake
# reviewer binary on PATH. The fake records its environment, argv (one per
# line), stdin, and how it received the prompt.
EXTERNAL_BIN="$TMP/external-bin"
mkdir -p "$EXTERNAL_BIN"
install_fake_provider() {
  local name="$1"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'env | sort > "%s/provider.env"\n' "$TMP"
    printf 'printf "%%s\\n" "$@" > "%s/provider.args"\n' "$TMP"
    printf 'printf "%%s\\n" "$*" >> "%s/provider.calls"\n' "$TMP"
    printf 'printf "%%s\\n" "$0" > "%s/provider.path"\n' "$TMP"
    printf 'cat > "%s/provider.stdin"\n' "$TMP"
    printf 'delivery=stdin\n'
    printf '[ -s "%s/provider.stdin" ] || delivery=none\n' "$TMP"
    printf 'while [ "$#" -gt 0 ]; do\n'
    printf '  case "$1" in\n'
    printf '    --prompt-file) cat "$2" > "%s/provider.stdin"; stat -c %%a "$2" > "%s/provider.prompt-mode"; dirname "$2" > "%s/provider.prompt-dir"; delivery=file; shift ;;\n' "$TMP" "$TMP" "$TMP"
    printf '    --scratch) printf "%%s\\n" "$2" > "%s/provider.scratch"; shift ;;\n' "$TMP"
    printf '  esac\n'
    printf '  shift\n'
    printf 'done\n'
    printf 'printf "%%s\\n" "$delivery" > "%s/provider.delivery"\n' "$TMP"
    printf 'printf "reviewed by fake provider via %%s\\n" "$delivery"\n'
  } > "$EXTERNAL_BIN/$name"
  chmod +x "$EXTERNAL_BIN/$name"
}
install_fake_provider fake-reviewer

ACTIVE_PATH="$EXTERNAL_BIN:$PATH"
CMD='fake-reviewer --mode review'
FAKE_HOME_VALUE="$TMP/fake-home"
mkdir -p "$FAKE_HOME_VALUE"

run_tool() {
  last_out="$(
    cd "$REPO" || exit 97
    PATH="$ACTIVE_PATH" MEGAPOWERS_TEST_LEAK='do-not-forward' \
      FAKE_API_KEY='test-credential' FAKE_HOME="$FAKE_HOME_VALUE" \
      FAKE_CONFIG_DIR="$FAKE_HOME_VALUE/config" \
      "$GO_BIN" run "$TOOL" "$@" 2>&1
  )"
  last_rc=$?
}

printf '== megapowers-review black-box tests ==\n'

# --- option validation ------------------------------------------------------------

must_fail_with 'input mode is required' 'exactly one input mode' \
  inspect --provider vendor-a --provider-command "$CMD"
must_fail_with 'file and range are mutually exclusive' 'exactly one input mode' \
  inspect --file app.go --base "$BASE_OID" --head "$HEAD_OID" --provider vendor-a --provider-command "$CMD"
must_fail_with 'range requires both endpoints' 'base and --head' \
  inspect --base "$BASE_OID" --provider vendor-a --provider-command "$CMD"
must_fail_with 'inspect requires a provider label' '--provider' \
  inspect --file app.go --provider-command "$CMD"
must_fail_with 'inspect requires a provider command' '--provider-command' \
  inspect --file app.go --provider vendor-a
must_fail_with 'review requires a provider command' '--provider-command' \
  review --file app.go --provider vendor-a --author vendor-b --approve-external invalid-token
must_fail_with 'review requires an author label' '--author' \
  review --file app.go --provider vendor-a --provider-command "$CMD" --approve-external invalid-token
must_fail_with 'author label cannot equal provider label' 'must differ' \
  review --file app.go --provider vendor-a --author vendor-a --provider-command "$CMD" --approve-external invalid-token
must_fail_with 'external disclosure needs explicit approval' 'approve-external' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD"

# --- command template parsing -------------------------------------------------------

for template in 'fake-reviewer | cat' 'fake-reviewer; id' 'fake-reviewer & true' \
  'fake-reviewer > out' 'fake-reviewer < in' 'fake-reviewer $(id)' 'fake-reviewer `id`'; do
  must_fail_with "shell metacharacter is rejected: $template" 'shell' \
    inspect --file app.go --provider vendor-a --provider-command "$template"
done
must_fail_with 'unterminated quote is rejected' 'quote' \
  inspect --file app.go --provider vendor-a --provider-command "fake-reviewer 'open"
must_fail_with 'empty command template is rejected' 'provider-command' \
  inspect --file app.go --provider vendor-a --provider-command '   '
must_fail_with 'missing provider binary is reported' 'not installed' \
  inspect --file app.go --provider vendor-a --provider-command 'no-such-reviewer-binary --x'

printf 'AWS_SECRET_ACCESS_KEY=not-part-of-the-commit\n' > "$REPO/.env"
must_succeed 'commit range ignores untracked worktree files' \
  inspect --base HEAD~1 --head HEAD --provider vendor-a --provider-command "$CMD"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q "$BASE_OID" && \
   printf '%s\n' "$last_out" | grep -q "$HEAD_OID" && \
   printf '%s\n' "$last_out" | grep -q 'app.go'; then
  ok 'commit range resolves to immutable OIDs and paths'
else
  bad 'commit range resolves to immutable OIDs and paths'
fi

must_fail_with 'secret-like path is rejected' 'secret-like path' \
  inspect --file .env --provider vendor-a --provider-command "$CMD"
printf 'api_key = "sk-abcdefghijklmnopqrstuvwxyz123456"\n' > "$REPO/config.txt"
must_fail_with 'likely secret content is rejected' 'likely secret content' \
  inspect --file config.txt --provider vendor-a --provider-command "$CMD"

ln -s app.go "$REPO/link.go"
must_fail_with 'explicit symlink is rejected' 'symlink' \
  inspect --file link.go --provider vendor-a --provider-command "$CMD"

printf 'text\000binary\n' > "$REPO/binary.dat"
must_fail_with 'binary input is rejected' 'binary' \
  inspect --file binary.dat --provider vendor-a --provider-command "$CMD"

dd if=/dev/zero bs=1100000 count=1 2>/dev/null | tr '\000' 'a' > "$REPO/large.txt"
must_fail_with 'oversized input is rejected' 'size limit' \
  inspect --file large.txt --provider vendor-a --provider-command "$CMD"

ln -s app.go "$REPO/committed-link.go"
git -C "$REPO" add committed-link.go
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add symlink'
LINK_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_fail_with 'range symlink is rejected' 'symlink' \
  inspect --base "$HEAD_OID" --head "$LINK_HEAD" --provider vendor-a --provider-command "$CMD"

git -C "$REPO" rm -q committed-link.go
git -C "$REPO" -c commit.gpgsign=false commit -qm 'remove symlink'
SUB_BASE="$(git -C "$REPO" rev-parse HEAD)"
mkdir -p "$REPO/vendor"
git -C "$REPO" update-index --add --cacheinfo "160000,$HEAD_OID,vendor/module"
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add gitlink'
SUB_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_fail_with 'submodule range is rejected' 'submodule' \
  inspect --base "$SUB_BASE" --head "$SUB_HEAD" --provider vendor-a --provider-command "$CMD"

EMPTY_BASE="$SUB_HEAD"
: > "$REPO/empty.txt"
git -C "$REPO" add empty.txt
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add empty file'
EMPTY_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_succeed 'empty file range can be inspected' \
  inspect --base "$EMPTY_BASE" --head "$EMPTY_HEAD" --provider vendor-a --provider-command "$CMD"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q 'empty.txt' && \
   printf '%s\n' "$last_out" | grep -q 'e3b0c44298fc1c149afbf4c8996fb924'; then
  ok 'empty files retain a source hash'
else
  bad 'empty files retain a source hash'
fi

# --- argv[0] resolution -------------------------------------------------------------

LOCAL_BIN="$REPO/local-bin"
mkdir -p "$LOCAL_BIN"
printf '#!/usr/bin/env bash\nprintf "must not run\\n"\n' > "$LOCAL_BIN/fake-reviewer"
chmod +x "$LOCAL_BIN/fake-reviewer"
ACTIVE_PATH="$LOCAL_BIN:$EXTERNAL_BIN:$PATH"
must_fail_with 'repository-local PATH entry for argv[0] is rejected' 'inside the repository' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
ACTIVE_PATH="$EXTERNAL_BIN:$LOCAL_BIN:$PATH"
must_fail_with 'absolute argv[0] inside the repository is rejected' 'inside the repository' \
  inspect --file app.go --provider vendor-a --provider-command "$LOCAL_BIN/fake-reviewer --mode review"
must_fail_with 'relative argv[0] inside the repository is rejected' 'inside the repository' \
  inspect --file app.go --provider vendor-a --provider-command "./local-bin/fake-reviewer --mode review"

must_succeed 'inspect resolves argv[0] and creates approval token' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
APPROVAL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
PROVIDER_PATH="$(cd "$EXTERNAL_BIN" && pwd -P)/fake-reviewer"
if [ -n "$APPROVAL_TOKEN" ] && printf '%s\n' "$last_out" | grep -q '"package_sha256"' && \
   printf '%s\n' "$last_out" | grep -q '"binary_sha256"' && \
   printf '%s\n' "$last_out" | grep -q '"provider": "vendor-a"' && \
   printf '%s\n' "$last_out" | grep -Fq "\"provider_command\": \"$CMD\"" && \
   printf '%s\n' "$last_out" | grep -Fq "$PROVIDER_PATH"; then
  ok 'inspect discloses label, command template, binary path, binary hash, package hash, and token'
else
  bad 'inspect discloses label, command template, binary path, binary hash, package hash, and token'
fi

# --- approval token binding ---------------------------------------------------------

cp "$REPO/app.go" "$TMP/app.go.original"
printf '\n// changed after approval\n' >> "$REPO/app.go"
rm -f "$TMP/provider.env" "$TMP/provider.stdin" "$TMP/provider.args"
must_fail_with 'file substitution invalidates approval token' 'approval token does not match' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN"
if [ ! -e "$TMP/provider.env" ] && [ ! -e "$TMP/provider.stdin" ]; then
  ok 'file substitution is rejected before credentials or input reach provider'
else
  bad 'file substitution is rejected before credentials or input reach provider'
fi
cp "$TMP/app.go.original" "$REPO/app.go"

rm -f "$TMP/provider.env" "$TMP/provider.stdin"
must_fail_with 'command template change invalidates approval token' 'approval token does not match' \
  review --file app.go --provider vendor-a --author vendor-b \
  --provider-command "$CMD --unapproved-flag" --approve-external "$APPROVAL_TOKEN"
must_fail_with 'provider label change invalidates approval token' 'approval token does not match' \
  review --file app.go --provider vendor-c --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN"
if [ ! -e "$TMP/provider.env" ] && [ ! -e "$TMP/provider.stdin" ]; then
  ok 'command or label substitution is rejected before the provider runs'
else
  bad 'command or label substitution is rejected before the provider runs'
fi

cp "$EXTERNAL_BIN/fake-reviewer" "$TMP/fake-reviewer.original"
{
  printf '#!/usr/bin/env bash\n'
  printf 'printf executed > "%s/swapped-provider-ran"\n' "$TMP"
  printf 'env > "%s/provider.env"\n' "$TMP"
  printf 'cat > "%s/provider.stdin"\n' "$TMP"
} > "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"
rm -f "$TMP/provider.env" "$TMP/provider.stdin" "$TMP/swapped-provider-ran"
must_fail_with 'PATH binary substitution invalidates approval token' 'approval token does not match' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN"
if [ ! -e "$TMP/swapped-provider-ran" ] && [ ! -e "$TMP/provider.env" ] && \
   [ ! -e "$TMP/provider.stdin" ]; then
  ok 'binary substitution is rejected before credentials or input reach provider'
else
  bad 'binary substitution is rejected before credentials or input reach provider'
fi
cp "$TMP/fake-reviewer.original" "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"

must_succeed 'fresh inspection creates happy-path token' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
APPROVAL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"

# --- receipt destination rules ------------------------------------------------------

must_fail_with 'explicit receipt output must already exist' 'already exist' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN" --out "$TMP/missing-review-out"

repo_mode_before="$(mode_of "$REPO")"
must_fail_with 'explicit receipt output cannot overlap the repository' 'outside the repository' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN" --retain-transcript --out "$REPO"
repo_mode_after="$(mode_of "$REPO")"
chmod "$repo_mode_before" "$REPO"
if [ "$repo_mode_after" = "$repo_mode_before" ]; then
  ok 'rejected receipt output leaves repository permissions unchanged'
else
  bad 'rejected receipt output leaves repository permissions unchanged'
fi

# --- stdin delivery happy path ------------------------------------------------------

OUT="$TMP/review-out"
mkdir -m 700 "$OUT"
rm -f "$TMP/provider.delivery" "$TMP/provider.args"
must_succeed 'approved different-label review succeeds' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN" --out "$OUT"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | grep -q 'provider=vendor-a' && \
   printf '%s\n' "$last_out" | grep -Fq "command=\"$CMD\"" && \
   printf '%s\n' "$last_out" | grep -q 'files=1' && \
   printf '%s\n' "$last_out" | grep -q 'app.go' && \
   printf '%s\n' "$last_out" | grep -q 'reviewed by fake provider'; then
  ok 'pre-dispatch disclosure names label, command, and provider output is visible'
else
  bad 'pre-dispatch disclosure names label, command, and provider output is visible'
fi

if [ "$(cat "$TMP/provider.delivery" 2>/dev/null)" = stdin ] && \
   grep -q 'review-package' "$TMP/provider.stdin" && \
   [ "$(printf '%s\n' '--mode' 'review')" = "$(cat "$TMP/provider.args")" ]; then
  ok 'without {prompt_file} the prompt arrives on stdin and template args pass verbatim'
else
  bad 'without {prompt_file} the prompt arrives on stdin and template args pass verbatim'
fi

if [ -s "$TMP/provider.path" ] && [ "$(cat "$TMP/provider.path")" != "$PROVIDER_PATH" ]; then
  ok 'provider executes from a verified private copy'
else
  bad 'provider executes from a verified private copy'
fi

receipt="$(find "$OUT" -mindepth 2 -maxdepth 2 -type f -name receipt.json -print -quit 2>/dev/null)"
chunk_receipt="$(find "$OUT" -mindepth 3 -maxdepth 3 -type f -path '*/chunk-01/receipt.json' -print -quit 2>/dev/null)"
if [ -n "$receipt" ] && [ -n "$chunk_receipt" ] && [ "$(mode_of "$OUT")" = 700 ] && \
   [ "$(mode_of "$(dirname "$receipt")")" = 700 ] && \
   [ "$(mode_of "$(dirname "$chunk_receipt")")" = 700 ] && \
   [ "$(mode_of "$receipt")" = 600 ] && [ "$(mode_of "$chunk_receipt")" = 600 ]; then
  ok 'index and chunk receipt directories and files are private'
else
  bad 'index and chunk receipt directories and files are private'
fi

if [ -n "$chunk_receipt" ] && grep -q 'megapowers.advisory-review-receipt.v1' "$chunk_receipt" && \
   grep -q '"provider": "vendor-a"' "$chunk_receipt" && \
   grep -Fq "\"provider_command\": \"$CMD\"" "$chunk_receipt" && \
   grep -q '"binary_path"' "$chunk_receipt" && \
   grep -q '"binary_sha256"' "$chunk_receipt" && \
   grep -q '"author": "vendor-b"' "$chunk_receipt" && \
   grep -q '"advisory": true' "$chunk_receipt" && \
   grep -q '"sha256"' "$chunk_receipt"; then
  ok 'chunk receipt records label, command template, binary identity, author, and source'
else
  bad 'chunk receipt records label, command template, binary identity, author, and source'
fi

if [ -n "$receipt" ] && grep -q 'megapowers.advisory-review-index.v1' "$receipt" && \
   grep -q '"chunk_count": 1' "$receipt" && \
   grep -q '"author": "vendor-b"' "$receipt" && \
   grep -Fq "\"provider_command\": \"$CMD\"" "$receipt" && \
   grep -q 'chunk-01/receipt.json' "$receipt"; then
  ok 'index receipt lists the single chunk receipt and the command template'
else
  bad 'index receipt lists the single chunk receipt and the command template'
fi

file_count="$(find "$OUT" -type f | wc -l | tr -d ' ')"
if [ "$file_count" = 2 ]; then ok 'transcript is not retained by default'; else bad 'transcript is not retained by default'; fi

# --- environment passthrough ---------------------------------------------------------

if grep -q '^MEGAPOWERS_TEST_LEAK=' "$TMP/provider.env" || grep -q '^FAKE_API_KEY=' "$TMP/provider.env"; then
  bad 'provider environment excludes ambient variables unless named by --provider-env'
elif grep -q -- "$LOCAL_BIN" "$TMP/provider.env"; then
  bad 'provider PATH excludes repository-local entries'
elif grep -q '^HOME=' "$TMP/provider.env" && grep -q '^PATH=' "$TMP/provider.env"; then
  ok 'provider environment uses an explicit allowlist with sanitized PATH'
else
  bad 'provider environment preserves required allowlisted variables'
fi

ENV_OUT="$TMP/env-out"
mkdir -m 700 "$ENV_OUT"
must_succeed 'review accepts repeatable --provider-env names' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --provider-env FAKE_API_KEY --provider-env FAKE_HOME --provider-env FAKE_CONFIG_DIR \
  --approve-external "$APPROVAL_TOKEN" --out "$ENV_OUT"
if grep -q '^FAKE_API_KEY=test-credential$' "$TMP/provider.env" && \
   grep -q "^FAKE_HOME=$FAKE_HOME_VALUE\$" "$TMP/provider.env" && \
   grep -q "^FAKE_CONFIG_DIR=$FAKE_HOME_VALUE/config\$" "$TMP/provider.env" && \
   ! grep -q '^MEGAPOWERS_TEST_LEAK=' "$TMP/provider.env"; then
  ok '--provider-env forwards only the named variables'
else
  bad '--provider-env forwards only the named variables'
fi

must_fail_with 'invalid --provider-env name is rejected' 'provider-env' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --provider-env 'FAKE=VALUE' --approve-external "$APPROVAL_TOKEN" --out "$ENV_OUT"

FAKE_HOME_VALUE="$REPO/inside-home"
mkdir -p "$FAKE_HOME_VALUE/config"
must_succeed 'review with _HOME and _CONFIG_DIR values inside the repository still runs' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --provider-env FAKE_API_KEY --provider-env FAKE_HOME --provider-env FAKE_CONFIG_DIR \
  --approve-external "$APPROVAL_TOKEN" --out "$ENV_OUT"
if grep -q '^FAKE_API_KEY=test-credential$' "$TMP/provider.env" && \
   ! grep -q '^FAKE_HOME=' "$TMP/provider.env" && \
   ! grep -q '^FAKE_CONFIG_DIR=' "$TMP/provider.env"; then
  ok 'passed-through *_HOME and *_CONFIG_DIR values inside the repository are dropped'
else
  bad 'passed-through *_HOME and *_CONFIG_DIR values inside the repository are dropped'
fi
FAKE_HOME_VALUE="$TMP/fake-home"

# --- placeholders -------------------------------------------------------------------

FILE_CMD="fake-reviewer --prompt-file {prompt_file} --scratch {scratch_dir} --note 'hello world' --pipe 'a|b' --sub \"\$(id)\" back\\slash"
must_succeed 'inspect accepts quoted and placeholder arguments' \
  inspect --file app.go --provider vendor-a --provider-command "$FILE_CMD"
FILE_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
FILE_OUT="$TMP/file-out"
mkdir -m 700 "$FILE_OUT"
rm -f "$TMP/provider.delivery" "$TMP/provider.args" "$TMP/provider.stdin" "$TMP/provider.scratch"
must_succeed 'review with {prompt_file} and {scratch_dir} succeeds' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$FILE_CMD" \
  --approve-external "$FILE_TOKEN" --out "$FILE_OUT"
if [ "$(cat "$TMP/provider.delivery" 2>/dev/null)" = file ] && \
   grep -q 'review-package' "$TMP/provider.stdin" && \
   [ "$(cat "$TMP/provider.prompt-mode")" = 600 ] && \
   printf '%s\n' "$last_out" | grep -q 'reviewed by fake provider via file'; then
  ok '{prompt_file} delivers the prompt through a 0600 file with empty stdin'
else
  bad '{prompt_file} delivers the prompt through a 0600 file with empty stdin'
fi
scratch_dir="$(cat "$TMP/provider.scratch" 2>/dev/null)"
if [ -n "$scratch_dir" ] && [ "$(cat "$TMP/provider.prompt-dir")" = "$scratch_dir" ] && \
   case "$scratch_dir" in "$REPO"/*) false ;; *) true ;; esac && \
   [ ! -e "$scratch_dir" ]; then
  ok '{scratch_dir} expands to the private scratch directory holding the prompt file, removed after the run'
else
  bad '{scratch_dir} expands to the private scratch directory holding the prompt file, removed after the run'
fi
if grep -qx 'hello world' "$TMP/provider.args" && grep -qx 'a|b' "$TMP/provider.args" && \
   grep -qx '$(id)' "$TMP/provider.args" && grep -qx 'backslash' "$TMP/provider.args" && \
   ! grep -q '{prompt_file}' "$TMP/provider.args" && ! grep -q '{scratch_dir}' "$TMP/provider.args"; then
  ok 'quoted arguments are single argv entries and quoted metacharacters pass verbatim'
else
  bad 'quoted arguments are single argv entries and quoted metacharacters pass verbatim'
fi

# --- transcript retention -----------------------------------------------------------

TRANSCRIPT_OUT="$TMP/transcript-out"
mkdir -m 700 "$TRANSCRIPT_OUT"
must_succeed 'transcript retention is explicit opt-in' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN" \
  --retain-transcript --out "$TRANSCRIPT_OUT"
transcript_receipt="$(find "$TRANSCRIPT_OUT" -type f -path '*/chunk-01/receipt.json' -print -quit 2>/dev/null)"
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

# --- provider failure diagnostics ---------------------------------------------------

{
  printf '#!/usr/bin/env bash\n'
  printf 'cat >/dev/null\n'
  printf 'printf "\\033[31mprovider authentication failed\\033[0m\\n" >&2\n'
  printf 'printf "api_key = \\"sk-abcdefghijklmnopqrstuvwxyz123456\\"\\n" >&2\n'
  printf 'printf "OAUTH_TOKEN=eyJhbGciOiJIUzI1NiJ9.sensitive.signature\\n" >&2\n'
  printf 'printf "account=user@example.com org=org_12345 url=https://example.invalid/?token=secret\\n" >&2\n'
  printf 'printf "tenant=internal-customer\\n" >&2\n'
  printf 'i=0; while [ "$i" -lt 6000 ]; do printf x >&2; i=$((i + 1)); done\n'
  printf 'exit 23\n'
} > "$EXTERNAL_BIN/failing-reviewer"
chmod +x "$EXTERNAL_BIN/failing-reviewer"
FAIL_CMD='failing-reviewer --mode review'
must_succeed 'inspect failing provider binary for a bound token' \
  inspect --file app.go --provider vendor-b --provider-command "$FAIL_CMD"
FAIL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
FAIL_OUT="$TMP/failure-out"
mkdir -m 700 "$FAIL_OUT"
must_fail_with 'nonzero provider exit fails review' 'provider exited' \
  review --file app.go --provider vendor-b --author vendor-a --provider-command "$FAIL_CMD" \
  --approve-external "$FAIL_TOKEN" --out "$FAIL_OUT"
if printf '%s\n' "$last_out" | grep -q 'provider diagnostic: authentication failed; verify provider login or API credentials' && \
   ! printf '%s\n' "$last_out" | grep -q 'sk-abcdefghijklmnopqrstuvwxyz123456' && \
   ! printf '%s\n' "$last_out" | grep -q 'eyJhbGciOiJIUzI1NiJ9' && \
   ! printf '%s\n' "$last_out" | grep -q 'user@example.com' && \
   ! printf '%s\n' "$last_out" | grep -q 'org_12345' && \
   ! printf '%s\n' "$last_out" | grep -q 'example.invalid' && \
   ! printf '%s\n' "$last_out" | grep -q 'internal-customer' && \
   [[ "$last_out" != *$'\033'* ]] && \
   [ "$(printf '%s' "$last_out" | wc -c | tr -d ' ')" -le 4096 ]; then
  ok 'provider failure includes only a classified secret-safe diagnostic'
else
  bad 'provider failure includes only a classified secret-safe diagnostic'
fi
if [ ! -e "$FAIL_OUT" ] || [ -z "$(find "$FAIL_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'failed provider writes no receipt'
else
  bad 'failed provider writes no receipt'
fi

RO_OUT="$TMP/readonly-out"
mkdir -m 500 "$RO_OUT"
rm -f "$TMP/provider.stdin" "$TMP/provider.env"
must_fail_with 'unwritable receipt destination fails early' 'receipt' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$APPROVAL_TOKEN" --out "$RO_OUT"
if [ ! -e "$TMP/provider.stdin" ] && [ ! -e "$TMP/provider.env" ]; then
  ok 'unwritable receipt destination is rejected before the provider runs'
else
  bad 'unwritable receipt destination is rejected before the provider runs'
fi
chmod 700 "$RO_OUT"

{
  printf '#!/usr/bin/env bash\n'
  printf 'cat >/dev/null\n'
  printf 'printf "401 OAuth access token has expired. Please log in again.\\n" >&2\n'
  printf 'exit 0\n'
} > "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"
must_succeed 'inspect empty-output provider binary for a bound token' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
EMPTY_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
EMPTY_OUT="$TMP/empty-out"
mkdir -m 700 "$EMPTY_OUT"
must_fail_with 'empty provider output surfaces a classified diagnostic' \
  'authentication failed; verify provider login or API credentials' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$EMPTY_TOKEN" --out "$EMPTY_OUT"
if [ -z "$(find "$EMPTY_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'empty provider output writes no receipt'
else
  bad 'empty provider output writes no receipt'
fi

# --- chunking -----------------------------------------------------------------

install_fake_provider fake-reviewer
CHUNK_BASE="$(git -C "$REPO" rev-parse HEAD)"
for dir in alpha beta gamma; do
  mkdir -p "$REPO/$dir"
  for i in $(seq 1 60); do
    printf 'file %s %s\n' "$dir" "$i" > "$REPO/$dir/f$(printf '%03d' "$i").txt"
  done
done
for i in $(seq 1 20); do
  printf 'root %s\n' "$i" > "$REPO/root$(printf '%02d' "$i").txt"
done
git -C "$REPO" add alpha beta gamma root*.txt
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add 200 files'
CHUNK_HEAD="$(git -C "$REPO" rev-parse HEAD)"

must_succeed '200-file range is chunked instead of rejected' \
  inspect --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --provider-command "$CMD"
CHUNK_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | jq -e '
     .file_count == 200 and .chunk_count == 2 and (.chunks | length) == 2 and
     .chunks[0].file_count == 80 and .chunks[1].file_count == 120 and
     (.chunks[0].paths[0] | startswith("root")) and
     (.chunks[0].paths[-1] | startswith("alpha/")) and
     (.chunks[1].paths[0] | startswith("beta/")) and
     (.chunks[1].paths[-1] | startswith("gamma/")) and
     (.chunks[0].package_sha256 | length) == 64 and
     (.chunks[1].package_sha256 | length) == 64 and
     .chunks[0].package_sha256 != .chunks[1].package_sha256 and
     (.chunks[0].byte_count > 0) and (.chunks[1].byte_count > 0) and
     (.approval_token | startswith("mpr1_"))' >/dev/null && \
   [ "$(printf '%s\n' "$last_out" | grep -c '"approval_token"')" = 1 ]; then
  ok 'chunks group by top-level directory with one approval token'
else
  bad 'chunks group by top-level directory with one approval token'
fi

must_succeed 'chunked inspection is repeatable' \
  inspect --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --provider-command "$CMD"
if [ -n "$CHUNK_TOKEN" ] && [ "$(printf '%s\n' "$last_out" | json_value approval_token)" = "$CHUNK_TOKEN" ]; then
  ok 'chunked approval token is deterministic'
else
  bad 'chunked approval token is deterministic'
fi

must_fail_with 'chunk ceiling rejects excessive chunk counts' 'chunk ceiling' \
  inspect --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --provider-command "$CMD" \
  --max-files-per-chunk 10

printf 'file beta 1 changed\n' > "$REPO/beta/f001.txt"
git -C "$REPO" add beta/f001.txt
git -C "$REPO" -c commit.gpgsign=false commit -qm 'change one chunked file'
CHUNK_HEAD_CHANGED="$(git -C "$REPO" rev-parse HEAD)"
rm -f "$TMP/provider.calls"
must_fail_with 'changed chunk invalidates approval token' 'approval token does not match' \
  review --base "$CHUNK_BASE" --head "$CHUNK_HEAD_CHANGED" --provider vendor-a --author vendor-b \
  --provider-command "$CMD" --approve-external "$CHUNK_TOKEN"
if [ ! -e "$TMP/provider.calls" ]; then
  ok 'changed chunk is rejected before the provider runs'
else
  bad 'changed chunk is rejected before the provider runs'
fi

CHUNK_OUT="$TMP/chunk-out"
mkdir -m 700 "$CHUNK_OUT"
rm -f "$TMP/provider.calls"
must_succeed 'chunked review dispatches every chunk' \
  review --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --author vendor-b \
  --provider-command "$CMD" --approve-external "$CHUNK_TOKEN" --out "$CHUNK_OUT"
chunk_index="$(find "$CHUNK_OUT" -mindepth 2 -maxdepth 2 -name receipt.json -print -quit 2>/dev/null)"
if [ "$last_rc" -eq 0 ] && [ "$(grep -c 'reviewed by fake provider' <<<"$last_out")" = 2 ] && \
   [ "$(wc -l < "$TMP/provider.calls" | tr -d ' ')" = 3 ] && \
   [ -n "$chunk_index" ] && jq -e '.chunk_count == 2 and (.chunks | length) == 2 and
     .chunks[0].receipt == "chunk-01/receipt.json" and .chunks[1].receipt == "chunk-02/receipt.json"' \
     "$chunk_index" >/dev/null && \
   [ -f "$(dirname "$chunk_index")/chunk-01/receipt.json" ] && \
   [ -f "$(dirname "$chunk_index")/chunk-02/receipt.json" ] && \
   jq -e '.chunk.index == 2 and .chunk.count == 2 and (.source.files | length) == 120' \
     "$(dirname "$chunk_index")/chunk-02/receipt.json" >/dev/null; then
  ok 'chunked review writes one receipt per chunk plus an index receipt'
else
  bad 'chunked review writes one receipt per chunk plus an index receipt'
fi

{
  printf '#!/usr/bin/env bash\n'
  printf 'cat >/dev/null\n'
  printf 'n=0; [ -f "%s/fail.count" ] && n="$(cat "%s/fail.count")"\n' "$TMP" "$TMP"
  printf 'n=$((n + 1)); printf "%%s" "$n" > "%s/fail.count"\n' "$TMP"
  printf 'if [ "$n" -ge 3 ]; then printf "rate limit exceeded\\n" >&2; exit 1; fi\n'
  printf 'printf "reviewed chunk %%s\\n" "$n"\n'
} > "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"
rm -f "$TMP/fail.count"
must_succeed 'inspect chunk-failing provider for a bound token' \
  inspect --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --provider-command "$CMD"
CHUNK_FAIL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
CHUNK_FAIL_OUT="$TMP/chunk-fail-out"
mkdir -m 700 "$CHUNK_FAIL_OUT"
must_fail_with 'provider failure on a later chunk stops the run' 'chunk 2 of 2' \
  review --base "$CHUNK_BASE" --head "$CHUNK_HEAD" --provider vendor-a --author vendor-b \
  --provider-command "$CMD" --approve-external "$CHUNK_FAIL_TOKEN" --out "$CHUNK_FAIL_OUT"
if printf '%s\n' "$last_out" | grep -q 'completed chunks: 1' && \
   [ -z "$(find "$CHUNK_FAIL_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'chunk failure reports completed chunks and writes no receipt'
else
  bad 'chunk failure reports completed chunks and writes no receipt'
fi
install_fake_provider fake-reviewer

BYTES_BASE="$CHUNK_HEAD_CHANGED"
mkdir -p "$REPO/bulk"
for i in 1 2 3 4 5 6; do
  yes "bulk file $i lorem ipsum dolor sit amet" | head -c 300000 > "$REPO/bulk/b$i.txt"
done
git -C "$REPO" add bulk
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add bulk files'
BYTES_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_succeed 'range above the package byte limit is chunked' \
  inspect --base "$BYTES_BASE" --head "$BYTES_HEAD" --provider vendor-a --provider-command "$CMD"
if [ "$last_rc" -eq 0 ] && printf '%s\n' "$last_out" | jq -e '
     .file_count == 6 and .chunk_count >= 2 and
     ([.chunks[].byte_count] | max) <= 1048576 and
     ([.chunks[].file_count] | add) == 6' >/dev/null; then
  ok 'byte-limited chunks each stay under the package limit'
else
  bad 'byte-limited chunks each stay under the package limit'
fi

dd if=/dev/zero bs=600000 count=1 2>/dev/null | tr '\000' 'b' > "$REPO/bulk/huge.txt"
git -C "$REPO" add bulk/huge.txt
git -C "$REPO" -c commit.gpgsign=false commit -qm 'add oversized file'
HUGE_HEAD="$(git -C "$REPO" rev-parse HEAD)"
must_fail_with 'oversized single file in a range is still rejected' 'size limit' \
  inspect --base "$BYTES_HEAD" --head "$HUGE_HEAD" --provider vendor-a --provider-command "$CMD"

# --- preflight ------------------------------------------------------------------

{
  printf '#!/usr/bin/env bash\n'
  printf 'cat >/dev/null\n'
  printf 'printf "You'"'"'ve reached your usage limit\\n"\n'
  printf 'exit 1\n'
} > "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"
must_succeed 'inspect limit-exhausted provider for a bound token' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
LIMIT_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
LIMIT_OUT="$TMP/limit-out"
mkdir -m 700 "$LIMIT_OUT"
must_fail_with 'preflight classifies a usage-limit failure' 'preflight' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$LIMIT_TOKEN" --out "$LIMIT_OUT"
if printf '%s\n' "$last_out" | grep -q 'provider vendor-a' && \
   printf '%s\n' "$last_out" | grep -qi 'auth-or-limit' && \
   [ -z "$(find "$LIMIT_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'preflight limit failure names provider label and class and writes no receipt'
else
  bad 'preflight limit failure names provider label and class and writes no receipt'
fi

{
  printf '#!/usr/bin/env bash\n'
  printf 'exec sleep 30\n'
} > "$EXTERNAL_BIN/fake-reviewer"
chmod +x "$EXTERNAL_BIN/fake-reviewer"
must_succeed 'inspect stalled provider for a bound token' \
  inspect --file app.go --provider vendor-a --provider-command "$CMD"
STALL_TOKEN="$(printf '%s\n' "$last_out" | json_value approval_token)"
STALL_OUT="$TMP/stall-out"
mkdir -m 700 "$STALL_OUT"
stall_started="$(date +%s)"
must_fail_with 'preflight times out on a stalled provider' 'timeout' \
  review --file app.go --provider vendor-a --author vendor-b --provider-command "$CMD" \
  --approve-external "$STALL_TOKEN" --out "$STALL_OUT" --preflight-timeout 1s
stall_elapsed="$(( $(date +%s) - stall_started ))"
if printf '%s\n' "$last_out" | grep -q 'provider vendor-a' && [ "$stall_elapsed" -lt 15 ] && \
   [ -z "$(find "$STALL_OUT" -name receipt.json -print -quit 2>/dev/null)" ]; then
  ok 'preflight timeout exits within the window and writes no receipt'
else
  bad "preflight timeout exits within the window and writes no receipt (elapsed ${stall_elapsed}s)"
fi
install_fake_provider fake-reviewer

printf '%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
