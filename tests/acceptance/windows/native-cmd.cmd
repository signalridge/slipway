@echo off
setlocal DisableDelayedExpansion

if "%~1"=="" (
  echo native Windows acceptance ^(Cmd^) failed: usage: %~nx0 C:\path\to\slipway.exe 1>&2
  exit /b 2
)

set "SLIPWAY_EXE=%~f1"
if not exist "%SLIPWAY_EXE%" (
  echo native Windows acceptance ^(Cmd^) failed: binary not found: %SLIPWAY_EXE% 1>&2
  exit /b 2
)

where git.exe >nul 2>nul
if errorlevel 1 (
  echo native Windows acceptance ^(Cmd^) failed: git.exe is required 1>&2
  exit /b 2
)
where powershell.exe >nul 2>nul
if errorlevel 1 (
  echo native Windows acceptance ^(Cmd^) failed: powershell.exe is required for JSON assertions and safe encoded argv 1>&2
  exit /b 2
)

rem The PowerShell file is the shared JSON assertion driver. In Cmd mode it
rem resolves structured Next argv, executes special values through cmd.exe
rem /d /v:on, and executes the binary's own rendered recovery command there.
powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%~dp0native-powershell.ps1" -SlipwayExe "%SLIPWAY_EXE%" -Mode Cmd
if errorlevel 1 exit /b %errorlevel%

exit /b 0
