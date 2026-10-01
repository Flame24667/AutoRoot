param([switch]$PrepareFirmware, [switch]$UseExistingSession, [switch]$PreparePatch, [switch]$AutoPatch, [switch]$ProbeEngine, [string]$BindDownloadSerial, [switch]$ConfirmOnlyDevice, [switch]$ExecuteFlash, [switch]$ConfirmWipe)
$ErrorActionPreference = 'Stop'
if ($ExecuteFlash -and (!$ConfirmWipe -or !$UseExistingSession)) { throw 'Flash requires -UseExistingSession -ExecuteFlash -ConfirmWipe and fresh human approval.' }
if ($ExecuteFlash -and ($PrepareFirmware -or $PreparePatch -or $AutoPatch -or $ProbeEngine -or $BindDownloadSerial)) { throw 'Flash cannot be combined with preparation or probe operations.' }
$projectRoot = Split-Path -Parent $PSScriptRoot
$backendPath = Join-Path $projectRoot 'bin\myapp-go.new.exe'
if (Test-Path -LiteralPath (Join-Path $projectRoot 'bin\myapp-go.automation.exe')) { $backendPath = Join-Path $projectRoot 'bin\myapp-go.automation.exe' }
if (!(Test-Path -LiteralPath $backendPath)) { throw 'Build bin/myapp-go.new.exe first' }
$processInfo = [Diagnostics.ProcessStartInfo]::new()
$processInfo.FileName = $backendPath
$processInfo.WorkingDirectory = $projectRoot
$processInfo.UseShellExecute = $false
$processInfo.CreateNoWindow = $true
$processInfo.RedirectStandardInput = $true
$processInfo.RedirectStandardOutput = $true
$processInfo.RedirectStandardError = $true
$backendProcess = [Diagnostics.Process]::new()
$backendProcess.StartInfo = $processInfo
[void]$backendProcess.Start()
$stderrRead = $backendProcess.StandardError.ReadToEndAsync()
function Invoke-AutoRootRequest([string]$Action, [hashtable]$Payload = @{}) {
    $requestId = [Guid]::NewGuid().ToString('N')
    $request = @{id=$requestId; action=$Action; payload=$Payload} | ConvertTo-Json -Compress -Depth 10
    $backendProcess.StandardInput.WriteLine($request)
    $backendProcess.StandardInput.Flush()
    while (!$backendProcess.HasExited) {
        $line = $backendProcess.StandardOutput.ReadLine()
        if (!$line) { continue }
        if (!$line.TrimStart().StartsWith('{')) { continue }
        $reply = $line | ConvertFrom-Json
        if ($reply.id -ne $requestId) { continue }
        if ($reply.error) { throw $reply.error }
        return $reply.result
    }
    throw 'Backend exited before replying'
}
try {
    $session = if ($UseExistingSession) { Invoke-AutoRootRequest 'sessionState' } else { Invoke-AutoRootRequest 'startSession' }
    Write-Output ('SESSION: ' + ($session | ConvertTo-Json -Compress -Depth 8))
    $plan = Invoke-AutoRootRequest 'databasePlan'
    Write-Output ('DATABASE: ' + ($plan | ConvertTo-Json -Compress -Depth 8))
    $operations = @()
    if ($PrepareFirmware) { $operations += 'prepareDatabase' }
    if ($PreparePatch) { $operations += 'preparePatch' }
    if ($AutoPatch) { $operations += 'automateMagiskPatch' }
    if ($ProbeEngine) { $operations += 'probeEngine' }
    if ($BindDownloadSerial) {
        if (!$ConfirmOnlyDevice) { throw 'Binding requires -ConfirmOnlyDevice after explicit operator confirmation.' }
        $operations += 'bindDownloadDevice'
    }
    if ($ExecuteFlash) { $operations += 'approveFlash'; $operations += 'executeFlash' }
    foreach ($operation in $operations) {
        $jobPayload = @{operation=$operation}
        if ($operation -eq 'probeEngine') { $jobPayload.confirmReboot = $true }
        if ($operation -eq 'bindDownloadDevice') { $jobPayload.confirmOnlyDevice = $true; $jobPayload.downloadSerial = $BindDownloadSerial }
        if ($operation -eq 'approveFlash') { $jobPayload.by = 'human explicit flash/wipe consent in current chat' }
        if ($operation -eq 'executeFlash') { $jobPayload.confirmWipe = $true }
        $job = Invoke-AutoRootRequest 'startAutomationJob' $jobPayload
        $lastProgress = ''
        do {
            Start-Sleep -Seconds 3
            $status = Invoke-AutoRootRequest 'automationStatus'
            $progressKey = "$($status.detail)|$([math]::Floor($status.percent / 10))|$($status.status)"
            if ($progressKey -ne $lastProgress) {
                Write-Output ('JOB: ' + ($status | ConvertTo-Json -Compress -Depth 10))
                $lastProgress = $progressKey
            }
        } while ($status.status -eq 'running')
        if ($status.status -ne 'completed' -or $status.result.ok -eq $false) { throw ($status.error + ' ' + ($status.result.errors -join '; ')) }
    }
    if ($ExecuteFlash) {
        Write-Output 'Flash engine completed. Root is NOT verified yet. Restore phone setup and USB debugging, then verify root separately.'
    } elseif ($ProbeEngine -or $BindDownloadSerial) {
        Write-Output 'Read-only Download Mode probe completed; no flash, wipe or root approval. Phone may remain in Download Mode.'
    } elseif ($AutoPatch) {
        Write-Output 'On-phone patch and collection completed. No reboot, flash or root approval performed.'
    } elseif ($PreparePatch) {
        Write-Output 'Patch prepared on phone; select and patch the returned remoteAP in Magisk. No reboot, flash or root approval performed.'
    } else {
        Write-Output 'No APK install, reboot, flash or root approval was performed by this script.'
    }
} finally {
    try { $backendProcess.StandardInput.Close() } catch { Write-Warning 'Backend input was already closed.' }
    [void]$backendProcess.WaitForExit(10000)
    # Never kill the backend if a firmware preparation job is still working.
    if ($backendProcess.HasExited) { $backendProcess.Dispose() }
}
