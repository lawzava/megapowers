#!/usr/bin/env bash
# Dependency-free tests for probe-routes. Every probe is faked through PATH and a
# stubbed HOME so the result never depends on which CLIs this machine has.
# Run: plugins/mega-orchestration/skills/multi-agent-delegation/scripts/tests/probe-routes.test.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PR="$HERE/../probe-routes"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export XDG_CONFIG_HOME="$TMP/xdg"
export HOME="$TMP/home"
mkdir -p "$TMP/xdg" "$TMP/home" "$TMP/bin"
REAL_PATH="$PATH"

pass=0; fail=0
check() {  # $1=desc $2=want-substring $3=got
  if printf '%s' "$3" | grep -qF -- "$2"; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL %s\n    want: %s\n    got:  %s\n' "$1" "$2" "$3"; fi
}
absent() {  # $1=desc $2=unwanted-substring $3=got
  if printf '%s' "$3" | grep -qF -- "$2"; then fail=$((fail+1)); printf '  FAIL %s\n    unwanted: %s\n    got:  %s\n' "$1" "$2" "$3"; else pass=$((pass+1)); fi
}
check_exit() {  # $1=desc $2=want-code $3=got-code
  if [ "$2" = "$3" ]; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL %s (want exit %s, got %s)\n' "$1" "$2" "$3"; fi
}

# A fake harness binary. `opencode models` prints a canned catalogue so the probe
# never reaches the network. The stub bodies are single-quoted on purpose: they are
# scripts to be written out verbatim, not strings to expand here.
# shellcheck disable=SC2016
stub() {  # $1=name  $2=body
  printf '#!/usr/bin/env bash\n%s\n' "$2" > "$TMP/bin/$1"
  chmod +x "$TMP/bin/$1"
}

echo "== probe-routes tests =="

# --- nothing installed ---
PATH="$TMP/bin:/usr/bin:/bin"
out="$("$PR" 2>&1)"; rc=$?
check_exit "no harness installed still exits 0" 0 "$rc"
check "an absent harness is reported, not omitted" "claude" "$out"
check "absent harnesses are marked" "binary=no" "$out"
check "zero reachable vendors is stated plainly" "ALTERNATES=0" "$out"

# --- claude only: one vendor, so no independence ---
stub claude 'exit 0'
out="$("$PR" 2>&1)"
check "an installed harness is marked present" "claude" "$out"
check "claude alone reports its models" "claude-opus-5" "$out"
check "one vendor is not an independence route" "ALTERNATES=0" "$out"

# --- claude + codex: the shipped two-vendor baseline ---
stub codex 'exit 0'
out="$("$PR" 2>&1)"
check "codex is detected" "codex" "$out"
check "codex models come from the catalog" "gpt-5.6-sol" "$out"
check "two vendors give one alternate" "ALTERNATES=1" "$out"

# --- opencode with a model catalogue: the openrouter-hosted vendors ---
# shellcheck disable=SC2016
stub opencode 'if [ "${1:-}" = "models" ]; then
  printf "openrouter/qwen/qwen3.8-max\nopenrouter/moonshotai/kimi-k3\nopenrouter/deepseek/deepseek-v4-pro\nopenrouter/x-ai/grok-4.5\nopencode/big-pickle\n"
  exit 0
fi
exit 0'
out="$("$PR" 2>&1)"
check "opencode is detected" "opencode" "$out"
check "a catalogued model is reported reachable" "qwen3.8-max" "$out"
check "so is the fourth vendor" "kimi-k3" "$out"
# Six vendors reachable (claude, codex, and the four opencode hosts), so an
# artifact from any one of them has five alternates.
check "every reachable vendor counts toward independence" "ALTERNATES=5" "$out"

# --- a harness present but carrying none of the catalogue's models ---
# shellcheck disable=SC2016
stub opencode 'if [ "${1:-}" = "models" ]; then printf "opencode/some-other-model\n"; exit 0; fi
exit 0'
out="$("$PR" 2>&1)"
absent "an uncatalogued model is not invented as a route" "qwen3.8-max" "$out"
check "a harness with no catalogued model drops back to two vendors" "ALTERNATES=1" "$out"

# --- a harness whose model listing fails must not be read as empty ---
# shellcheck disable=SC2016
stub opencode 'if [ "${1:-}" = "models" ]; then echo "auth error" >&2; exit 1; fi
exit 0'
out="$("$PR" 2>&1)"
check "a failed listing is reported as unknown, not as none" "models=unknown" "$out"

# --- --suggest emits a valid override layer ---
# shellcheck disable=SC2016
# shellcheck disable=SC2016
stub opencode 'if [ "${1:-}" = "models" ]; then
  printf "openrouter/qwen/qwen3.8-max\nopenrouter/moonshotai/kimi-k3\n"
  exit 0
fi
exit 0'
out="$("$PR" --suggest 2>&1)"; rc=$?
check_exit "--suggest exits 0" 0 "$rc"
check "--suggest emits a providers table" "[providers.qwen]" "$out"
check "--suggest enables what it found" "enabled = true" "$out"
check "--suggest is a paste-ready comment header" "# " "$out"
absent "--suggest never emits a provider it could not reach" "[providers.deepseek]" "$out"

# --suggest must be parseable TOML, since the user pastes it into a real layer.
printf '%s\n' "$out" > "$TMP/suggested.toml"
if command -v python3 >/dev/null 2>&1; then
  if python3 -c "import tomllib,sys; tomllib.load(open(sys.argv[1],'rb'))" "$TMP/suggested.toml" 2>/dev/null; then
    pass=$((pass+1))
  else
    fail=$((fail+1)); printf '  FAIL --suggest output must parse as TOML\n'
  fi
fi

# --- the probe never writes ---
before="$(find "$TMP/xdg" "$TMP/home" -type f 2>/dev/null | sort)"
"$PR" --suggest >/dev/null 2>&1
after="$(find "$TMP/xdg" "$TMP/home" -type f 2>/dev/null | sort)"
if [ "$before" = "$after" ]; then pass=$((pass+1)); else fail=$((fail+1)); printf '  FAIL probe-routes must not write anything\n'; fi

# --- unknown flags are refused rather than ignored ---
out="$("$PR" --wat 2>&1)"; rc=$?
check_exit "an unknown flag exits 2" 2 "$rc"
check "and says what it accepts" "--suggest" "$out"

PATH="$REAL_PATH"
echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
