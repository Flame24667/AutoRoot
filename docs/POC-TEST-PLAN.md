# AutoRoot PoC — operator-led test plan

## Scope

Windows x64; SM-A065F / XID / PDA A065FXXS4AYE2 / OMC A065FOLE4AYE2 /
binary 4; English Magisk 30.7; Ramdisk Yes. Other devices/builds are not claimed.
Success requires actual `uid=0(root)` after normal boot, not engine exit success,
an APK or a saved stage. Bootloader unlock, recovery reset/setup, USB authorization,
Superuser consent and destructive approval remain operator checkpoints.
Database selection uses existing local stock firmware; automatic network firmware
download is not part of the validated PoC.

## Local checks (no phone)

```powershell
npm --prefix frontend test
powershell -NoProfile -File scripts\test-ui.ps1
cd backend
go test ./...
go vet ./...
go build -o ..\bin\myapp-go.automation.exe .
cd ..
powershell -NoProfile -File scripts\build-poc.ps1
```

Static build runs native esbuild with automatic JSX; output is work-cache/poc-ui
(ignored by Git). This is separate from the normal Vite production build, which
still hits Node subprocess-pipe EPERM in the managed execution environment.

## UI smoke (no repeat flash)

Keep the current rooted phone unchanged. Close the old app, then:

```powershell
# Run from the project root.
powershell -NoProfile -File scripts\start-poc.ps1
```

Expect session rooted and **ROOTED: YES** after an actual matching-phone
query. Initial-root controls stay disabled. Disconnect/reconnect must show unknown
status until authorization and live verification, never automatic flash or false
success. Save screenshots/errors. GPT guides; the operator controls the app/phone.

Check all four tabs: Overview, Preparation, Flash, Checks & logs. Tested
configuration is fixed scope; Connected device is live identity after refresh.
On the current rooted session, preparation, flash and new-session buttons must
stay disabled. Refresh after reconnecting; the app does not continuously monitor
USB, so a previously displayed check is not a guarantee that USB remains connected.

## Full fresh-flow test (destructive, separately approved)

Do not begin on the currently rooted phone. Full stock restore and wipe are a
separate reviewed operation. Keep all stock slots and logs; do not relock.
Know Google/Samsung credentials or remove accounts through Settings before reset.
Unrooting does not reset Knox.

After separately reviewed complete stock restore:

1. Complete setup to Home and enable/authorize USB debugging.
2. Reconnect in AutoRoot; choose **Start a new PoC session** and confirm.
   Backend requires the same model/serial, unlocked bootloader and no su. It
   archives prior local evidence. The button never restores/wipes/reboots/flashes.
3. Check database model/CSC/PDA/OMC/binary/source. Select and validate its stock ZIP.
   If unavailable, choose the correct stock ZIP manually; do not guess a firmware.
4. Prepare patch: install full Magisk and transfer stock AP.
5. Automate patch + collect. Keep screen unlocked, wait for new output and validation.
   Unknown/ambiguous UI stops instead of guessing or starting a duplicate patch.
6. Approve the read-only Download Mode engine probe. Confirm identity pairing
   only if just the intended phone is connected; read PIT. Restart manually
   afterward because a no-reboot PIT session may make subsequent identity empty.
7. Reconnect/authorize. Refresh preflight; all required checks must pass.
8. Choose **Approve plan**, **Show command**, then **Flash & wipe data** and explicitly confirm wipe. Keep USB/laptop/app
   connected and awake. No retry or arbitrary kill during an active flash.
   Redirected transfer percent is unavailable; hashing percent is not flash percent.
9. Perform recovery-requested reset if shown, finish initial setup, restore USB
   authorization, install/open full Magisk and finish additional setup/reboot.
10. Reauthorize if needed and verify actual root. Grant Shell consent yourself.
    Pass requires uid=0(root), normal Android boot and session rooted.

Stop for unknown screens, identity/build mismatch, validator failure or flash error.
Preserve logs; never automatically retry a failed flash.

