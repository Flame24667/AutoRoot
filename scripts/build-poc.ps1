$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$compilerPath = Join-Path $projectRoot 'frontend\node_modules\@esbuild\win32-x64\esbuild.exe'
if (!(Test-Path -LiteralPath $compilerPath)) { throw 'Frontend dependencies missing. Run npm ci in frontend first.' }
$outputDir = Join-Path $projectRoot 'work-cache\poc-ui'
New-Item -ItemType Directory -Force -Path (Join-Path $outputDir 'assets') | Out-Null
# Run the native compiler directly; this avoids Node child-process pipe restrictions.
# Automatic JSX runtime is essential because App.jsx does not import React's default export.
& $compilerPath (Join-Path $projectRoot 'frontend\src\main.jsx') --bundle --minify --jsx=automatic --format=esm --platform=browser "--outfile=$(Join-Path $outputDir 'assets\poc.js')"
if ($LASTEXITCODE -ne 0) { throw 'PoC frontend compilation failed.' }
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'poc-index.html') -Destination (Join-Path $outputDir 'index.html') -Force
Write-Output 'Static PoC frontend built. No phone commands were executed.'
