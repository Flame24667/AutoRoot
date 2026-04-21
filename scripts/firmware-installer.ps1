param($InstallDir)

$LogPath = Join-Path $env:APPDATA "AutoRoot\firmware-install.log"
$FwDir = Join-Path $env:APPDATA "AutoRoot\firmware"
New-Item -ItemType Directory -Force -Path (Split-Path $LogPath -Parent) | Out-Null

function Log($msg) {
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Add-Content -Path $LogPath -Value "$ts | $msg" -Encoding UTF8
    Write-Host $msg
}

function Download-GoogleDriveFile {
    param($Url, $Dest)
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
    $wc.DownloadFile($Url, $Dest)
    
    # Handle Google Drive Warning Page
    $Content = Get-Content $Dest -Raw -Encoding UTF8
    if ($Content -match 'confirm=([^&]+)&uuid=([^&"]+)') {
        Log "  🔄 Google Drive warning detected. Bypassing..."
        $Confirm = $matches[1]
        $Uuid = $matches[2]
        $FileId = if ($Url -match 'id=([-\w]+)') { $matches[1] } else { '' }
        $ConfirmUrl = "https://drive.google.com/uc?export=download&confirm=$Confirm&uuid=$Uuid&id=$FileId"
        $wc.DownloadFile($ConfirmUrl, $Dest)
    }
    $wc.Dispose()
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
            Download-GoogleDriveFile -Url $Url -Dest $ZipPath
            
            # Validate file size & type
            $Size = (Get-Item $ZipPath).Length
            if ($Size -lt 1MB) {
                # ✅ FIXED: Removed -TotalCount which caused the crash
                $Content = Get-Content $ZipPath -Raw -Encoding UTF8
                if ($Content -match "<!DOCTYPE|<html") {
                    Log "❌ Downloaded HTML page instead of file (Link likely expired or requires manual confirmation)."
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
            
            foreach ($file in $Files) {
                $Target = Join-Path $FwDir $file.Name
                if (Test-Path $Target) { $Target = Join-Path $FwDir "$($file.BaseName)_$(Get-Random)$($file.Extension)" }
                Copy-Item -Path $file.FullName -Destination $Target -Force -ErrorAction SilentlyContinue
                Log "  + Saved: $($file.Name)"
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