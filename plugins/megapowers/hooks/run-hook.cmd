: << 'CMDBLOCK'
@echo off
setlocal enabledelayedexpansion

if "%~1"=="" (
    echo megapowers hook: cannot run hook: missing hook name 1>&2
    exit /b 1
)

set "HOOK_DIR=%~dp0"
set "PLUGIN_DIR=%HOOK_DIR%.."
set "CACHE_BASE=%MEGAPOWERS_HOOK_CACHE%"
if not defined CACHE_BASE set "CACHE_BASE=%LOCALAPPDATA%"
if not defined CACHE_BASE if defined USERPROFILE set "CACHE_BASE=%USERPROFILE%\.cache"
if not defined CACHE_BASE (
    echo megapowers hook: cannot resolve a private cache directory 1>&2
    exit /b 1
)
set "CACHE_DIR=%CACHE_BASE%\megapowers-hooks"
if not defined PROCESSOR_ARCHITECTURE (
    echo megapowers hook: cannot resolve Windows architecture 1>&2
    exit /b 1
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
        echo megapowers hook: cannot build hook runner: cannot read Go version ^(go env GOVERSION failed^) 1>&2
    ) else (
        echo megapowers hook: cannot build hook runner: Go 1.25 or newer is required 1>&2
    )
    exit /b 1
)

if exist "%CACHE_DIR%" (
    fsutil reparsepoint query "%CACHE_DIR%" >nul 2>nul
    if not errorlevel 1 (
        echo megapowers hook: refusing symlink hook cache: %CACHE_DIR% 1>&2
        exit /b 1
    )
)
if not exist "%CACHE_DIR%" mkdir "%CACHE_DIR%" >nul 2>nul
if not exist "%CACHE_DIR%" (
    echo megapowers hook: cannot create hook cache: %CACHE_DIR% 1>&2
    exit /b 1
)
if exist "%RUNNER%" (
    fsutil reparsepoint query "%RUNNER%" >nul 2>nul
    if not errorlevel 1 (
        echo megapowers hook: refusing symlink cached runner: %RUNNER% 1>&2
        exit /b 1
    )
)
if exist "%RUNNER%\" (
    echo megapowers hook: cached runner is not a file: %RUNNER% 1>&2
    exit /b 1
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
        echo megapowers hook: cannot build hook runner: Go 1.25 or newer is required ^(found !GO_KEY!^) 1>&2
        exit /b 1
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
        echo megapowers hook: cannot build cached hook runner 1>&2
        exit /b 1
    )
    move /y "!TMP_RUNNER!" "%RUNNER%" >nul
    if errorlevel 1 (
        if exist "%RUNNER%" (
            del /q "!TMP_RUNNER!" >nul 2>nul
        ) else (
            del /q "!TMP_RUNNER!" >nul 2>nul
            echo megapowers hook: cannot install cached hook runner 1>&2
            exit /b 1
        )
    )
)

set "MEGAPOWERS_PLUGIN_ROOT=%PLUGIN_DIR%"
set "MEGAPOWERS_HOOK_CACHE_DIR=%CACHE_DIR%"
"%RUNNER%" %*
set "RC=!errorlevel!"
exit /b !RC!
CMDBLOCK

set -u
umask 077

if [ "$#" -ne 1 ]; then
  printf 'megapowers hook: cannot run hook: expected deny-destructive, session-start, subagent-start, output-style, or doctor\n' >&2
  exit 1
fi
hook_name="$1"

hook_dir="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)" || {
  printf 'megapowers hook: cannot resolve hook directory\n' >&2
  exit 1
}
plugin_dir="$(CDPATH='' cd -- "$hook_dir/.." && pwd)" || {
  printf 'megapowers hook: cannot resolve plugin directory\n' >&2
  exit 1
}
platform_os="$(uname -s)" || {
  printf 'megapowers hook: cannot resolve operating system\n' >&2
  exit 1
}
platform_arch="$(uname -m)" || {
  printf 'megapowers hook: cannot resolve architecture\n' >&2
  exit 1
}
case "$platform_os:$platform_arch" in
  *[!A-Za-z0-9_.:-]*)
    printf 'megapowers hook: unsafe platform identifier\n' >&2
    exit 1
    ;;
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
  cache_dir="$(mktemp -d "$scratch_root/megapowers-hooks.XXXXXX")" || {
    printf 'megapowers hook: cannot create private temporary hook cache\n' >&2
    exit 1
  }
  ephemeral_cache=1
fi

if [ "$ephemeral_cache" -eq 0 ]; then
  cache_dir="$cache_base/megapowers-hooks"
  if [ -L "$cache_dir" ]; then
    printf 'megapowers hook: refusing symlink hook cache: %s\n' "$cache_dir" >&2
    exit 1
  fi
  if [ -e "$cache_dir" ] && [ ! -d "$cache_dir" ]; then
    printf 'megapowers hook: hook cache is not a directory: %s\n' "$cache_dir" >&2
    exit 1
  fi
  mkdir -p -- "$cache_dir" || {
    printf 'megapowers hook: cannot create hook cache: %s\n' "$cache_dir" >&2
    exit 1
  }
  if [ -L "$cache_dir" ]; then
    printf 'megapowers hook: refusing symlink hook cache: %s\n' "$cache_dir" >&2
    exit 1
  fi
fi
chmod 700 "$cache_dir" || {
  printf 'megapowers hook: cannot secure hook cache: %s\n' "$cache_dir" >&2
  exit 1
}

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
      printf 'megapowers hook: cannot build hook runner: cannot read Go version (go env GOVERSION failed)\n' >&2
    else
      printf 'megapowers hook: cannot build hook runner: Go 1.25 or newer is required\n' >&2
    fi
    exit 1
  fi
fi
if [ -L "$runner" ]; then
  printf 'megapowers hook: refusing symlink cached runner: %s\n' "$runner" >&2
  exit 1
fi
if [ -e "$runner" ] && { [ ! -f "$runner" ] || [ ! -x "$runner" ]; }; then
  printf 'megapowers hook: cached runner is not an executable file: %s\n' "$runner" >&2
  exit 1
fi

if [ ! -x "$runner" ]; then
  if ! go_supported "$go_key"; then
    printf 'megapowers hook: cannot build hook runner: Go 1.25 or newer is required (found %s)\n' "$go_key" >&2
    exit 1
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
    printf 'megapowers hook: cannot build cached hook runner\n' >&2
    exit 1
  fi
  if ! mv -f -- "$tmp_runner" "$runner"; then
    rm -f -- "$tmp_runner"
    if [ ! -x "$runner" ] || [ -L "$runner" ]; then
      printf 'megapowers hook: cannot install cached hook runner\n' >&2
      exit 1
    fi
  fi
  chmod 700 "$runner" || {
    printf 'megapowers hook: cannot secure cached hook runner\n' >&2
    exit 1
  }
fi

export MEGAPOWERS_PLUGIN_ROOT="$plugin_dir" MEGAPOWERS_HOOK_CACHE_DIR="$cache_dir"
if [ "$ephemeral_cache" -eq 1 ]; then
  "$runner" "$hook_name"
  exit $?
fi
trap - EXIT HUP INT TERM
exec "$runner" "$hook_name"
