: << 'CMDBLOCK'
@echo off
setlocal enabledelayedexpansion

if "%~1"=="" (
    echo megapowers hook: cannot run hook: missing hook name 1>&2
    exit /b 1
)
set "HOOK_NAME=%~1"

set "HOOK_DIR=%~dp0"
set "PLUGIN_DIR=%HOOK_DIR%.."
set "CACHE_BASE=%MEGAPOWERS_HOOK_CACHE%"
if not defined CACHE_BASE set "CACHE_BASE=%LOCALAPPDATA%"
if not defined CACHE_BASE if defined USERPROFILE set "CACHE_BASE=%USERPROFILE%\.cache"
if not defined CACHE_BASE (
    set "REASON=cannot resolve a private cache directory"
    goto fail
)
set "CACHE_DIR=%CACHE_BASE%\megapowers-hooks"
if not defined PROCESSOR_ARCHITECTURE (
    set "REASON=cannot resolve Windows architecture"
    goto fail
)
set "GO111MODULE=off"
set "GOTOOLCHAIN=local"
set "GO_VERSION="
set "GO_KEY="
set "GO_FOUND="
where go >nul 2>nul
if not errorlevel 1 (
    set "GO_FOUND=1"
    for /f "delims=" %%v in ('go env GOVERSION 2^>nul') do set "GO_VERSION=%%v"
)
if defined GO_VERSION (
    set "GO_KEY=!GO_VERSION:devel =!"
    for /f "tokens=1" %%k in ("!GO_KEY!") do set "GO_KEY=%%k"
)
set "RUNNER_PREFIX=%CACHE_DIR%\megapowers-hook-bc35fe0f66e19a84-windows-%PROCESSOR_ARCHITECTURE%"
set "RUNNER="
if defined GO_KEY (
    set "RUNNER=!RUNNER_PREFIX!-!GO_KEY!.exe"
) else (
    for %%f in ("!RUNNER_PREFIX!-*.exe") do (
        if /i not "%%~xf"==".tmp" set "RUNNER=%%~ff"
    )
)
if not defined RUNNER (
    if defined GO_FOUND (
        set "REASON=cannot build hook runner: cannot read Go version (go env GOVERSION failed)"
    ) else (
        set "REASON=cannot build hook runner: Go 1.25 or newer is required"
    )
    goto fail
)

if exist "%CACHE_DIR%" (
    fsutil reparsepoint query "%CACHE_DIR%" >nul 2>nul
    if not errorlevel 1 (
        set "REASON=refusing symlink hook cache: %CACHE_DIR%"
        goto fail
    )
)
if not exist "%CACHE_DIR%" mkdir "%CACHE_DIR%" >nul 2>nul
if not exist "%CACHE_DIR%" (
    set "REASON=cannot create hook cache: %CACHE_DIR%"
    goto fail
)
if exist "%RUNNER%" (
    fsutil reparsepoint query "%RUNNER%" >nul 2>nul
    if not errorlevel 1 (
        set "REASON=refusing symlink cached runner: %RUNNER%"
        goto fail
    )
)
if exist "%RUNNER%\" (
    set "REASON=cached runner is not a file: %RUNNER%"
    goto fail
)

if not exist "%RUNNER%" (
    set "GO_SEMVER=!GO_KEY:go=!"
    set "GO_MAJOR=0"
    set "GO_MINOR=0"
    for /f "tokens=1,2 delims=.-" %%a in ("!GO_SEMVER!") do (
        set "GO_MAJOR=%%a"
        if not "%%b"=="" set "GO_MINOR=%%b"
    )
    set "GO_OK="
    if !GO_MAJOR! GTR 1 set "GO_OK=1"
    if !GO_MAJOR! EQU 1 if !GO_MINOR! GEQ 25 set "GO_OK=1"
    if not defined GO_OK (
        set "REASON=cannot build hook runner: Go 1.25 or newer is required (found !GO_KEY!)"
        goto fail
    )
    if not defined GOCACHE (
        set "GO_DEFAULT_CACHE="
        for /f "delims=" %%c in ('go env GOCACHE 2^>nul') do set "GO_DEFAULT_CACHE=%%c"
        if not defined GO_DEFAULT_CACHE set "GOCACHE=%CACHE_DIR%\go-build"
        if /i "!GO_DEFAULT_CACHE!"=="off" set "GOCACHE=%CACHE_DIR%\go-build"
    )
    set "TMP_RUNNER=%CACHE_DIR%\build-!RANDOM!.tmp.exe"
    go build -trimpath -o "!TMP_RUNNER!" "%HOOK_DIR%hook_runner.go" "%HOOK_DIR%deny_destructive.go" "%HOOK_DIR%output_style.go" "%HOOK_DIR%gate_context.go" "%HOOK_DIR%doctor.go"
    if errorlevel 1 (
        del /q "!TMP_RUNNER!" >nul 2>nul
        set "REASON=cannot build cached hook runner"
        goto fail
    )
    move /y "!TMP_RUNNER!" "%RUNNER%" >nul
    if errorlevel 1 (
        if exist "%RUNNER%" (
            del /q "!TMP_RUNNER!" >nul 2>nul
        ) else (
            del /q "!TMP_RUNNER!" >nul 2>nul
            set "REASON=cannot install cached hook runner"
            goto fail
        )
    )
)

