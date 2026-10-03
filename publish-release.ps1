[CmdletBinding()]
param(
    [string]$Version,
    [Parameter(Mandatory = $true)][string]$AssetDirectory
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = Read-Host 'Release version (example: v3.1.0)'
}
$Version = $Version.Trim()
if ($Version -notmatch '^v?\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$') {
    throw 'Invalid version. Example: v3.1.0 or v4.0.0-rc.1.'
}
if (-not $Version.StartsWith('v')) { $Version = "v$Version" }

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw 'GitHub CLI (gh) is required. Install it from https://cli.github.com/.'
}
& gh auth status
if ($LASTEXITCODE -ne 0) { throw 'Run gh auth login before publishing a release.' }

if (-not (Test-Path -LiteralPath $AssetDirectory)) {
    New-Item -ItemType Directory -Force -Path $AssetDirectory | Out-Null
}
$Assets = @(Get-ChildItem -LiteralPath $AssetDirectory -File | Select-Object -ExpandProperty FullName)
if ($Assets.Count -eq 0) {
    throw "No files to upload. Put prepared release files in: $AssetDirectory"
}

$null = & gh release view $Version --json url 2>$null
if ($LASTEXITCODE -eq 0) {
    Write-Host "Release $Version already exists; uploading new assets without replacing existing files."
    & gh release upload $Version @Assets
} else {
    Write-Host "Creating release $Version and uploading $($Assets.Count) file(s)."
    & gh release create $Version @Assets --verify-tag --title "PufferPanel $Version" --generate-notes
}
if ($LASTEXITCODE -ne 0) {
    throw 'GitHub release upload failed. Check the tag, asset names, and gh permissions.'
}

Write-Host "Release published: $Version"