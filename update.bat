@echo off
setlocal

set "UPDATE_VERSION=%~1"
if /i "%UPDATE_VERSION%"=="/?" goto :help
if not defined UPDATE_VERSION set "UPDATE_VERSION=%PANEL_UPDATE_VERSION%"
if not defined UPDATE_VERSION set "UPDATE_VERSION=latest"

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0update.ps1" -Version "%UPDATE_VERSION%"
exit /b %ERRORLEVEL%

:help
echo Usage: update.bat [latest^|vX.Y.Z]
echo Examples:
echo   update.bat
echo   update.bat v3.0.0
exit /b 0