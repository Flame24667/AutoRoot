import { useState, useEffect, useCallback } from 'react';

// The UI mirrors the backend's stage order. It cannot skip ahead because the
// backend refuses to build a flash plan until every earlier stage has been
// recorded and approved, so the button states here are a convenience, not the
// actual guard.
const STAGES = [
  { key: 'device-check', label: 'Detect device' },
  { key: 'firmware-valid', label: 'Firmware validated' },
  { key: 'patched-ap', label: 'AP patched' },
  { key: 'preflight-ok', label: 'Preflight passed' },
  { key: 'flash-approved', label: 'Flash approved' },
];

const STAGE_RANK = Object.fromEntries(STAGES.map((s, i) => [s.key, i]));

function App() {
  const [device, setDevice] = useState(null);
  const [session, setSession] = useState(null);
  const [busy, setBusy] = useState(false);
  const [log, setLog] = useState([]);
  const [error, setError] = useState('');
  const [preflight, setPreflight] = useState(null);
  const [engine, setEngine] = useState(null);
  const [flashCmd, setFlashCmd] = useState('');
  const [downloadUrl, setDownloadUrl] = useState('');
  const [approved, setApproved] = useState(false);

  const say = useCallback((line) => {
    setLog((prev) => [...prev, line]);
  }, []);

  const run = useCallback(async (label, action, payload = {}) => {
    setBusy(true);
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
      return null;
    }
  }, []);

  // On mount: detect the phone, resume any persisted session, and read the
  // engine and preflight status. All of it is read-only.
  useEffect(() => {
    (async () => {
      const info = await refresh();
      if (!info) return;

      try {
        const started = await window.goAPI.call('startSession', {});
        setSession(started.session);
        say(
          started.resumed
            ? `Resumed session at stage "${started.stage}"`
            : `Started a new session at stage "${started.stage}"`
        );
      } catch (err) {
        setError(err.message);
        return;
      }

      try {
        setEngine(await window.goAPI.call('engineStatus', {}));
        setPreflight(await window.goAPI.call('preflight', {}));
      } catch (err) {
        setError(err.message);
      }
    })();
  }, [refresh, say]);

  const currentRank = session ? (STAGE_RANK[session.stage] ?? -1) : -1;

  const onValidateFirmware = async () => {
    // A local file is required; the app never picks a package by name alone.
    const picked = await window.goAPI.pickFirmwareFile?.();
    if (!picked) {
      say('No file selected.');
      return;
    }
    const res = await run('Validate firmware', 'adoptFirmware', {
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

  const onDryRunDownload = async () => {
    if (!downloadUrl.trim()) {
      setError('Enter a firmware URL first.');
      return;
    }
    const res = await run('Plan download', 'fetchFirmware', {
      url: downloadUrl.trim(),
      filename: downloadUrl.trim().split('/').pop(),
      dryRun: true,
    });
    say(res.message);
  };

  const onDownload = async () => {
    if (!downloadUrl.trim()) return;
    const res = await run('Download firmware', 'fetchFirmware', {
      url: downloadUrl.trim(),
      filename: downloadUrl.trim().split('/').pop(),
      dryRun: false,
    });
    say(res.message || `Downloaded to ${res.path}`);
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
    const res = await run('Approve flash', 'approveFlash', { by: 'operator' });
    setSession({ ...session, stage: res.stage, approval: true });
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

  const blockers = preflight?.blockers || [];

  return (
    <div style={S.page}>
      <header style={S.header}>
        <h1 style={S.title}>AutoRoot</h1>
        <p style={S.subtitle}>
          Samsung Galaxy A06 (SM-A065F) · every destructive step is gated
        </p>
      </header>

      {error && <div style={S.error}>{error}</div>}

      {device ? (
        <div style={S.grid}>
          <div style={S.card}>
            <h2 style={S.h2}>Device</h2>
            <dl style={S.dl}>
              <dt>Model</dt><dd>{device.model}</dd>
              <dt>Serial</dt><dd>{device.serial}</dd>
              <dt>Android</dt><dd>{device.androidVersion}</dd>
              <dt>Build</dt><dd>{device.buildVersion}</dd>
              <dt>CSC</dt><dd>{device.salesCode || 'unknown'}</dd>
              <dt>Bootloader binary</dt><dd>{device.binaryBit}</dd>
              <dt>Bootloader</dt>
              <dd>{device.bootloaderLocked ? 'LOCKED' : 'unlocked'}</dd>
              <dt>Verified boot</dt><dd>{device.verifiedBootState}</dd>
              <dt>Root</dt><dd>{device.rooted ? 'active' : 'not active'}</dd>
            </dl>
          </div>

          <div style={S.card}>
            <h2 style={S.h2}>Flashing engine</h2>
            {engine?.available ? (
              <p style={S.ok}>
                samloader {engine.version} verified
                <br />
                <code style={S.code}>{engine.path}</code>
              </p>
            ) : (
              <p style={S.bad}>
                Not ready
                <br />
                <span style={S.small}>{engine?.reason || 'unknown'}</span>
              </p>
            )}
          </div>
        </div>
      ) : (
        <div style={S.card}>
          <h2 style={S.h2}>No device</h2>
          <p style={S.muted}>
            Connect the phone over USB and accept the USB debugging prompt. The
            phone must show as <code>device</code>, not <code>unauthorized</code>.
          </p>
          <button style={S.secondary} onClick={refresh}>Check again</button>
        </div>
      )}

      {session && (
        <div style={S.card}>
          <h2 style={S.h2}>Progress</h2>
          <ol style={S.stages}>
            {STAGES.map((s) => {
              const done = (STAGE_RANK[session.stage] ?? -1) >= STAGE_RANK[s.key];
              return (
                <li key={s.key} style={done ? S.stageDone : S.stageTodo}>
                  {done ? '✓' : '○'} {s.label}
                </li>
              );
            })}
          </ol>
        </div>
      )}

      <div style={S.card}>
        <h2 style={S.h2}>Firmware</h2>
        <p style={S.muted}>
          Packages download straight to the D: volume and are validated before
          anything else can use them. Validation checks the model, the sales
          code, anti-rollback, ZIP integrity and every internal MD5.
        </p>
        <div style={S.row}>
          <button style={S.primary} disabled={busy} onClick={onValidateFirmware}>
            Choose a local .zip and validate
          </button>
        </div>
        <div style={S.row}>
          <input
            style={S.input}
            placeholder="Firmware URL (Samsung FUS link)"
            value={downloadUrl}
            onChange={(e) => setDownloadUrl(e.target.value)}
          />
        </div>
        <div style={S.row}>
          <button style={S.secondary} disabled={busy} onClick={onDryRunDownload}>
            Check download (dry run)
          </button>
          <button style={S.secondary} disabled={busy} onClick={onDownload}>
            Download
          </button>
        </div>
      </div>

      <div style={S.card}>
        <h2 style={S.h2}>Preflight</h2>
        {preflight ? (
          <>
            <ul style={S.checks}>
              {(preflight.checks || []).map((c) => (
                <li key={c.id} style={c.passed ? S.checkOk : S.checkBad}>
                  {c.passed ? '✓' : '✗'} {c.label}
                  {c.detail ? <span style={S.small}> — {c.detail}</span> : null}
                </li>
              ))}
            </ul>
            <p style={preflight.ok ? S.ok : S.bad}>{preflight.summary}</p>
          </>
        ) : (
          <p style={S.muted}>Run a preflight to see what is still missing.</p>
        )}
        <div style={S.row}>
          <button style={S.secondary} disabled={busy} onClick={onDryRun}>
            Run dry run
          </button>
          <button
            style={S.secondary}
            disabled={busy}
            onClick={() => run('Preflight', 'preflight', {}).then(setPreflight)}
          >
            Refresh preflight
          </button>
        </div>
      </div>

      <div style={S.card}>
        <h2 style={S.h2}>Flash</h2>
        {blockers.length > 0 && (
          <p style={S.bad}>
            {blockers.length} requirement(s) are still unmet, so flashing stays
            locked.
          </p>
        )}
        <div style={S.row}>
          <button
            style={S.danger}
            disabled={busy || !approved}
            onClick={onApprove}
          >
            Approve flashing
          </button>
          <button
            style={S.secondary}
            disabled={busy || !approved}
            onClick={onBuildPlan}
          >
            Show the exact command
          </button>
        </div>
        {flashCmd && (
          <pre style={S.pre}>{flashCmd}</pre>
        )}
        <p style={S.muted}>
          AutoRoot will not flash on its own. Approval is recorded in the saved
          session, so a restart does not silently re-arm it.
        </p>
      </div>

      <div style={S.card}>
        <h2 style={S.h2}>Log</h2>
        <pre style={S.pre}>{log.join('\n') || 'No activity yet.'}</pre>
      </div>
    </div>
  );
}

const S = {
  page: {
    minHeight: '100vh',
    background: '#0b1220',
    color: '#e6edf7',
    fontFamily: 'system-ui, sans-serif',
    padding: '2rem 1rem 4rem',
    maxWidth: 900,
    margin: '0 auto',
  },
  header: { marginBottom: '1.5rem' },
  title: { fontSize: '2rem', fontWeight: 700, margin: 0 },
  subtitle: { color: '#93a4bd', margin: '0.25rem 0 0', fontSize: '0.95rem' },
  card: {
    background: '#131c2e',
    border: '1px solid #22304a',
    borderRadius: 12,
    padding: '1.25rem',
    marginBottom: '1rem',
  },
  h2: { fontSize: '1.1rem', margin: '0 0 0.75rem', color: '#cdd9ea' },
  grid: { display: 'grid', gap: '1rem', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' },
  dl: { display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '0.35rem 1rem', margin: 0, fontSize: '0.9rem' },
  stages: { margin: 0, paddingLeft: '1.2rem', lineHeight: 1.9, fontSize: '0.95rem' },
  checks: { margin: 0, paddingLeft: 0, listStyle: 'none', lineHeight: 1.8, fontSize: '0.9rem' },
  row: { display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginTop: '0.5rem' },
  input: {
    flex: '1 1 320px',
    padding: '0.6rem 0.75rem',
    background: '#0b1220',
    border: '1px solid #2c3d5c',
    borderRadius: 8,
    color: '#e6edf7',
  },
  primary: {
    padding: '0.7rem 1.1rem',
    background: '#2563eb',
    color: '#fff',
    border: 'none',
    borderRadius: 8,
    cursor: 'pointer',
    fontWeight: 600,
  },
  secondary: {
    padding: '0.6rem 1rem',
    background: 'transparent',
    color: '#cdd9ea',
    border: '1px solid #3a4d70',
    borderRadius: 8,
    cursor: 'pointer',
  },
  danger: {
    padding: '0.6rem 1rem',
    background: '#b91c1c',
    color: '#fff',
    border: 'none',
    borderRadius: 8,
    cursor: 'pointer',
    fontWeight: 600,
  },
  pre: {
    background: '#0b1220',
    border: '1px solid #22304a',
    borderRadius: 8,
    padding: '0.75rem',
    overflowX: 'auto',
    fontSize: '0.8rem',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-all',
  },
  ok: { color: '#34d399' },
  bad: { color: '#f87171' },
  muted: { color: '#93a4bd', fontSize: '0.9rem', lineHeight: 1.6 },
  small: { color: '#7c8ba5', fontSize: '0.82rem' },
  code: { color: '#7dd3fc', fontSize: '0.82rem' },
  error: {
    background: '#3b1418',
    border: '1px solid #7f1d1d',
    color: '#fca5a5',
    padding: '0.75rem 1rem',
    borderRadius: 8,
    marginBottom: '1rem',
  },
  stageDone: { color: '#34d399' },
  stageTodo: { color: '#64748b' },
  checkOk: { color: '#34d399' },
  checkBad: { color: '#f87171' },
};

export default App;