set "MEGAPOWERS_PLUGIN_ROOT=%PLUGIN_DIR%"
set "MEGAPOWERS_HOOK_CACHE_DIR=%CACHE_DIR%"
"%RUNNER%" %*
set "RC=!errorlevel!"
exit /b !RC!

:fail
rem A launcher failure never blocks the tool call: the guard hooks warn the
rem user through systemMessage and exit 0; doctor keeps a hard failure.
echo megapowers hook: !REASON! 1>&2
if /i "%HOOK_NAME%"=="deny-destructive" goto warn
if /i "%HOOK_NAME%"=="session-start" goto warn
if /i "%HOOK_NAME%"=="subagent-start" exit /b 0
exit /b 1
:warn
set "JSON_REASON=!REASON:\=\\!"
set JSON_REASON=!JSON_REASON:"=\"!
echo {"systemMessage":"megapowers: destructive-command guard is inactive (!JSON_REASON!). Run megapowers-doctor for the fix."}
exit /b 0
CMDBLOCK

set -u
umask 077

if [ "$#" -ne 1 ]; then
  printf 'megapowers hook: cannot run hook: expected deny-destructive, session-start, subagent-start, output-style, or doctor\n' >&2
  exit 1
fi
hook_name="$1"

# json_escape quotes a short message for a JSON string using only shell
# builtins, so it works under a PATH without sed or awk.
json_escape() {
  remaining="$1"
  escaped=""
  while [ -n "$remaining" ]; do
    char="${remaining%"${remaining#?}"}"
    remaining="${remaining#?}"
    case "$char" in
      \\) escaped="$escaped\\\\" ;;
      \") escaped="$escaped\\\"" ;;
      *) escaped="$escaped$char" ;;
    esac
  done
  printf '%s' "$escaped"
}

# fail reports a launcher failure. The guard hooks must never block a tool
# call because the launcher itself broke: they warn the user via
# systemMessage and exit 0. subagent-start stays silent; doctor fails hard.
fail() {
  printf 'megapowers hook: %s\n' "$1" >&2
  case "$hook_name" in
    deny-destructive|session-start)
      printf '{"systemMessage":"megapowers: destructive-command guard is inactive (%s). Run megapowers-doctor for the fix."}\n' "$(json_escape "$1")"
      exit 0
      ;;
    subagent-start)
      exit 0
      ;;
  esac
  exit 1
}

hook_dir="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)" || fail "cannot resolve hook directory"
plugin_dir="$(CDPATH='' cd -- "$hook_dir/.." && pwd)" || fail "cannot resolve plugin directory"
platform_os="$(uname -s)" || fail "cannot resolve operating system"
platform_arch="$(uname -m)" || fail "cannot resolve architecture"
case "$platform_os:$platform_arch" in
  *[!A-Za-z0-9_.:-]*) fail "unsafe platform identifier" ;;
esac

# The Go version is part of the cache key: a toolchain upgrade rebuilds the
# runner instead of reusing a binary built by an older Go.
export GO111MODULE=off GOTOOLCHAIN=local
go_key=""
go_found=0
if command -v go >/dev/null 2>&1; then
  go_found=1
  go_version="$(go env GOVERSION 2>/dev/null)" || go_version=""
  go_key="${go_version#devel }"
  go_key="${go_key%% *}"
  case "$go_key" in
    ''|*[!A-Za-z0-9_.+-]*) go_key="" ;;
  esac
fi

go_supported() {
  semver="${1#go}"
  major="${semver%%.*}"
  rest="${semver#*.}"
  minor="${rest%%[!0-9]*}"
  case "$major$minor" in
    ''|*[!0-9]*) return 1 ;;
  esac
  [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 25 ]; }
}

ephemeral_cache=0
if [ -n "${MEGAPOWERS_HOOK_CACHE:-}" ]; then
  cache_base="$MEGAPOWERS_HOOK_CACHE"
