@echo off
chcp 65001 >nul 2>&1

rem ===========================================================================
rem  Double-click to start ai-service + backend + frontend.
rem
rem  Why this file exists: Windows has no idea what a .sh file is -- double
rem  clicking one just opens the "How do you want to open this?" dialog. This
rem  file does exactly one thing: hand the job to bash from Git for Windows,
rem  which runs scripts/dev.sh. The real logic lives there, in one place.
rem
rem  Usage:
rem    double click     start all three
rem    dev.cmd ai       from a command prompt, pick services
rem                     (ai / backend / frontend / all)
rem
rem  NOTE about Ctrl+C: cmd.exe adds its own "Terminate batch job (Y/N)?"
rem  prompt on top of everything. Answer N -- that lets the cleanup inside
rem  dev.sh finish. Answering Y kills this .cmd on the spot and can leave
rem  processes still holding the ports. That prompt is cmd.exe's, not ours.
rem
rem  This file is deliberately ASCII-only. Non-ASCII text in a .cmd is
rem  unreliable: cmd.exe reads the file line by line using the current code
rem  page, and multi-byte characters can eat the line terminator, after which
rem  the parser executes the remains of the line as garbage commands. The
rem  Chinese documentation lives in scripts/dev.sh, which has no such problem.
rem ===========================================================================

rem  Switch the console to UTF-8, otherwise the Chinese output of dev.sh is
rem  mojibake in this window. cmd's own localized messages (the pause hint)
rem  may then look garbled -- every message this file prints itself is
rem  English, and pause is silenced below, so that costs nothing.
chcp 65001 >nul 2>&1

rem  Find bash.exe. Known install locations first, PATH only as a fallback:
rem  on a machine with WSL installed, C:\Windows\System32\bash.exe comes first
rem  on PATH, and that is WSL's bash, not Git's -- handing the job to it fails
rem  in confusing ways.
set "BASH="
if exist "%ProgramFiles%\Git\bin\bash.exe" set "BASH=%ProgramFiles%\Git\bin\bash.exe"
if not defined BASH if exist "%ProgramFiles(x86)%\Git\bin\bash.exe" set "BASH=%ProgramFiles(x86)%\Git\bin\bash.exe"
if not defined BASH if exist "%LOCALAPPDATA%\Programs\Git\bin\bash.exe" set "BASH=%LOCALAPPDATA%\Programs\Git\bin\bash.exe"
if not defined BASH for /f "delims=" %%i in ('where bash.exe 2^>nul') do (
  echo %%i | findstr /i /c:"\System32\bash.exe" >nul || if not defined BASH set "BASH=%%i"
)

if not defined BASH (
  echo [x] bash.exe not found.
  echo     This launcher needs Git for Windows installed.
  echo     Or open Git Bash and run:  bash scripts/dev.sh
  echo.
  echo Press any key to close this window...
  pause >nul
  exit /b 1
)

rem  Double clicking sets the working directory to this file's folder, but
rem  running dev.cmd from some other directory does not. cd /d "%~dp0" pins it
rem  to the repository root either way.
cd /d "%~dp0"

echo Using %BASH%
echo.

rem  Pass the relative path scripts/dev.sh rather than %~dp0scripts\dev.sh:
rem  the repository path contains non-ASCII characters, and letting MSYS
rem  convert a full Windows path is an unnecessary risk. The cd above already
rem  put us where the relative path resolves.
"%BASH%" scripts/dev.sh %*
set "RC=%ERRORLEVEL%"

echo.
if not "%RC%"=="0" echo [x] exit code %RC%
echo All services stopped.
echo Press any key to close this window...
pause >nul
