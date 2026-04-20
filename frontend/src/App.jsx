import { useState, useEffect } from 'react';

function App() {
  const [step, setStep] = useState('guide');
  const [device, setDevice] = useState(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(false);

  const checkConnection = async () => {
    setChecking(true);
    setError('');
    setMessage('Checking ADB connection...');
    setFirmwareStatus('idle');
    try {
      const info = await window.goAPI.call('getDeviceInfo', {});
      setDevice(info);
      setStep('ready');
      setMessage(`✅ Connected: ${info.brand} ${info.displayName || info.model}`);
      
      setFirmwareStatus('checking');
      const fwRes = await window.goAPI.call('checkFirmware', {
        model: info.model,
        device: info.device
      });
      setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');
    } catch (err) {
      setError(err.message);
      setMessage('');
    } finally {
      setChecking(false);
    }
  };

  const startDetection = async () => {
    setStep('waiting');
    setError('');
    setMessage('Waiting for device... Enable USB Debugging & tap Allow on your phone.');
    setChecking(true);
    setFirmwareStatus('idle');
    
    const interval = setInterval(async () => {
      try {
        const info = await window.goAPI.call('getDeviceInfo', {});
        setDevice(info);
        setStep('ready');
        setMessage(`✅ Connected: ${info.brand} ${info.displayName || info.model}`);
        
        // 🔍 Check firmware availability immediately after detection
        setFirmwareStatus('checking');
        const fwRes = await window.goAPI.call('checkFirmware', {
          model: info.model,
          device: info.device
        });
        setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');
        
        clearInterval(interval);
      } catch (e) {
        // Still waiting, ignore transient errors
      }
    }, 2000);

    setTimeout(() => {
      clearInterval(interval);
      if (step === 'waiting') {
        setStep('guide');
        setError('Timeout. No device detected. Check USB cable & drivers.');
      }
    }, 60000);
  };

  const handleRoot = async () => {
    if (!device) return;
    setStep('rooting');
    setMessage('Starting root process. Keep device connected & screen on.');
    try {
      const res = await window.goAPI.call('rootDevice', {});
      setStep('success');
      setMessage(res.message || 'Root completed successfully!');
    } catch (err) {
      setStep('error');
      setError(err.message);
    }
  };

  const [firmwareStatus, setFirmwareStatus] = useState('idle'); // idle | checking | available | unavailable

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>🔓 AutoRoot</h1>
        <p style={styles.subtitle}>Secure Android Root Automation</p>
        <button style={styles.noteBtn} onClick={() => alert('Welcome to AutoRoot!')}>
          📝 Note
        </button>
      </header>

      <div style={styles.card}>
          {/* Optionally, show the note text somewhere on the main screen */}
          {/* <div style={styles.noteText}>Welcome to AutoRoot</div> */}
        {step === 'guide' && (
          <>
            <h2 style={styles.stepTitle}>Step 1: Enable USB Debugging</h2>
            <ol style={styles.steps}>
              <li>Open <b>Settings</b> → <b>About Phone</b></li>
              <li>Tap <b>Build Number</b> 7 times</li>
              <li>Go back → <b>Developer Options</b></li>
              <li>Toggle <b>USB Debugging</b> ON</li>
              <li>Connect USB & tap <b>"Allow"</b> on phone prompt</li>
            </ol>
            <button onClick={startDetection} style={styles.primaryBtn}>
              ✅ I've Enabled USB Debugging
            </button>
            <button onClick={checkConnection} style={styles.secondaryBtn} disabled={checking}>
              {checking ? 'Checking...' : '🔍 Manual Check Connection'}
            </button>
            <button onClick={() => { 
              setStep('guide'); 
              setDevice(null); 
              setError(''); 
              setFirmwareStatus('idle'); 
            }} style={styles.secondaryBtn}>
              Start Over
            </button>
          </>
        )}

        {step === 'waiting' && (
          <div style={styles.center}>
            <div style={styles.spinner}></div>
            <p style={styles.statusText}>{message}</p>
            <p style={styles.hint}>Auto-detects when phone authorizes ADB.</p>
          </div>
        )}
        {step === 'ready' && device && (
          <>
            <h2 style={styles.stepTitle}>Device Connected</h2>
            <div style={styles.deviceName}>
              {/* Prevents duplicate text like "ASUS_AI2401_D ASUS_AI2401_D" */}
              {device.displayName && device.displayName !== device.model
                ? device.displayName
                : `${device.brand} ${device.model}`}
            </div>
            <div style={styles.infoGrid}>
              <div><b>Brand:</b> {device.brand}</div>
              <div><b>Model Code:</b> {device.model}</div>
              {device.device && <div><b>Codename:</b> {device.device}</div>}
              <div><b>Android:</b> {device.androidVersion} (SDK {device.sdkVersion})</div>
              {device.securityPatch && <div><b>Security Patch:</b> {device.securityPatch}</div>}
              <div><b>Rooted:</b> {device.rooted ? 'Yes ✓' : 'No'}</div>
            </div>
            <button 
              onClick={handleRoot} 
              disabled={device.rooted || firmwareStatus !== 'available'}
              style={{
                ...styles.primaryBtn, 
                opacity: (device.rooted || firmwareStatus !== 'available') ? 0.6 : 1,
                background: firmwareStatus === 'unavailable' ? '#475569' : 'linear-gradient(135deg, #3b82f6, #6366f1)',
                cursor: (device.rooted || firmwareStatus !== 'available') ? 'not-allowed' : 'pointer'
              }}
            >
              {device.rooted ? '✅ Already Rooted' : 
              firmwareStatus === 'checking' ? '⏳ Checking Firmware...' : 
              firmwareStatus === 'unavailable' ? '📦 Firmware Unavailable' : 
              '🚀 Start Root Process'}
            </button>

            {firmwareStatus === 'unavailable' && (
              <p style={{ color: '#f87171', fontSize: '0.85rem', textAlign: 'center', marginTop: '0.5rem' }}>
                Firmware Unavailable. Contact the developer to add it to the application, or add it yourself by downloading the firmware and placing it in the firmware folder in the app directory.
              </p>
            )}
          </>
        )}

        {(step === 'rooting' || step === 'success' || step === 'error') && (
          <div style={styles.center}>
            <h2 style={styles.stepTitle}>
              {step === 'rooting' ? '⚙️ Rooting in Progress...' : 
                step === 'success' ? '✅ Root Successful!' : '❌ Root Failed'}
            </h2>
            <p style={styles.statusText}>{message || error}</p>
            {step !== 'rooting' && (
              <button onClick={() => { setStep('guide'); setDevice(null); setError(''); }} style={styles.secondaryBtn}>
                Start Over
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

const styles = {
    noteBtn: {
      marginTop: '0.5rem',
      background: 'linear-gradient(90deg, #38bdf8, #818cf8)',
      color: '#fff',
      border: 'none',
      borderRadius: '8px',
      padding: '0.5rem 1.2rem',
      fontWeight: 600,
      fontSize: '1rem',
      cursor: 'pointer',
      boxShadow: '0 2px 8px rgba(56,189,248,0.15)',
      transition: 'background 0.2s',
    },
    noteText: {
      margin: '1rem 0',
      padding: '0.75rem 1rem',
      background: '#334155',
      borderRadius: '8px',
      color: '#f8fafc',
      fontWeight: 500,
      textAlign: 'center',
      fontSize: '1.05rem',
      boxShadow: '0 2px 8px rgba(56,189,248,0.10)',
    },
  container: {
    height: '100vh',
    overflowY: 'auto',
    background: 'linear-gradient(135deg, #0f172a, #1e293b)',
    color: '#f8fafc',
    fontFamily: 'system-ui, sans-serif',
    padding: '1.5rem',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    boxSizing: 'border-box'
  },
  header: { textAlign: 'center', marginBottom: '1.5rem', flexShrink: 0 },
  title: { fontSize: '2rem', fontWeight: '700', margin: '0 0 0.25rem 0', background: 'linear-gradient(90deg, #38bdf8, #818cf8)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' },
  subtitle: { color: '#94a3b8', margin: 0, fontSize: '0.95rem' },
  card: { background: '#1e293b', borderRadius: '12px', padding: '1.5rem', width: '100%', maxWidth: '480px', boxShadow: '0 8px 30px rgba(0,0,0,0.4)', border: '1px solid #334155', flexShrink: 0 },
  stepTitle: { margin: '0 0 1rem 0', fontSize: '1.2rem', color: '#e2e8f0' },
  steps: { color: '#cbd5e1', lineHeight: '1.7', paddingLeft: '1.25rem', marginBottom: '1.25rem', fontSize: '0.95rem' },
  primaryBtn: { width: '100%', padding: '0.85rem', background: 'linear-gradient(135deg, #3b82f6, #6366f1)', color: 'white', border: 'none', borderRadius: '10px', fontSize: '1rem', fontWeight: '600', cursor: 'pointer', marginTop: '0.5rem' },
  secondaryBtn: { width: '100%', padding: '0.7rem', background: 'transparent', color: '#94a3b8', border: '1px solid #475569', borderRadius: '8px', fontSize: '0.9rem', cursor: 'pointer', marginTop: '0.75rem' },
  center: { textAlign: 'center', padding: '1rem 0' },
  spinner: { width: '36px', height: '36px', border: '3px solid #334155', borderTop: '3px solid #38bdf8', borderRadius: '50%', animation: 'spin 0.8s linear infinite', margin: '0 auto 1rem' },
  statusText: { fontSize: '1rem', color: '#e2e8f0', margin: '0.5rem 0' },
  hint: { color: '#64748b', fontSize: '0.85rem', marginTop: '0.5rem' },
  infoGrid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', background: '#0f172a', padding: '1rem', borderRadius: '10px', marginBottom: '1.25rem', color: '#cbd5e1', fontSize: '0.95rem' },
  deviceName: { fontSize: '1.3rem', fontWeight: '600', color: '#38bdf8', textAlign: 'center', marginBottom: '1rem', padding: '0.75rem', background: 'linear-gradient(135deg, rgba(56, 189, 248, 0.1), rgba(129, 140, 248, 0.1))', borderRadius: '10px', border: '1px solid rgba(56, 189, 248, 0.3)' }
};

const styleSheet = document.createElement("style");
styleSheet.innerText = `
  html, body { margin: 0; padding: 0; height: 100%; overflow: hidden; }
  #root { height: 100%; }
  @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
  ::-webkit-scrollbar { width: 6px; }
  ::-webkit-scrollbar-track { background: #0f172a; }
  ::-webkit-scrollbar-thumb { background: #334155; border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: #475569; }
`;
document.head.appendChild(styleSheet);

export default App;