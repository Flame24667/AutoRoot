export const STAGE_ORDER = ['idle', 'device-check', 'adb-authorized', 'firmware-found', 'firmware-valid', 'patched-ap', 'preflight-ok', 'flash-approved', 'download-mode', 'flashing', 'waiting-boot', 'verifying', 'rooted'];

export function stageRank(stage) { return STAGE_ORDER.indexOf(stage); }
export function isPostFlash(session) { return ['waiting-boot', 'verifying', 'rooted'].includes(session?.stage); }
export function matchesDevice(session, device) { return !!session && !!device && session.serial === device.serial && session.model === device.model; }
export function canPrepare(session, device) {
  return matchesDevice(session, device) && !device.rooted && stageRank(session.stage) >= 1 && stageRank(session.stage) <= stageRank('patched-ap');
}
export function hasRootProof(result) { return result?.rooted === true && /(?:^|\s)uid=0\(root\)(?:\s|$)/.test(result.raw || ''); }
export function rootLabel(device, proof) {
  if (!device) return 'Unknown — reconnect and authorize ADB';
  if (proof?.serial === device.serial && hasRootProof(proof)) return 'Yes — uid=0 verified';
  return 'Not verified — run actual root check';
}
