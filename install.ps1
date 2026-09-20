[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Root = $PSScriptRoot
$Compose = Join-Path $Root 'deploy/docker-compose.yml'

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker Desktop is required. Install and start Docker Desktop, then run this script again.'
}

& docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw 'Docker Compose v2 is required.'
}

foreach ($Directory in @('data/config', 'data/data', 'data/logs')) {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $Directory) | Out-Null
}

$Config = Join-Path $Root 'data/config/config.json'
if (-not (Test-Path $Config)) {
    Copy-Item (Join-Path $Root 'config.docker.json') $Config
}

& docker compose -f $Compose up -d --build
if ($LASTEXITCODE -ne 0) { throw 'PufferPanel could not be started. Check Docker Desktop and try again.' }
Write-Host 'PufferPanel is available at http://localhost:8080'
