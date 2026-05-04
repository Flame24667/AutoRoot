param($InstallDir)

$LogPath = Join-Path $env:APPDATA "AutoRoot\firmware-install.log"
$FwDir = Join-Path $env:APPDATA "AutoRoot\firmware"
New-Item -ItemType Directory -Force -Path (Split-Path $LogPath -Parent) | Out-Null

function Log($msg) {
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Add-Content -Path $LogPath -Value "$ts | $msg" -Encoding UTF8
    Write-Host $msg
}

function Download-GDriveLargeFile {
    param($Url, $Dest)
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
    try {
        $page = $wc.DownloadString($Url)
        $match = [regex]::Match($page, 'id="download-form".*?action="([^"]+)"', [System.Text.RegularExpressions.RegexOptions]::Singleline)
        if ($match.Success) {
            $confirmUrl = $match.Groups[1].Value -replace '&amp;', '&'
            Log "  🔄 Large file detected. Confirming download..."
            $wc.DownloadFile($confirmUrl, $Dest)
        } else {
            $wc.DownloadFile($Url, $Dest)
        }
    } finally {
        $wc.Dispose()
    }
}

Log "=== FIRMWARE INSTALL STARTED ==="
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$DbPath = Join-Path $InstallDir "resources\setup\firmware-db.json"
if (-Not (Test-Path $DbPath)) { Log "ERROR: DB not found"; exit 1 }

$Db = Get-Content $DbPath -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Force -Path $FwDir | Out-Null

$Ok = 0; $Skip = 0; $Fail = 0

foreach ($dev in $Db.devices) {
    foreach ($fw in $dev.firmware) {
        $ZipPath = Join-Path $FwDir $fw.filename
        if (Test-Path $ZipPath) { Log "SKIP: $($fw.filename)"; $Skip++; continue }

        $Url = $fw.url
        if ($Url -match "drive\.google\.com.*?id=([-\w]+)") {
            $Url = "https://drive.google.com/uc?export=download&id=$($matches[1])"
        }

        Log "DOWNLOAD: $($fw.filename)"
        try {
            Download-GDriveLargeFile -Url $Url -Dest $ZipPath

            if (-Not (Test-Path $ZipPath)) { throw "File not created" }
            $Size = (Get-Item $ZipPath).Length
            if ($Size -lt 1MB) {
                $Content = Get-Content $ZipPath -Raw -Encoding UTF8
                if ($Content -match "<!DOCTYPE|<html") {
                    Log "❌ Downloaded HTML page. Link may require manual browser confirmation."
                    Remove-Item $ZipPath -Force -ErrorAction SilentlyContinue
                    $Fail++; continue
                }
            }

            Log "✅ Downloaded ($([math]::Round($Size/1MB, 2)) MB). Extracting..."

            $TempDir = Join-Path $env:TEMP "AutoRoot_$(Get-Random)"
            New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
            Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force -ErrorAction Stop

            $Files = Get-ChildItem -Path $TempDir -Recurse -File
            Log "  Found $($Files.Count) files"

            # 🔑 STANDARDIZED RENAMING: [model]_[androidVersion]_firmware[ext]
            $baseName = "$($dev.model)_$($fw.androidVersion)_firmware"
            $counter = 1

            foreach ($file in $Files) {
                $newName = "$baseName$($file.Extension)"
                $Target = Join-Path $FwDir $newName

                # Handle duplicates by appending _1, _2, etc.
                while (Test-Path $Target) {
                    $newName = "$baseName`_$counter$($file.Extension)"
                    $Target = Join-Path $FwDir $newName
                    $counter++
                }

                Copy-Item -Path $file.FullName -Destination $Target -Force -ErrorAction SilentlyContinue
                Log "  + Saved: $newName"
            }

            Remove-Item -Path $ZipPath -Force -ErrorAction SilentlyContinue
            Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
            Log "✅ Zip cleaned up."
            $Ok++

        } catch {
            Log "❌ Error: $($_.Exception.Message)"
            $Fail++
        }
    }
}

Log "=== SUMMARY ==="
Log "Success: $Ok | Skipped: $Skip | Failed: $Fail"
Get-ChildItem -Path $FwDir -File | ForEach-Object { Log "  - $($_.Name) ($([math]::Round($_.Length/1MB, 2)) MB)" }
exit 0