# AutoRoot

AutoRoot is an experimental Electron + React application that coordinates Android device detection and a guarded Samsung Magisk/Odin workflow through a Go backend.

> **Safety status:** device detection works. Rooting is intentionally blocked while the bootloader is locked, while the firmware set is incomplete, or while a valid Odin executable is unavailable. Do not use a daily-driver phone. Back up all data first.

## Requirements

- Windows 10/11
- Node.js and npm
- Go 1.26.2 or newer
- Android SDK Platform Tools (`adb` and `fastboot`) on `PATH`
- Samsung USB driver when using a Samsung device
- Exactly one connected Android device

## Development setup

```powershell
npm ci
npm approve-scripts electron
npm rebuild electron

cd frontend
npm ci
npm approve-scripts esbuild
npm rebuild esbuild
cd ..

New-Item -ItemType Directory -Force bin
cd backend
go build -o ..\bin\myapp-go.exe .
cd ..

npm run dev
```

## Safe workflow

1. Enable USB debugging and approve the computer on the phone.
2. Confirm that `adb devices` shows exactly one device with status `device`.
3. Start AutoRoot and detect the phone.
4. Review model, build, CSC, root state, and bootloader state.
5. If the bootloader is locked, stop. Unlocking is a manual, device-specific operation and normally factory-resets the phone.
6. Supply only an official firmware ZIP matching the exact model and bootloader revision.
7. AutoRoot requires AP, BL, CP, and **CSC** for the first Samsung Magisk installation, which performs the required data wipe. A later firmware update on an already rooted device uses **HOME_CSC**.
8. The Samsung workflow installs Magisk, sends AP to the phone, pauses for manual patching, retrieves the patched AP, checks Odin, and only then offers the flash stage.
9. Root is verified through `su -c id` after Android boots again.

## What is not automated

- OEM unlocking and bootloader confirmation
- Physical button confirmations in Samsung Download Mode
- Selecting and approving the AP patch inside Magisk
- Obtaining a legally distributable, trusted Odin-compatible command-line tool

The bundled `odin/Odin3.exe` in the original archive is only a Git LFS pointer and is rejected by the backend. AutoRoot will stop before rebooting the phone until a valid executable is installed.

## Build

```powershell
npm run build
npm run package:win
```

Packaged builds include `Magisk-v30.7.apk`. Verify its hash against a trusted upstream source before use.

## Current scope

The guarded workflow is implemented for Samsung only. Fastboot brands are explicitly disabled until their boot-image patching and model validation flow is implemented and tested.
