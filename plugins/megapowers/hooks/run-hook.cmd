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
set "RUNNER=%CACHE_DIR%\megapowers-hook-b8ae5a54608e71f6-windows-%PROCESSOR_ARCHITECTURE%.exe"

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
    where go >nul 2>nul
    if errorlevel 1 (
        echo megapowers hook: cannot build hook runner: Go 1.25 or newer is required 1>&2
        exit /b 1
    )
    set "TMP_RUNNER=%RUNNER%.!RANDOM!.tmp.exe"
    set "GO111MODULE=off"
    set "GOTOOLCHAIN=local"
    if not defined GOCACHE set "GOCACHE=%CACHE_DIR%\go-build"
    go build -trimpath -o "!TMP_RUNNER!" "%HOOK_DIR%hook_runner.go" "%HOOK_DIR%deny_destructive.go" "%HOOK_DIR%output_style.go"
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
"%RUNNER%" %*
set "RC=!errorlevel!"
exit /b !RC!
CMDBLOCK

set -u
umask 077

if [ "$#" -ne 1 ]; then
  printf 'megapowers hook: cannot run hook: expected deny-destructive, session-start, or output-style\n' >&2
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

runner="$cache_dir/megapowers-hook-b8ae5a54608e71f6-$platform_os-$platform_arch"
if [ -L "$runner" ]; then
  printf 'megapowers hook: refusing symlink cached runner: %s\n' "$runner" >&2
  exit 1
fi
if [ -e "$runner" ] && { [ ! -f "$runner" ] || [ ! -x "$runner" ]; }; then
  printf 'megapowers hook: cached runner is not an executable file: %s\n' "$runner" >&2
  exit 1
fi

if [ ! -x "$runner" ]; then
  command -v go >/dev/null 2>&1 || {
    printf 'megapowers hook: cannot build hook runner: Go 1.25 or newer is required\n' >&2
    exit 1
  }
  tmp_runner="$runner.$$.tmp"
  go_cache="${GOCACHE:-$cache_dir/go-build}"
  if ! GO111MODULE=off GOTOOLCHAIN=local GOCACHE="$go_cache" go build -trimpath -o "$tmp_runner" \
    "$hook_dir/hook_runner.go" "$hook_dir/deny_destructive.go" "$hook_dir/output_style.go"; then
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

if [ "$ephemeral_cache" -eq 1 ]; then
  MEGAPOWERS_PLUGIN_ROOT="$plugin_dir" "$runner" "$hook_name"
  exit $?
fi
trap - EXIT HUP INT TERM
MEGAPOWERS_PLUGIN_ROOT="$plugin_dir" exec "$runner" "$hook_name"
