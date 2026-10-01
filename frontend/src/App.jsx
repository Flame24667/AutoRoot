import { useState, useEffect, useCallback, useRef } from 'react';
import { isPostFlash, matchesDevice } from './workflow-view.js';
import WorkbenchView from './WorkbenchView.jsx';
import './App.css';

function App() {
  const [device, setDevice] = useState(null);
  const [session, setSession] = useState(null);
  const [busy, setBusy] = useState(false);
  const [log, setLog] = useState([]);
  const [error, setError] = useState('');
  const [preflight, setPreflight] = useState(null);
  const [engine, setEngine] = useState(null);
  const [flashCmd, setFlashCmd] = useState('');
  const [activeTab, setActiveTab] = useState('overview');
  const [approved, setApproved] = useState(false);
  const [database, setDatabase] = useState(null);
  const [patchPath, setPatchPath] = useState('');
  const [job, setJob] = useState(null);
  const [downloadIdentity, setDownloadIdentity] = useState(null);
  const [rootProof, setRootProof] = useState(null);
  const initialCheckStarted = useRef(false);

  const runJob = async (label, operation, payload = {}) => {
    setBusy(true); setError(''); say(`→ ${label}`);
    if (!['approveFlash', 'executeFlash'].includes(operation)) { setApproved(false); setFlashCmd(''); setPreflight(null); }
    try {
      const started = await window.goAPI.call('startAutomationJob', {operation, ...payload});
      setJob(started);
      for (;;) {
        await new Promise(resolve => setTimeout(resolve, 1500));
        const status = await window.goAPI.call('automationStatus', {});
        setJob(status);
        if (status.id !== started.id) throw new Error('Job identity changed. Stop and inspect.');
        if (status.status === 'running') continue;
        if (status.status !== 'completed') throw new Error(status.error || 'Job failed');
        if (status.result?.ok === false) throw new Error((status.result.errors || []).join('; ') || 'Validation failed');
        say(`✓ ${label}`);
        const saved = await window.goAPI.call('sessionState', {});
        setSession(saved.session || null);
        return status.result;
      }
    } catch (err) { setError(err.message); say(`✗ ${label}: ${err.message}`); throw err; }
    finally { setBusy(false); }
  };

  const say = useCallback((line) => {
    setLog((prev) => [...prev, line]);
  }, []);

  const run = useCallback(async (label, action, payload = {}) => {
    setBusy(true);
    setError('');
    try {
      say(`→ ${label}`);
      const result = await window.goAPI.call(action, payload);
      say(`✓ ${label}`);
      return result;
    } catch (err) {
      setError(err.message);
      say(`✗ ${label}: ${err.message}`);
      throw err;
    } finally {
      setBusy(false);
    }
  }, [say]);

  const refresh = useCallback(async () => {
    try {
      const info = await window.goAPI.call('getDeviceInfo', {});
      setDevice(info);
      return info;
    } catch {
      setDevice(null);
      setRootProof(null);
      return null;
    }
  }, []);

  const checkConnection = useCallback(async () => {
    setBusy(true); setError(''); setApproved(false); setRootProof(null);
    try {
      const saved = await window.goAPI.call('sessionState', {});
      let active = saved.session || null;
      setSession(active); // Keep post-flash guidance visible even without ADB.
      const info = await refresh();
      if (!info) return;
      if (active && !matchesDevice(active, info)) {
        setError('Connected phone does not match the saved session. No workflow actions will run.');
        return;
      }
      if (!active) {
        const started = await window.goAPI.call('startSession', {});
        active = started.session;
        setSession(active);
      }
      setEngine(await window.goAPI.call('engineStatus', {}));
      if (isPostFlash(active) || info.rooted) {
        const proof = await window.goAPI.call('verifyRoot', {deviceID: info.serial});
        setRootProof({...proof, serial: info.serial});
        const latest = await window.goAPI.call('sessionState', {});
        setSession(latest.session || active);
        setPreflight(null); // Initial-root preflight is not a post-root health check.
      } else {
        setPreflight(await window.goAPI.call('preflight', {}));
      }
      setDatabase(await window.goAPI.call('databasePlan', {}));
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }, [refresh, say]);

  useEffect(() => {
    if (initialCheckStarted.current) return; // React development StrictMode replays effects.
    initialCheckStarted.current = true;
    checkConnection();
  }, [checkConnection]);
  const onNewRun = async () => {
    if (!matchesDevice(session, device) || device.rooted) return;
    if (!window.confirm('Start a NEW initial-root PoC after FULL STOCK RESTORE? The bootloader must remain unlocked and su must be absent. Old local session evidence will be archived. This does NOT restore, wipe or flash the phone.')) return;
    try {
      const res = await run('Archive old evidence and start new run', 'beginNewRun', {serial: device.serial, confirmNewRun: true});
      setSession(res.session); setJob(null); setRootProof(null); setPreflight(null); setApproved(false); setFlashCmd(''); setPatchPath('');
      say(res.message);
      if (res.archivedSession) say(`Previous session preserved: ${res.archivedSession}`);
      await checkConnection();
    } catch { /* already displayed */ }
  };
  const onVerifyRoot = async () => {
    setRootProof(null);
    try {
      const info = await refresh();
      if (!matchesDevice(session, info)) throw new Error('Reconnect and authorize the phone belonging to this session.');
      const res = await run('Verify actual root', 'verifyRoot', {deviceID: info.serial});
      setRootProof({...res, serial: info.serial});
      say(res.message);
      const saved = await window.goAPI.call('sessionState', {});
      setSession(saved.session || null);
      setApproved(false); setPreflight(null);
    } catch (err) { setError(err.message); }
  };

  const onValidateFirmware = async () => {
    // A local file is required; the app never picks a package by name alone.
    const picked = await window.goAPI.pickFirmwareFile?.();
    if (!picked) {
      say('No file selected.');
      return;
    }
    const res = await runJob('Validate firmware', 'adoptFirmware', {
      archivePath: picked,
    });
    if (res.ok) {
      setSession(res.session);
      say(`Firmware accepted: model ${res.modelCode}, binary ${res.binary}.`);
    } else {
      (res.errors || []).forEach((e) => say(`  ✗ ${e}`));
    }
    setPreflight(await window.goAPI.call('preflight', {}));
  };

  const onDryRun = async () => {
    const res = await run('Dry run', 'dryRun', {});
    (res.steps || []).forEach((s) => say(`  ${s.ok ? '✓' : '✗'} ${s.name}: ${s.detail}`));
    setPreflight(res.preflight);
  };

  const onApprove = async () => {
    // The confirmation is the point of this step: flashing wipes data on a
    // first install, and it must never happen because of a stray click.
    const ok = window.confirm(
      'This will flash the phone.\n\n' +
        'The first Magisk install uses the standard CSC and WIPES ALL DATA on the device.\n\n' +
        'Make sure everything is backed up. Continue?'
    );
    if (!ok) {
      say('Approval declined.');
      return;
    }
    await runJob('Approve flash', 'approveFlash', { by: 'operator' });
    setApproved(true);
    say('Flash approved. You can now inspect the exact command.');
  };

  const onBuildPlan = async () => {
    const res = await run('Build flash plan', 'flashPlan', {
      installMode: 'initial-root',
    });
    setFlashCmd(res.command);
    say('Flash plan built. Nothing has been flashed.');
  };

  const onProbeEngine = async () => {
    if (!window.confirm('Reboot this phone into Download Mode for a READ-ONLY identity/PIT probe? No partitions will be flashed. You must restart the phone manually afterwards.')) return;
    try { const res = await runJob('Probe engine without flashing', 'probeEngine', {confirmReboot: true}); say(res.message); if (res.needsBinding) setDownloadIdentity(res); }
    catch { /* error is already shown */ }
  };

  const onExecuteFlash = async () => {
    if (!window.confirm(`FINAL CONFIRMATION: flash ${session?.model} / ${session?.serial} with the approved files? This WILL WIPE DATA. Do not disconnect USB or close the app during flashing.`)) return;
    try { const res = await runJob('Flash approved firmware', 'executeFlash', {confirmWipe: true}); say(res.message); setApproved(false); setDevice(null); setRootProof(null); setPreflight(null); }
    catch { /* no automatic retry */ }
  };

  const onBind = async () => {
    if (!downloadIdentity || !window.confirm(`Pair ${downloadIdentity.model}: ADB ${downloadIdentity.adbSerial} → Download Mode ${downloadIdentity.downloadSerial}? Confirm only this phone is connected. No flashing.`)) return;
    try {
      const res = await runJob('Bind identity and read PIT', 'bindDownloadDevice', {confirmOnlyDevice: true, downloadSerial: downloadIdentity.downloadSerial});
      say(res.message); setDownloadIdentity(null);
    } catch { /* already displayed */ }
  };
  const safely = (action) => () => Promise.resolve().then(action).catch(err => setError(err.message));
  return <WorkbenchView
    {...{device, session, busy, error, rootProof, preflight, engine, database, job, log, approved, flashCmd, patchPath, downloadIdentity, activeTab}}
    onTab={setActiveTab}
    actions={{
      reconnect: safely(checkConnection), verify: safely(onVerifyRoot), newRun: safely(onNewRun),
      database: safely(() => runJob('Prepare firmware from database', 'prepareDatabase')),
      localFirmware: safely(onValidateFirmware),
      preparePatch: safely(() => runJob('Install Magisk and transfer AP', 'preparePatch').then(res => say(res.message))),
      autoPatch: safely(() => runJob('Automate Magisk patch and collect output', 'automateMagiskPatch')),
      patchPath: setPatchPath,
      collect: safely(() => runJob('Pull and validate new patch', 'collectPatch', {remotePath: patchPath})),
      probe: safely(onProbeEngine), bind: safely(onBind),
      diagnose: safely(() => run('Engine launch diagnostic (--version only)', 'diagnoseEngineLaunch').then(res => say(JSON.stringify(res, null, 2)))),
      recover: safely(async () => {
        if (!window.confirm('Recover ONLY a proven engine launch failure before any flash? The failed session will be archived, files/device rechecked and old approval revoked. No reboot or flash.')) return;
        const res = await runJob('Recover proven pre-launch failure', 'recoverEngineLaunch', {confirmRecovery: true, serial: device?.serial});
        say(res.message); if (res.archivedSession) say(`Failure evidence: ${res.archivedSession}`);
      }),
      preflight: safely(() => run('Preflight', 'preflight').then(setPreflight)),
      dryRun: safely(onDryRun), approve: safely(onApprove), plan: safely(onBuildPlan), flash: safely(onExecuteFlash),
    }}
  />;
}

export default App;
