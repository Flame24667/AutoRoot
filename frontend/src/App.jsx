import { useState, useEffect, useRef } from 'react';

function App() {
  // --- STATE ---
  const [step, setStep] = useState('guide');
  const [device, setDevice] = useState(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(false);
  
  // Firmware States
  const [firmwareStatus, setFirmwareStatus] = useState('idle'); // idle | checking | available | unavailable
  
  // Odin Rooting States
  const [odinState, setOdinState] = useState('idle'); // idle | rebooting | flashing | success | error
  const [odinLog, setOdinLog] = useState('');

  // --- REFS ---
  const connectionCheckInterval = useRef(null);

  // --- CONNECTION LOGIC ---
  const resetToGuide = () => {
    if (connectionCheckInterval.current) {
      clearInterval(connectionCheckInterval.current);
      connectionCheckInterval.current = null;
    }
    setStep('guide');
    setDevice(null);
    setError('');
    setMessage('');
    setFirmwareStatus('idle');
    setChecking(false);
    setOdinState('idle');
    setOdinLog('');
  };

  const checkDeviceConnection = async () => {
    try {
      const info = await window.goAPI.call('getDeviceInfo', {});
      return true;
    } catch (err) {
      resetToGuide();
      return false;
    }
  };

  // Auto-check connection on startup
  useEffect(() => {
    const checkInitialConnection = async () => {
      try {
        const info = await window.goAPI.call('getDeviceInfo', {});
        setDevice(info);
        setStep('ready');
        setMessage(`✅ Connected: ${info.brand} ${info.displayName || info.model}`);
        
        // Check firmware availability
        setFirmwareStatus('checking');
        const fwRes = await window.goAPI.call('checkFirmware', {
          model: info.model,
          device: info.device
        });
        setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');
        
        // Start polling for disconnection
        connectionCheckInterval.current = setInterval(checkDeviceConnection, 3000);
      } catch (err) {
        // No device connected on startup
      }
    };
    checkInitialConnection();

    return () => {
      if (connectionCheckInterval.current) clearInterval(connectionCheckInterval.current);
    };
  }, []);

  // --- UI BUTTONS LOGIC ---
  const startDetection = async () => {
    setStep('waiting');
    setMessage('Waiting for device...');
    setChecking(true);
    
    // Polling logic is same as initial check...
    const interval = setInterval(async () => {
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
        
        clearInterval(interval);
        setChecking(false);
        connectionCheckInterval.current = setInterval(checkDeviceConnection, 3000);
      } catch (e) {
        // still waiting
      }
    }, 2000);
  };

  const checkConnection = async () => {
    setChecking(true);
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
    } finally {
      setChecking(false);
    }
  };

  // --- ODIN ROOTING LOGIC ---
  const handleOdinRoot = async () => {
    if (!device || !device.model) return;
    
    setOdinState('rebooting');
    setOdinLog('🔍 Locating firmware file...');
    
    try {
      // 1. Check Firmware
      const fwRes = await window.goAPI.call('checkFirmware', {
        model: device.model,
        device: device.device
      });
      
      if (!fwRes.available || !fwRes.files || fwRes.files.length === 0) {
        throw new Error('No firmware files found. Please download firmware first.');
      }
      
      const tarFile = fwRes.files.find(f => f.endsWith('.tar') || f.endsWith('.tar.md5'));
      if (!tarFile) {
        throw new Error('No compatible .tar firmware file found.');
      }
      
      setOdinLog(`📦 Found: ${tarFile}\n⚡ Rebooting to Download Mode...`);
      
      // 2. Reboot
      await window.goAPI.call('rebootToDownloadMode', { deviceID: device.device });
      
      setOdinLog(`📱 Device rebooting...\n\n👉 On your phone:\n1. Press Volume UP to continue\n2. Wait for Odin connection`);
      
      await new Promise(resolve => setTimeout(resolve, 10000)); // Wait 10s for user
      
      // 3. Flash
      setOdinState('flashing');
      setOdinLog(`🔥 Flashing firmware with Odin...\n⚠️ DO NOT DISCONNECT DEVICE!`);
      
      const result = await window.goAPI.call('odinFlash', {
        deviceID: device.device,
        tarFile: tarFile // Note: In production, pass full path or handle in backend
      });
      
      if (result.success) {
        setOdinState('success');
        setOdinLog(`✅ ${result.message}\n\nYour device is now rooted!`);
        setDevice(prev => ({ ...prev, rooted: true }));
      } else {
        setOdinState('error');
        setOdinLog(`❌ ${result.message}\n\n${result.log ? 'Log:\n' + result.log.substring(0, 500) : ''}`);
      }
      
    } catch (err) {
      setOdinState('error');
      setOdinLog(`❌ Error: ${err.message}`);
    }
  };

  // --- STYLES ---
  const styles = {
    container: { height: '100vh', overflowY: 'auto', background: 'linear-gradient(135deg, #0f172a, #1e293b)', color: '#f8fafc', fontFamily: 'system-ui, sans-serif', padding: '1.5rem', display: 'flex', flexDirection: 'column', alignItems: 'center', boxSizing: 'border-box' },
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
  `;
  document.head.appendChild(styleSheet);

  // --- RENDER ---
  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>🔓 AutoRoot</h1>
        <p style={styles.subtitle}>Secure Android Root Automation</p>
      </header>

      <div style={styles.card}>
        {step === 'guide' && (
          <>
            <h2 style={styles.stepTitle}>Step 1: Enable USB Debugging</h2>
            <ol style={styles.steps}>
              <li>Open <b>Settings</b> → <b>About Phone</b></li>
              <li>Tap <b>Build Number</b> 7 times</li>
              <li>Go back → <b>Developer Options</b></li>
              <li>Toggle <b>USB Debugging</b> ON</li>
              <li>Connect USB & tap <b>"Allow"</b></li>
            </ol>
            <button onClick={startDetection} style={styles.primaryBtn}>✅ I've Enabled USB Debugging</button>
            <button onClick={checkConnection} style={styles.secondaryBtn} disabled={checking}>
              {checking ? 'Checking...' : '🔍 Manual Check Connection'}
            </button>
          </>
        )}

        {step === 'waiting' && (
          <div style={styles.center}>
            <div style={styles.spinner}></div>
            <p style={styles.statusText}>{message}</p>
          </div>
        )}

        {step === 'ready' && device && (
          <>
            <h2 style={styles.stepTitle}>Device Connected</h2>
            <div style={styles.deviceName}>
              {device.displayName && device.displayName !== device.model
                ? device.displayName
                : `${device.brand} ${device.model}`}
            </div>
            <div style={styles.infoGrid}>
              <div><b>Brand:</b> {device.brand}</div>
              <div><b>Model:</b> {device.model}</div>
              <div><b>Android:</b> {device.androidVersion}</div>
              <div><b>Rooted:</b> {device.rooted ? 'Yes ✓' : 'No'}</div>
            </div>

            {/* --- ROOTING UI --- */}
            {!device.rooted && (
              <>
                {firmwareStatus === 'unavailable' && (
                  <div style={{ textAlign: 'center', marginTop: '1rem', padding: '1rem', background: '#451a1a', borderRadius: '8px' }}>
                    <p style={{ color: '#fca5a5', fontSize: '0.85rem', margin: 0 }}>
                      Firmware not found. Re-run installer.
                    </p>
                  </div>
                )}

                {firmwareStatus === 'available' && device.brand.toLowerCase() === 'samsung' && (
                  <button 
                    onClick={handleOdinRoot}
                    disabled={odinState !== 'idle'}
                    style={{
                      ...styles.primaryBtn,
                      background: odinState === 'flashing' ? '#f59e0b' : 'linear-gradient(135deg, #1428a0, #1a3cd1)',
                      marginTop: '1rem'
                    }}
                  >
                    {odinState === 'idle' ? '🔥 Root with Odin' : 
                     odinState === 'rebooting' ? '⏳ Rebooting...' : 
                     odinState === 'flashing' ? '⚡ Flashing...' : 
                     odinState === 'success' ? '✅ Rooted!' : '❌ Failed'}
                  </button>
                )}

                {(odinState === 'rebooting' || odinState === 'flashing' || odinState === 'success' || odinState === 'error') && (
                  <div style={{
                    marginTop: '1rem', padding: '1rem', background: '#0f172a', borderRadius: '8px',
                    fontFamily: 'monospace', fontSize: '0.85rem', color: odinState === 'error' ? '#ef4444' : '#22c55e',
                    whiteSpace: 'pre-line', maxHeight: '200px', overflowY: 'auto'
                  }}>
                    {odinLog}
                  </div>
                )}
              </>
            )}

            {device.rooted && (
              <div style={{ marginTop: '1rem', textAlign: 'center', color: '#22c55e' }}>
                ✅ This device is already rooted!
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

export default App;