elif [ -n "${XDG_CACHE_HOME:-}" ]; then
  cache_base="$XDG_CACHE_HOME"
elif [ -n "${HOME:-}" ]; then
  cache_base="$HOME/.cache"
else
  scratch_root="${TMPDIR:-/tmp}"
  cache_dir="$(mktemp -d "$scratch_root/megapowers-hooks.XXXXXX")" || fail "cannot create private temporary hook cache"
  ephemeral_cache=1
fi

if [ "$ephemeral_cache" -eq 0 ]; then
  cache_dir="$cache_base/megapowers-hooks"
  if [ -L "$cache_dir" ]; then
    fail "refusing symlink hook cache: $cache_dir"
  fi
  if [ -e "$cache_dir" ] && [ ! -d "$cache_dir" ]; then
    fail "hook cache is not a directory: $cache_dir"
  fi
  mkdir -p -- "$cache_dir" || fail "cannot create hook cache: $cache_dir"
  if [ -L "$cache_dir" ]; then
    fail "refusing symlink hook cache: $cache_dir"
  fi
fi
chmod 700 "$cache_dir" || fail "cannot secure hook cache: $cache_dir"

cleanup() {
  [ "$ephemeral_cache" -eq 1 ] && rm -rf -- "$cache_dir"
}
trap cleanup EXIT HUP INT TERM

runner_prefix="$cache_dir/megapowers-hook-bc35fe0f66e19a84-$platform_os-$platform_arch"
if [ -n "$go_key" ]; then
  runner="$runner_prefix-$go_key"
else
  # Without Go on PATH the newest cached runner for these sources still serves.
  runner=""
  for candidate in "$runner_prefix"-*; do
    [ -e "$candidate" ] || continue
    case "$candidate" in *.tmp) continue ;; esac
    if [ ! -L "$candidate" ] && [ -f "$candidate" ] && [ -x "$candidate" ]; then
      runner="$candidate"
    fi
  done
  if [ -z "$runner" ]; then
    if [ "$go_found" -eq 1 ]; then
      fail "cannot build hook runner: cannot read Go version (go env GOVERSION failed)"
    fi
    fail "cannot build hook runner: Go 1.25 or newer is required"
  fi
fi
if [ -L "$runner" ]; then
  fail "refusing symlink cached runner: $runner"
fi
if [ -e "$runner" ] && { [ ! -f "$runner" ] || [ ! -x "$runner" ]; }; then
  fail "cached runner is not an executable file: $runner"
fi

if [ ! -x "$runner" ]; then
  if ! go_supported "$go_key"; then
    fail "cannot build hook runner: Go 1.25 or newer is required (found $go_key)"
  fi
  # Prefer the user's normal Go build cache so a cold build reuses the
  # compiled standard library; fall back to a private cache only when the
  # default is unset or unwritable.
  go_cache="${GOCACHE:-}"
  if [ -z "$go_cache" ]; then
    go_cache="$(go env GOCACHE 2>/dev/null)" || go_cache=""
    case "$go_cache" in
      ''|off) go_cache="$cache_dir/go-build" ;;
    esac
    if ! mkdir -p -- "$go_cache" 2>/dev/null || [ ! -w "$go_cache" ]; then
      go_cache="$cache_dir/go-build"
    fi
  fi
  # The temp name must not match the "$runner_prefix"-* fallback glob.
  tmp_runner="$cache_dir/build-$$.tmp"
  if ! GOCACHE="$go_cache" go build -trimpath -o "$tmp_runner" \
    "$hook_dir/hook_runner.go" "$hook_dir/deny_destructive.go" "$hook_dir/output_style.go" \
    "$hook_dir/gate_context.go" "$hook_dir/doctor.go"; then
    rm -f -- "$tmp_runner"
    fail "cannot build cached hook runner"
  fi
  if ! mv -f -- "$tmp_runner" "$runner"; then
    rm -f -- "$tmp_runner"
    if [ ! -x "$runner" ] || [ -L "$runner" ]; then
      fail "cannot install cached hook runner"
    fi
  fi
  chmod 700 "$runner" || fail "cannot secure cached hook runner"
fi

export MEGAPOWERS_PLUGIN_ROOT="$plugin_dir" MEGAPOWERS_HOOK_CACHE_DIR="$cache_dir"
if [ "$ephemeral_cache" -eq 1 ]; then
  "$runner" "$hook_name"
  exit $?
fi
trap - EXIT HUP INT TERM
exec "$runner" "$hook_name"
