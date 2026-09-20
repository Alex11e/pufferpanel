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

function New-PanelUser {
    if ((& docker inspect --format '{{.State.Running}}' pufferpanel 2>$null) -ne 'true') { throw 'A PufferPanel konténer nem fut. Előbb válaszd a 2-es telepítés/frissítés opciót.' }
    if (-not (Confirm-Removal 'Létrehozol most egy PufferPanel felhasználót?')) { return }
    $Username = Read-Host 'Felhasználónév'
    $Email = Read-Host 'E-mail cím'
    $Password = Read-Host 'Jelszó' -AsSecureString
    $Confirm = Read-Host 'Jelszó újra' -AsSecureString
    $PasswordText = [System.Net.NetworkCredential]::new('', $Password).Password
    $ConfirmText = [System.Net.NetworkCredential]::new('', $Confirm).Password
    if ([string]::IsNullOrWhiteSpace($Username) -or [string]::IsNullOrWhiteSpace($Email) -or [string]::IsNullOrWhiteSpace($PasswordText)) { throw 'A felhasználónév, e-mail cím és jelszó kötelező.' }
    if ($PasswordText -cne $ConfirmText) { throw 'A két jelszó nem egyezik.' }
    Write-Host '1) Normál felhasználó'
    Write-Host '2) Adminisztrátor'
    $Role = Read-Host 'Szerepkör'
    $Arguments = @('exec', 'pufferpanel', '/pufferpanel/bin/pufferpanel', 'user', 'add', '--name', $Username, '--email', $Email, '--password', $PasswordText)
    if ($Role -eq '2') { $Arguments += '--admin' }
    elseif ($Role -ne '1') { throw 'Érvénytelen szerepkör.' }
    & docker @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'A felhasználó létrehozása sikertelen.' }
    Write-Host "Felhasználó létrehozva: $Username"
}

Write-Host '1) PufferPanel eltávolítása'
Write-Host '2) PufferPanel telepítése vagy frissítése'
Write-Host '3) Felhasználó létrehozása'
$InstallAction = Read-Host 'Választás'
if ($InstallAction -eq '1') { Remove-PanelFiles; exit 0 }
if ($InstallAction -eq '3') { New-PanelUser; exit 0 }
if ($InstallAction -ne '2') { throw 'Érvénytelen választás.' }

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker Desktop is required. Install and start Docker Desktop, then run this script again.'
}

& docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw 'Docker Compose v2 is required.'
}

foreach ($Directory in @('data/config', 'data/data/backups', 'data/data/servers', 'data/data/binaries', 'data/data/cache', 'data/logs')) {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $Directory) | Out-Null
}

$Config = Join-Path $Root 'data/config/config.json'
if (-not (Test-Path $Config)) {
    Copy-Item (Join-Path $Root 'config.docker.json') $Config
}

& docker compose -f $Compose up -d --build --wait --wait-timeout 120
if ($LASTEXITCODE -ne 0) {
    & docker compose -f $Compose ps
    & docker compose -f $Compose logs --tail 200 pufferpanel
    throw 'A PufferPanel nem indult el. A fenti Docker naplóban látható a hiba.'
}
New-PanelUser
Write-Host 'PufferPanel is available at http://localhost:8080'
