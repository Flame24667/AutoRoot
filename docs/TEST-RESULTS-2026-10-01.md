# Operator-led test results — 1 October 2026

## Result and scope

**PASS with interventions:** the operator ran the desktop UI after a full stock
restore on SM-A065F / XID / Android 14 / A065FXXS4AYE2 /
A065FOLE4AYE2 / binary 4. English Magisk 30.7, Ramdisk Yes.
Normal Android Home and app verification returned:

```text
uid=0(root) gid=0(root) groups=0(root) context=u:r:magisk:s0
```

The final local session recorded rooted at 16:15:35 (Asia/Bangkok).
The successful automated flash attempt ran 15:50:34–15:55:46.
This report deliberately excludes device serials, Download Mode identifiers,
absolute lab paths and raw logs. Original evidence stays locally in bin/state.

## Observed checks

| Check | Observed result |
| --- | --- |
| Full stock restore before repeat test | Odin PASS; operator finished setup |
| Rooted UI smoke / preparation and flash guards | Buttons disabled on completed session |
| Disconnect and explicit refresh | ADB disconnected, NOT VERIFIED; no false root badge |
| Reconnect and verification | Matching live uid=0 restored the badge |
| Fresh session gate and archive | adb-authorized; old evidence preserved |
| Database selection / stock integrity validation | firmware-valid using local ZIP |
| Install Magisk and transfer AP | Completed |
| Automatic UI patch and collection | Completed; patched-ap after checksum/structure validation |
| Read-only identity/PIT probe | Completed; operator restarted to Home afterwards |
| Preflight / separate approvals | Passed, command reviewed and wipe explicitly approved |
| First automated flash attempt | Failed before engine launch; no automatic retry |
| Diagnostics / explicit recovery | Version test passed; failure archived, approval revoked |
| Fresh approval and subsequent automated flash | Engine completed without error |
| Recovery/reset/setup/Magisk security checkpoints | Completed manually by operator |
| Final root verification | uid=0; session rooted |

## Issues encountered and disposition

- **Screen-awake value 15 rejected:** old code allowed only 0–7. Added dock bit
  support and tests covering 0–15; preserves the original preference for restore.
- **Unknown Magisk screen / null Android UI root:** adapter stopped without
  guessing taps. Operator presented Magisk Home and later retried. A successful
  patch was observed; intermittent hierarchy failure is not claimed eliminated.
- **Collection progress stayed at 0%:** local file appeared and backend CPU
  increased; job eventually completed. UI progress remains coarse for transfer
  and remote checksum; avoid interpreting 0% as an automatic retry signal.
- **Engine Access is denied:** process launch failed; failed session/log were
  preserved. Version probes from terminal and app subsequently succeeded.
  Root cause remains unproven; no antivirus disabling or automatic retry.
- **Recovery stage check failed:** ordinary preflight read stage failed instead
  of validating a proposed patched-ap candidate. Added a regression test;
  candidate checks do not reset persisted evidence before successful validation.
- **Android Recovery cannot-load-system screen:** operator performed the
  already-approved factory reset, then completed setup. No repeat flash required.

## Not validated / deferred

- SM-A045F and all other device/build combinations.
- Network firmware downloading and the future Firmware Manager.
- Fully unattended rooting or automatic security/recovery confirmations.
- Clean-machine installation, installer packaging and full Vite build in the
  managed environment (Node subprocess restrictions).
- Long-duration/repeated-run reliability and the engine permission root cause.

Local source changes for publication cleanup were tested offline, not by another
destructive phone run. The app may need rebuilding before using those sources.
No new device actions, commit or push were performed for documentation cleanup.

## Offline cleanup verification

- Go tests (`go test -count=1 ./...`), `go vet` and backend build: passed.
- UI safety tests: 11 passed, 0 failed (including AutoRoot branding); static PoC
  frontend build: passed.
- Electron main/preload JavaScript syntax checks: passed.
- Intended tracked source diff whitespace check and public catalog JSON parse:
  passed. Private catalog, firmware, engine and runtime state remain ignored.
- Public source/docs scan found no tested phone identifiers or lab-specific
  absolute paths. This is not a full Git-history or secrets audit.

Go emitted a sandbox-denied telemetry-token warning; test, vet and build commands
still exited successfully. No extra permissions were requested for telemetry.
