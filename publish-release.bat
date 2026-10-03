@echo off
setlocal

set "RELEASE_VERSION=%~1"
if /i "%RELEASE_VERSION%"=="/?" goto :help

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0publish-release.ps1" -Version "%RELEASE_VERSION%" -AssetDirectory "%~dp0release-assets"
exit /b %ERRORLEVEL%

:help
echo Usage: publish-release.bat vX.Y.Z
echo Put the prepared files to upload in the release-assets folder.
echo GitHub CLI (gh) login and a pushed Git tag are required.
exit /b 0