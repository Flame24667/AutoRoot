# AutoRoot

AutoRoot is an Electron + React application with a Go backend that drives a guarded
Samsung Magisk rooting workflow. It targets the Samsung Galaxy A06 (SM-A065F).

> **Safety status:** the workflow is staged and every destructive step is gated.
> AutoRoot will not flash firmware on its own. It stops before rebooting the phone
> until firmware is validated, the flashing engine is checksum-verified, and you
> have explicitly approved the flash.

## What changed from the original archive

The original project called `Odin3.exe` with flags like `-device:` and `-auto`.
That was never going to work: Odin is a GUI with no supported command line, and
the `odin/Odin3.exe` in the archive is a 132-byte Git LFS pointer, not a program.
AutoRoot now uses **samloader-rs** instead, which has a real, documented CLI.

Other corrections:

- **ADB unauthorized** is now reported precisely instead of a generic failure.
- **Downloads stream to `D:`** with resume, because the system drive was nearly
  full (~10 GB free) and a Samsung package is 5.81 GB.
- **Firmware is validated** against model, sales code, anti-rollback, ZIP
  integrity and every internal `.tar.md5` before it can be used.
- **State is persisted**, so a restart or a dropped USB cable resumes the run
  instead of starting over.
- **Flashing requires explicit approval** that is recorded in the saved session.

## Requirements

- Windows 10/11
- Node.js and npm
- Go 1.26.2 or newer
- Android SDK Platform Tools (`adb`) on `PATH`
- Samsung USB driver
- `samloader` placed in `tools/` (see below)

## Setup

```powershell
npm ci
cd frontend; npm ci; cd ..

cd backend
go build -o ..\bin\myapp-go.exe .
cd ..

npm run dev
```

## Firmware locations

Large files never touch the system drive.

| What | Where |
|---|---|
| Firmware packages | `D:\Data Kelola IT\Firmware\SM-A065F` |
| Extracted slots | `D:\Data Kelola IT\Firmware\SM-A065F\extracted` |
| Session state | `bin\state\session.json` |
| Download resume state | `bin\state\cache\download-state.json` |
| Engine | `tools\samloader.exe` |

Set `AUTOROOT_FIRMWARE_ROOT` to override the firmware root.

## Installing the flashing engine

The engine is already installed and pinned: `tools\samloader.exe`, version 2.2.0,
SHA-256 `b83b8244ecc86ecb4f5efc08e1604214d869838ca59b329e02a34e70832d175b`,
recorded in `backend/samloader.go`. AutoRoot refuses to run any other build.

To reinstall, or to move to a newer release:

```powershell
.\scripts\fetch-samloader.ps1 -Version 2.2.0 -ExpectedSHA256 <sha256-from-official-release>
```

Review a pin before installing anything:

```powershell
.\scripts\fetch-samloader.ps1 -PinOnly -Version 2.2.0 -ExpectedSHA256 <sha256>
```

### What has been verified about the engine

- `samloader --version` reports 2.2.0.
- The documented subcommands are present: `download`, `check-update`, `detect`,
  `dump-pit`, `print-pit`, `flash`, `verify-md5`, `reboot-download`.
- The default USB backend is `vcom`, which on Windows works with the stock
  Samsung driver and needs no Zadig driver replacement.
- `check-update -m SM-A065F -r XID` reaches Samsung FUS successfully and returns
  real version data, so the earlier SChannel failure does not affect this build.

`samloader detect` correctly reports "Failed to detect compatible download-mode
device" while the phone is running Android. That is the expected answer, not a
driver problem.

## The workflow

Each stage must complete before the next one is reachable. The backend refuses
to skip a stage, so a crash or restart cannot bypass a validation step.

1. **Detect device** — model, CSC, PDA, bootloader binary, verified boot state.
2. **Validate firmware** — exact model, sales code, anti-rollback, ZIP integrity,
   internal MD5 for AP/BL/CP/CSC.
3. **Patch the AP** — Magisk patches the AP on the phone. This step is manual
   because Magisk's patcher is an on-device UI.
4. **Pull the patched AP** — the backend refuses a file that is byte-identical to
   the stock AP, which catches pulling the wrong file.
5. **Preflight** — thirteen read-only checks covering ADB, session, bootloader,
   root state, every slot file, the patched AP, the engine, free space and Magisk.
6. **Approve** — an explicit confirmation, recorded in the session.
7. **Show the command** — the exact `samloader flash` invocation, still unexecuted.
8. **Flash** — deliberately not wired to the UI yet. See *Blockers*.

### Dry run

```powershell
'{"id":"1","action":"startSession","payload":{}}',
'{"id":"2","action":"dryRun","payload":{}}' | .\bin\myapp-go.exe
```

## What is still manual

These cannot be automated safely:

- **Bootloader unlocking** — a device-specific process that normally wipes data.
  AutoRoot refuses to run while the bootloader is locked, and never re-locks it.
- **Patching the AP in Magisk** — the patcher is an on-device UI.
- **Entering Download Mode** — on some models the phone must be powered off and
  booted with Volume Up held, because Download Mode is not reachable over ADB
  from a running system.
- **Approving the flash** — by design.

## Tests

```powershell
cd backend
go vet ./...
go test ./...
```

The suite covers the firmware validator (wrong model, anti-rollback, corrupted
MD5, HTML capture, missing slot), the downloader (resume, server ignoring Range,
checksum and size mismatch), the state machine (persistence, stage skipping,
approval gating) and the flash planner (CSC mode, engine pinning).

## Blockers

- **Firmware is not downloaded.** `D:\Data Kelola IT\Firmware\SM-A065F` is empty.
  A 5.81 GB download has not been attempted, per instruction. FUS confirms the
  build exists: `A065FXXS4AYE2/A065FOLE4AYE2/A065FXXS4AYE1/A065FXXS4AYE2`, with
  `A065FXXS9CZA1` as the newest revision.
- **Magisk is not on the phone.** `com.topjohnwu.magisk` is absent, so the AP
  cannot be patched yet.
- **The AP has not been patched.** This needs the Magisk UI on the device and
  remains manual by nature.
- **Download Mode detection is unverified on this device.** `samloader detect`
  has not been exercised against an SM-A065F in Download Mode, because that
  requires rebooting the phone. Until then, the device's Download Mode reachability
  is an assumption, not a verified fact.
- **Flashing is not wired to the UI.** `flashPlan` builds and displays the exact
  `samloader flash` command, but the execution step is deliberately absent. There
  is no `flash` action in the Electron IPC whitelist at all.
