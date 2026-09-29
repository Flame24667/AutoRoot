<#
.SYNOPSIS
    Downloads and verifies the samloader-rs flashing engine.

.DESCRIPTION
    samloader-rs (github.com/topjohnwu/samloader-rs) is the flashing engine used
    by AutoRoot. It is the only Samsung flashing tool here with a real, documented
    command line: Odin is a GUI with no supported CLI, and the Odin3.exe in the
    archive is a 132-byte Git LFS pointer rather than a program.

    The binary is pinned by SHA-256. The script refuses to install anything whose
    hash does not match the expected value, and it records what it installed so
    backend/samloader.go can be pinned to the same version.

.PARAMETER Version
    Release tag to install. Defaults to the version pinned in the backend.

.PARAMETER PinOnly
    Print the expected SHA-256 for the pinned version and exit. Useful for
    reviewing the pin before installing anything.

.EXAMPLE
    .\fetch-samloader.ps1 -PinOnly
#>
[CmdletBinding()]
param(
    [string]$Version = '0.6.0',
    [string]$ExpectedSHA256 = '',
    [switch]$PinOnly
)

$ErrorActionPreference = 'Stop'

$ToolsDir = Join-Path $PSScriptRoot '..\tools'
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')

function Get-ReleaseAsset {
    param([string]$Tag)

    $api = "https://api.github.com/repos/topjohnwu/samloader-rs/releases/tags/v$Tag"
    Write-Host "Querying $api" -ForegroundColor DarkGray

    $release = Invoke-RestMethod -Uri $api -Headers @{
        'User-Agent' = 'AutoRoot-engine-setup'
        'Accept'     = 'application/vnd.github+json'
    }

    $windows = $release.assets | Where-Object {
        $_.name -match 'windows' -and $_.name -match 'x86_64|amd64|x64'
    } | Select-Object -First 1

    if (-not $windows) {
        throw "No Windows x64 asset found in release v$Tag. Assets: $(($release.assets.name) -join ', ')"
    }
    return $windows
}

if ($PinOnly) {
    Write-Host "Pinned engine version: $Version" -ForegroundColor Cyan
    if ($ExpectedSHA256) {
        Write-Host "Expected SHA-256:     $ExpectedSHA256" -ForegroundColor Cyan
    } else {
        Write-Host "No expected SHA-256 was supplied, so the install would be refused." -ForegroundColor Yellow
        Write-Host "Supply -ExpectedSHA256 taken from the official release notes." -ForegroundColor Yellow
    }
    exit 0
}

if (-not $ExpectedSHA256) {
    throw @"
No -ExpectedSHA256 was given.

AutoRoot will not install or run an unverified flashing engine. Obtain the
SHA-256 published with the official release and pass it explicitly:

    .\fetch-samloader.ps1 -Version $Version -ExpectedSHA256 <sha256>
"@
}

$asset = Get-ReleaseAsset -Tag $Version
Write-Host "Asset: $($asset.name) ($([math]::Round($asset.size / 1MB, 2)) MB)" -ForegroundColor Cyan

New-Item -ItemType Directory -Force -Path $ToolsDir | Out-Null

# Download to the D: volume when it is available, so a ~5 MB engine install
# never competes with firmware space. A temporary file is used so a failed
# download cannot be mistaken for a complete one.
$tmpDir = 'D:\Data Kelola IT\Firmware\.cache'
if (-not (Test-Path 'D:\')) { $tmpDir = [System.IO.Path]::GetTempPath() }
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

$tmp = Join-Path $tmpDir "samloader-$Version.zip"
Write-Host "Downloading to $tmp" -ForegroundColor DarkGray
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tmp -UseBasicParsing

$actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLower()
if ($actual -ne $ExpectedSHA256.ToLower()) {
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    throw @"
SHA-256 mismatch. The download was discarded and nothing was installed.

  expected $ExpectedSHA256
  actual   $actual

If this is expected, the pin in backend/samloader.go must be updated to match
the official release before the engine will run.
"@
}

$extractDir = Join-Path $tmpDir "samloader-$Version-extract"
if (Test-Path $extractDir) { Remove-Item $extractDir -Recurse -Force }
Expand-Archive -Path $tmp -DestinationPath $extractDir -Force

$exe = Get-ChildItem -Path $extractDir -Filter 'samloader.exe' -Recurse |
    Select-Object -First 1
if (-not $exe) {
    throw "samloader.exe was not found inside the release archive."
}

$dest = Join-Path $ToolsDir 'samloader.exe'
Copy-Item $exe.FullName $dest -Force
Remove-Item $tmp -Force -ErrorAction SilentlyContinue
Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Installed $Version" -ForegroundColor Green
Write-Host "  path:   $dest"
Write-Host "  sha256: $actual"
Write-Host ""
Write-Host "Record this in backend/samloader.go:" -ForegroundColor Cyan
Write-Host "  const samloaderVersion = `"$Version`""
Write-Host "  // samloaderPins[`"$Version`"] = `"$actual`""
Write-Host ""
Write-Host "Then verify with:" -ForegroundColor Cyan
Write-Host "  .\fetch-samloader.ps1 -PinOnly -Version $Version -ExpectedSHA256 $actual"
