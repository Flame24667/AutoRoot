$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$electronPath = Join-Path $projectRoot 'node_modules\electron\dist\electron.exe'
if (!(Test-Path -LiteralPath $electronPath)) { throw 'Electron missing. Install project dependencies first.' }
if (!(Test-Path -LiteralPath (Join-Path $projectRoot 'bin\myapp-go.automation.exe'))) { throw 'Build the current backend as bin\myapp-go.automation.exe first.' }
& (Join-Path $PSScriptRoot 'build-poc.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
$previousStatic = $env:AUTOROOT_STATIC_UI
try {
    $env:AUTOROOT_STATIC_UI = '1'
    Push-Location $projectRoot
    # User deliberately launches this interactive window. No Vite server is required.
    & $electronPath $projectRoot
} finally {
    Pop-Location
    $env:AUTOROOT_STATIC_UI = $previousStatic
}
