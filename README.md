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

AutoRoot refuses to run an unverified flashing engine. Install samloader-rs with
its official SHA-256:

```powershell
.\scripts\fetch-samloader.ps1 -Version 0.6.0 -ExpectedSHA256 <sha256-from-official-release>
```

Then record the digest in `backend/samloader.go`:

```go
const samloaderVersion = "0.6.0"
var samloaderPins = map[string]string{ "0.6.0": "<sha256>" }
```

Review the pin before installing:

```powershell
.\scripts\fetch-samloader.ps1 -PinOnly -Version 0.6.0 -ExpectedSHA256 <sha256>
```

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

- **Firmware is not downloaded.** `D:\Data Kelola IT\Firmware\SM-A065F` is
  empty. A 5.81 GB download has not been attempted.
- **The engine is not installed and not pinned.** `samloaderPins` is empty, so
  AutoRoot refuses to run the engine until a digest is recorded.
- **Download Mode detection is unverified on this device.** The `samloader detect`
  path has not been exercised against an SM-A065F, because doing so requires
  rebooting the phone into Download Mode.
- **Flashing is not wired to the UI.** The plan is built and displayed, but the
  execution step is intentionally absent until the blockers above clear.
