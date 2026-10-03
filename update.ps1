[CmdletBinding()]
param([string]$Version = 'latest')

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Version)) { $Version = $env:PANEL_UPDATE_VERSION }
if ([string]::IsNullOrWhiteSpace($Version)) { $Version = 'latest' }
$Root = $PSScriptRoot
$ConfigPath = Join-Path $Root 'data/config/config.json'
$ComposeRelativePath = 'deploy/docker-compose.yml'
$Headers = @{
    Accept = 'application/vnd.github+json'
    'User-Agent' = 'PufferPanel-Windows-Updater'
}

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker Desktop not found. Install and start Docker Desktop, then retry.'
}
if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "Existing config not found: $ConfigPath. Run install.ps1 first."
}

& docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker Compose v2 is required.' }
& docker info | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop is not running.' }

$Version = $Version.Trim()
if ($Version -ieq 'latest') {
    $ReleaseUri = 'https://api.github.com/repos/pufferpanel/pufferpanel/releases/latest'
} else {
    if ($Version -notmatch '^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
        throw 'Invalid version. Example: v3.0.0 or v3.1.0-rc.1.'
    }
    if (-not $Version.StartsWith('v')) { $Version = "v$Version" }
    $ReleaseUri = "https://api.github.com/repos/pufferpanel/pufferpanel/releases/tags/$([uri]::EscapeDataString($Version))"
}

$Release = Invoke-RestMethod -Uri $ReleaseUri -Headers $Headers -TimeoutSec 20
$Tag = [string]$Release.tag_name
if ($Tag -notmatch '^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
    throw "GitHub returned an invalid release tag: $Tag"
}

$Stage = Join-Path $env:TEMP ("pufferpanel-update-" + [guid]::NewGuid().ToString('N'))
$Archive = Join-Path $Stage 'source.zip'
$Source = Join-Path $Stage 'source'
$ArchiveUri = "https://github.com/pufferpanel/pufferpanel/archive/refs/tags/$([uri]::EscapeDataString($Tag)).zip"

try {
    New-Item -ItemType Directory -Force -Path $Source | Out-Null
    Write-Host "Downloading PufferPanel $Tag..."
    Invoke-WebRequest -Uri $ArchiveUri -Headers $Headers -OutFile $Archive -TimeoutSec 120
    Expand-Archive -LiteralPath $Archive -DestinationPath $Source -Force

    $Checkout = Get-ChildItem -LiteralPath $Source -Directory | Select-Object -First 1
    if (-not $Checkout -or -not (Test-Path (Join-Path $Checkout.FullName $ComposeRelativePath))) {
		throw 'The release archive is missing the Docker Compose files.'
    }

    $env:PANEL_CONFIG_DIR = [System.IO.Path]::GetFullPath((Join-Path $Root 'data/config')).Replace([char]92, [char]47)
    $env:PANEL_DATA_DIR = [System.IO.Path]::GetFullPath((Join-Path $Root 'data/data')).Replace([char]92, [char]47)
    $env:PANEL_LOGS_DIR = [System.IO.Path]::GetFullPath((Join-Path $Root 'data/logs')).Replace([char]92, [char]47)
    $ComposeFile = Join-Path $Checkout.FullName $ComposeRelativePath
    Write-Host "Building and updating the Docker image to $Tag..."
    & docker compose --project-name deploy --file $ComposeFile up --detach --build --wait --wait-timeout 120
    if ($LASTEXITCODE -ne 0) {
        & docker compose --project-name deploy --file $ComposeFile logs --tail 200 pufferpanel
        throw 'The update failed. See the Docker output above for details.'
    }

    Write-Host "Update complete: $Tag"
    Write-Host 'The data/config, data/data, and data/logs folders were preserved.'
} finally {
    Remove-Item -LiteralPath $Stage -Recurse -Force -ErrorAction SilentlyContinue
}