## Stock restore preparation — separate destructive operation

Do not restore as part of UI smoke testing. When the operator is ready for a
fresh initial-root test, review the actual files and live phone identity first.
Uninstalling the Magisk app alone is not a full stock restore.

1. Preserve the prior session/engine logs and stock package. Verify model SM-A065F,
   XID inclusion in the multi-CSC package and the phone's current rollback binary.
   Binary 4 is historical test scope, not permission to downgrade a changed phone.
2. Use the complete, validated stock BL/AP/CP/CSC package from one firmware set;
   AP must be the original stock AP, not magisk_patched. Do not restore individual
   boot/recovery/vbmeta partitions or mix patched and stock slots.
3. Review account credentials, wipe consent, battery, USB and correct device before
   entering Download Mode. In Odin select BL, stock AP, CP and CSC (not HOME_CSC
   for this clean, destructive baseline). Leave USERDATA empty; do not manually
   enable Re-Partition or provide a separate PIT. Do not start until reviewed.
4. After an approved full restore, wait for completion, perform any requested
   recovery reset and finish setup. Keep the bootloader unlocked; do not relock
   for this PoC. Knox Warranty Bit is not restored by returning to stock.
5. Reauthorize USB. AutoRoot's new-session gate requires the same phone, unlocked
   bootloader, no root proof and no su executable before preserving old evidence
   and opening a fresh run. A blocker is a stop condition, not a bypass request.

Reference: [Magisk Samsung installation and important notes](https://topjohnwu.github.io/Magisk/install.html#samsung-devices).
The destructive stock restore and operator-led root test were completed before
publication cleanup; see TEST-RESULTS-2026-10-01.md. Cleanup does not authorize a
new restore/flash or change the current rooted phone.

## Evidence and release gates — 1 October 2026

| Check | Evidence |
| --- | --- |
| DB local selection + firmware validation | Passed in earlier live run |
| Magisk UI patch + collection | Passed in earlier live run |
| CLI flash/reboot + uid=0 verification | Passed in earlier live run |
| Latest new-run guards / evidence archive | Operator confirmed live after full stock restore |
| Static PoC build and Go/UI policy tests | Passed |
| Latest static launcher visual run | Overview/Preparation/Flash and disconnect/reconnect screenshots confirmed |
| Latest full flow repeated by operator | Completed with interventions; final uid=0 and rooted |
| Network firmware download | Not tested; outside validated PoC |
| Vite build / installer packaging | Vite EPERM in managed environment; packaging untested |

## Recovery of a proven engine launch failure

This is not recovery from a failed/partial firmware write. Unknown process state,
an engine that started, or an exit-status failure must remain blocked. Only the
recorded process-start failure (including the observed legacy Access is denied
CreateProcess error) is eligible. Keep the phone at normal Home and authorize ADB.

In Checks & logs, the operator selects **Recover pre-launch failure (no flash)**
and explicitly confirms. A background job runs the pinned engine's --version,
checks the same stock/unrooted phone with no su, preflight, PIT checksum and exact
firmware/patch fingerprint. It archives the original failed session, returns to
patched-ap and revokes approval. No reboot, flash or new patch is performed.

Recovery itself is not consent to retry. Refresh preflight, review a new plan and
give fresh destructive approval separately. Engine launch success does not prove
that flash will succeed or explain the earlier permission denial.

## Git handoff

Record smoke/full-flow results before push. No commit/push is performed now.
Review diffs and stage explicit files, not git add .; unrelated .commandcode edits
belong to their owner. Include new frontend helpers/tests, backend code, scripts
and this guide. Exclude generated outputs, firmware, state/logs and new binaries.
Some APK/EXE/dist assets were already tracked upstream; ignore rules do not remove
them from history. Review licensing/size separately, do not silently delete them.
Machine-specific paths use ignored setup/firmware-db.local.json; the public
catalog uses a project-relative local ZIP path. This is not a verified network
download database. The tested Windows samloader.exe must match the pinned
SHA-256 in backend/samloader.go before it can run.
