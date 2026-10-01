<#
Installs a caller-supplied trusted samloader-rs 2.2.0 Windows executable.
No network access. The digest is of the executable, not a release ZIP.
#>
[CmdletBinding()]
param([string]$SourcePath, [switch]$PinOnly)
$ErrorActionPreference = 'Stop'
$engineVersion = '2.2.0'
$engineHash = 'b83b8244ecc86ecb4f5efc08e1604214d869838ca59b329e02a34e70832d175b'
if ($PinOnly) {
    Write-Output "Version: $engineVersion"
    Write-Output "Executable SHA-256: $engineHash"
    return
}
if (!$SourcePath) { throw 'Supply -SourcePath to trusted samloader.exe from the official 2.2.0 release. No download is performed.' }
$source = (Resolve-Path -LiteralPath $SourcePath).Path
if ((Get-Item -LiteralPath $source).PSIsContainer) { throw 'Source must be an executable file.' }
$actual = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $engineHash) { throw 'Executable checksum mismatch. Nothing was installed.' }
$projectRoot = Split-Path -Parent $PSScriptRoot
$toolsPath = Join-Path $projectRoot 'tools'
$destination = Join-Path $toolsPath 'samloader.exe'
New-Item -ItemType Directory -Force -Path $toolsPath | Out-Null
if ($source -ne $destination) {
    if (Test-Path -LiteralPath $destination) {
        $oldHash = (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($oldHash -ne $engineHash) { throw 'A different engine exists. Preserve/inspect it manually before replacement.' }
    }
    Copy-Item -LiteralPath $source -Destination $destination -Force
}
Write-Output "Verified samloader $engineVersion at $destination"
