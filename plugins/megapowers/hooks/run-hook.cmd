: << 'CMDBLOCK'
@echo off
rem Cross-platform wrapper for the destructive-command hook.
rem Usage: run-hook.cmd <script-name> [args...]

if "%~1"=="" (
    echo run-hook.cmd: cannot run hook: missing script name 1>&2
    exit /b 1
)

set "HOOK_DIR=%~dp0"
if not exist "%HOOK_DIR%%~1" (
    echo run-hook.cmd: cannot run missing hook target: %~1 1>&2
    exit /b 1
)

rem Try Git for Windows bash in standard locations
if exist "C:\Program Files\Git\bin\bash.exe" (
    "C:\Program Files\Git\bin\bash.exe" "%HOOK_DIR%%~1" %2 %3 %4 %5 %6 %7 %8 %9
    if errorlevel 1 exit /b 1
    exit /b 0
)
if exist "C:\Program Files (x86)\Git\bin\bash.exe" (
    "C:\Program Files (x86)\Git\bin\bash.exe" "%HOOK_DIR%%~1" %2 %3 %4 %5 %6 %7 %8 %9
    if errorlevel 1 exit /b 1
    exit /b 0
)

rem Try bash on PATH (e.g. user-installed Git Bash, MSYS2, Cygwin)
where bash >nul 2>nul
if %ERRORLEVEL% equ 0 (
    bash "%HOOK_DIR%%~1" %2 %3 %4 %5 %6 %7 %8 %9
    if errorlevel 1 exit /b 1
    exit /b 0
)

echo run-hook.cmd: cannot run hook: bash is unavailable 1>&2
exit /b 1
CMDBLOCK

# Unix: run the named script directly
if [ "$#" -lt 1 ]; then
  printf 'run-hook.cmd: cannot run hook: missing script name\n' >&2
  exit 1
fi
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT_NAME="$1"
shift
if [ ! -f "${SCRIPT_DIR}/${SCRIPT_NAME}" ]; then
  printf 'run-hook.cmd: cannot run missing hook target: %s\n' "$SCRIPT_NAME" >&2
  exit 1
fi
exec bash "${SCRIPT_DIR}/${SCRIPT_NAME}" "$@"
