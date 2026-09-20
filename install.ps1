[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Root = $PSScriptRoot
$Compose = Join-Path $Root 'deploy/docker-compose.yml'

function Confirm-Removal([string]$Prompt) {
    return (Read-Host "$Prompt [i/N]") -match '^(i|igen|y|yes)$'
}

function Remove-PanelFiles {
    if ((Get-Command docker -ErrorAction SilentlyContinue) -and (Confirm-Removal 'Leállítod és eltávolítod a PufferPanel konténert?')) { & docker compose -f $Compose down --remove-orphans }
    if ((Get-Command docker -ErrorAction SilentlyContinue) -and (Confirm-Removal 'Törlöd a pufferpanel-custom:latest Docker image-et?')) { & docker image rm pufferpanel-custom:latest }
    foreach ($Item in @(@('konfiguráció', 'data/config'), @('szerveradatok és mentések', 'data/data'), @('naplók', 'data/logs'))) {
        $Target = Join-Path $Root $Item[1]
        if ((Test-Path -LiteralPath $Target) -and (Confirm-Removal "Törlöd ezt: $($Item[0]) ($Target)?")) { Remove-Item -LiteralPath $Target -Recurse -Force }
    }
    if (Confirm-Removal 'Törlöd a panel programfájljait? A meg nem erősített data elemek megmaradnak.') {
        Get-ChildItem -LiteralPath $Root -Force | Where-Object Name -ne 'data' | Remove-Item -Recurse -Force
    }
}

Write-Host '1) PufferPanel eltávolítása'
Write-Host '2) PufferPanel telepítése vagy frissítése'
$InstallAction = Read-Host 'Választás'
if ($InstallAction -eq '1') { Remove-PanelFiles; exit 0 }
if ($InstallAction -ne '2') { throw 'Érvénytelen választás.' }

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
