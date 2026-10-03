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

$PanelConfig = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
$Repo = $env:PANEL_UPDATE_REPO
if ([string]::IsNullOrWhiteSpace($Repo)) { $Repo = [string]$PanelConfig.panel.update.repo }
if ([string]::IsNullOrWhiteSpace($Repo)) { $Repo = 'Alex11e/pufferpanel' }
if ($Repo -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') { throw "Invalid update repository: $Repo" }

& docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker Compose v2 is required.' }
& docker info | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop is not running.' }

$Version = $Version.Trim()
$CommitSha = ''
if ($Version -match '^commit:([0-9a-fA-F]{7,40})$') {
    $CommitRef = $Matches[1]
    $Commit = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/commits/$CommitRef" -Headers $Headers -TimeoutSec 20
    $CommitSha = [string]$Commit.sha
    if ($CommitSha -notmatch '^[0-9a-fA-F]{40}$') { throw 'GitHub returned an invalid commit hash.' }
    $Tag = $CommitSha
    $BuildVersion = 'commit-' + $CommitSha.Substring(0, 7)
} else {
    if ($Version -ieq 'latest') {
        $Commit = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/commits?per_page=1" -Headers $Headers -TimeoutSec 20 | Select-Object -First 1
        $CommitSha = [string]$Commit.sha
        if ($CommitSha -notmatch '^[0-9a-fA-F]{40}$') { throw 'GitHub returned an invalid latest commit hash.' }
        $Tag = $CommitSha
        $BuildVersion = 'commit-' + $CommitSha.Substring(0, 7)
    } else {
        if ($Version -notmatch '^v?\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$') {
            throw 'Invalid version. Use latest, commit:<SHA>, or a release tag such as v3.0.0.'
        }
        if (-not $Version.StartsWith('v')) { $Version = "v$Version" }
        $ReleaseUri = "https://api.github.com/repos/$Repo/releases/tags/$([uri]::EscapeDataString($Version))"
        $Release = Invoke-RestMethod -Uri $ReleaseUri -Headers $Headers -TimeoutSec 20
        $Tag = [string]$Release.tag_name
        if ($Tag -notmatch '^v?\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$') { throw "GitHub returned an invalid release tag: $Tag" }
        $Commit = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/commits/$([uri]::EscapeDataString($Tag))" -Headers $Headers -TimeoutSec 20
        $CommitSha = [string]$Commit.sha
        if ($CommitSha -notmatch '^[0-9a-fA-F]{40}$') { throw 'GitHub returned an invalid release commit hash.' }
        $BuildVersion = $Tag
    }
}

$Stage = Join-Path $env:TEMP ("pufferpanel-update-" + [guid]::NewGuid().ToString('N'))
$Archive = Join-Path $Stage 'source.zip'
$Source = Join-Path $Stage 'source'
$ArchiveUri = "https://github.com/$Repo/archive/$([uri]::EscapeDataString($Tag)).zip"

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
    $env:PANEL_VERSION = $BuildVersion
    $env:PANEL_SHA = $CommitSha
    Write-Host "Building and updating $Repo to $BuildVersion ($($CommitSha.Substring(0, 7)))..."
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