# AutoRoot

Guarded desktop automation for a Samsung Magisk initial-root workflow.
Electron + React frontend, Go backend, pinned samloader-rs engine.

**Scope:** Windows x64, SM-A065F, XID, PDA A065FXXS4AYE2,
OMC A065FOLE4AYE2, binary 4, English Magisk 30.7, Ramdisk Yes.
Other devices/builds are not claimed as supported. SM-A045F is deferred.

The operator-led test on **1 October 2026** reached normal Android Home and
returned `uid=0(root)` through the app. It included an engine launch failure
and explicit recovery; it was not a failure-free or unattended run.

- [Operator test plan](docs/POC-TEST-PLAN.md)
- [Test results and known issues](docs/TEST-RESULTS-2026-10-01.md)
- [Git handoff checklist](docs/GIT-HANDOFF.md)

## Safety and limits

Initial-root deletes user data. Bootloader unlocking, recovery reset, Android
setup, USB authorization and Superuser consent remain operator checkpoints.
Do not relock with modified partitions. Returning to stock does not reset Knox.
See the [Magisk Samsung instructions](https://topjohnwu.github.io/Magisk/install.html#samsung-devices).

Database selection uses an operator-supplied **local stock ZIP**. Automatic
network download is not tested. Firmware Manager and additional devices are
future work, not current features. ZIP CRC, embedded MD5 and SHA-256 comparisons
check integrity; they do not independently authenticate a Samsung signature.

A saved stage is historical evidence. The UI shows ROOTED: YES only with live,
matching `uid=0` evidence. Refresh connection after USB changes; continuous device
monitoring is not implemented. Engine exit success alone is not root success.

## Requirements

- Windows x64; Node.js/npm and Go matching `backend/go.mod` (1.26.2).
- Android SDK Platform Tools: `adb` on PATH; Samsung USB driver.
- Stock firmware for the exact supported configuration.
- English Magisk 30.7 APK placed as `Magisk-v30.7.apk` at the project root.
- Trusted Windows samloader-rs 2.2.0 executable in `tools/samloader.exe`.

Firmware/AP files are several GB. Choose a volume with sufficient headroom,
keep USB stable and prevent laptop sleep during an approved flash. Firmware,
engine binaries, runtime state and logs are not source-code deliverables.

## Setup and run (source PoC, not a packaged installer)

From the project root in PowerShell:

```powershell
npm ci
npm --prefix frontend ci
New-Item -ItemType Directory -Force bin
Push-Location backend
go build -o ..\bin\myapp-go.automation.exe .
Pop-Location
powershell -NoProfile -File scripts\start-poc.ps1
```

Review required dependency install scripts if npm asks for approval; do not
disable system security to make installation work. Close the old app before
rebuilding/replacing its backend.

The launcher builds a static frontend into ignored `work-cache/poc-ui` using
native esbuild and opens Electron. It avoids the observed Node subprocess-pipe
EPERM in the managed environment. The normal Vite build and installer packaging
have not passed that environment's release checks. Clean-machine installation
still needs a separate test.

### Engine provisioning

Review the [official pinned release](https://github.com/topjohnwu/samloader-rs/releases/tag/2.2.0),
then supply a trusted extracted executable:

```powershell
powershell -NoProfile -File scripts\fetch-samloader.ps1 -SourcePath "D:\TrustedTools\samloader.exe"
```

This script does not download anything. The executable must match SHA-256
`b83b8244ecc86ecb4f5efc08e1604214d869838ca59b329e02a34e70832d175b`.
A version string alone is not trusted. Changing the pin requires separate review
and testing. This engine can flash an archive-supplied PIT; do not assume that
the absence of a repartition flag makes it partition-table-neutral.

## Local firmware configuration

Public metadata lives in `setup/firmware-db.json`. Its relative localPath points
to `firmware/SM-A065F/<stock-package>.zip` in the project. No firmware is shipped.

For machine-specific paths, copy the metadata to
`setup/firmware-db.local.json` and edit localPath. That file is ignored and takes
precedence. `AUTOROOT_DATABASE` explicitly selects a different catalog.
A relative path in a setup/ catalog resolves against its project; a catalog
outside setup/ resolves relative to the catalog directory.

Large output files default to project `firmware/`; select a suitable volume via:

```powershell
$env:AUTOROOT_FIRMWARE_ROOT = "D:\AutoRootData\firmware"
```

The local override preserves this lab's existing firmware location; it is not
included in the public metadata. Session/evidence directories are under the
running backend's `state/` directory (normally `bin/state`).

## App workflow

1. Authorize the phone and refresh connection. After a separately approved full
   stock restore, choose **Start a new PoC session**; the old session is archived.
2. Preparation: **Select & validate from database** (or choose a local stock ZIP).
3. **Install Magisk & transfer AP**, then **Patch & collect automatically**.
   Keep the screen unlocked. Unknown screens stop the adapter rather than guessing.
4. Confirm a **Read-only engine probe**; pair the displayed identities only for
   the intended phone. Restart manually after PIT reading and reauthorize ADB.
5. Flash: **Refresh preflight**, require PASSED, **Approve plan**, **Show command**.
6. Give separate final consent with **Flash & wipe data**. Keep the app/USB/laptop
   running; do not retry automatically or terminate an active flash.
7. Complete Recovery reset/setup if requested, authorize ADB, finish Magisk
   additional setup/reboot, and **Verify root**. Success requires `uid=0(root)`.

Use **Checks & logs** for status and diagnostics. Transfer/remote hashing may
show 0% despite activity; that is not proof of failure. Progress is not a whole-run
or flash-transfer percentage. Preserve logs and inspect before retrying.

A proven engine process-start failure has a separate explicit recovery action.
It rechecks the phone/files, archives failure evidence, clears approval and
returns to patched-ap without reboot/flash. Started or uncertain flashes cannot
use it. The observed Access is denied cause remains unproven despite a later
successful launch and flash.

## Local verification (no phone operations)

```powershell
powershell -NoProfile -File scripts\test-ui.ps1
Push-Location backend
go test -count=1 ./...
go vet ./...
Pop-Location
powershell -NoProfile -File scripts\build-poc.ps1
node --check electron/main.js
node --check electron/preload.js
```

Tests cover firmware integrity/anti-rollback, downloader fixtures, staged
approval, root proof, UI guards, screen-awake mask 15, diagnostics and pre-launch
recovery. They do not establish universal device compatibility or network-source
trust. Legacy scripts and package commands are not the tested PoC launch path.

## Publication

Do not publish runtime evidence with device identifiers or local machine paths.
Stage explicit source/docs files; do not blindly use git add .
Some binary/build assets were already tracked upstream; ignore rules do not
untrack them or remove repository history. Review those separately before a
public push. See the Git handoff checklist. No commit/push is part of cleanup.
