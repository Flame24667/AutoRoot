$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$compiler = Join-Path $projectRoot 'frontend\node_modules\@esbuild\win32-x64\esbuild.exe'
$output = Join-Path $projectRoot 'work-cache\workbench-tests.cjs'
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $output) | Out-Null
& $compiler (Join-Path $projectRoot 'frontend\src\workbench-view.test.jsx') --bundle --jsx=automatic --platform=node --format=cjs "--outfile=$output"
if ($LASTEXITCODE -ne 0) { throw 'UI test compilation failed.' }
& node --test --test-isolation=none $output (Join-Path $projectRoot 'frontend\src\workflow-view.test.js')
if ($LASTEXITCODE -ne 0) { throw 'UI safety tests failed.' }
