import { useState, useEffect, useRef } from 'react';

function App() {
  // --- STATE ---
  const [step, setStep] = useState('guide');
  const [device, setDevice] = useState(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(false);
  const [firmwareStatus, setFirmwareStatus] = useState('idle');
  const [rootState, setRootState] = useState('idle');
  const [rootLog, setRootLog] = useState('');

  // --- REFS ---
  const connectionCheckInterval = useRef(null);
  const detectionInterval = useRef(null);
  const rootLogRef = useRef(null);
  const isRootingRef = useRef(false);

  // --- CONNECTION LOGIC ---
  const checkDeviceConnection = async () => {
    if (isRootingRef.current) {
      return; // Skip during rooting
    }
    
    try {
      await window.goAPI.call('getDeviceInfo', {});
      return true;
    } catch (err) {
      resetToGuide();
      return false;
    }
  };

  const resetToGuide = () => {
    if (connectionCheckInterval.current) {
      clearInterval(connectionCheckInterval.current);
      connectionCheckInterval.current = null;
    }
    if (detectionInterval.current) {
      clearInterval(detectionInterval.current);
      detectionInterval.current = null;
    }
    setStep('guide');
    setDevice(null);
    setError(''); 
    setMessage('');
    setFirmwareStatus('idle');
    setChecking(false);
    setRootLog('');
    setRootState('idle');
    isRootingRef.current = false;
  };

  // --- DETECTION LOGIC ---
  const startDetection = async () => {
    setStep('waiting');
    setError('');
    setMessage('Waiting for device...');
    setChecking(true);

    if (detectionInterval.current) clearInterval(detectionInterval.current);
    if (connectionCheckInterval.current) clearInterval(connectionCheckInterval.current);

    detectionInterval.current = setInterval(async () => {
      try {
        const info = await window.goAPI.call('getDeviceInfo', {});
        setDevice(info);
        setStep('ready');
        setMessage(`✅ Connected: ${info.brand} ${info.displayName || info.model}`);
        setChecking(false);

        setFirmwareStatus('checking');
        const fwRes = await window.goAPI.call('checkFirmware', {
          model: info.model, device: info.device
        });
        setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');

        connectionCheckInterval.current = setInterval(checkDeviceConnection, 3000);
        clearInterval(detectionInterval.current);
      } catch (e) {
        // Still waiting
      }
    }, 2000);

    setTimeout(() => {
      clearInterval(detectionInterval.current);
      if (step === 'waiting') {
        setStep('guide');
        setError('Timeout. No device detected.');
        setChecking(false);
      }
    }, 60000);
  };

  // --- ROOTING LOGIC ---
  const handleRoot = async () => {
    if (!device) return;
    
    // 🔑 SET REF IMMEDIATELY (synchronous)
    isRootingRef.current = true;
    setRootState('rebooting');
    setRootLog(`🔍 Starting root process for ${device.brand}...\n`);
    
    try {
      const fwRes = await window.goAPI.call('checkFirmware', {
        model: device.model, device: device.device
      });
      
      if (!fwRes.available || !fwRes.files || fwRes.files.length === 0) {
        throw new Error('No firmware files found.');
      }

      const firmwareFile = fwRes.files[0];
      const brand = device.brand.toLowerCase();
      
      setRootLog(prev => prev + `✅ Firmware: ${firmwareFile}\n⚡ Preparing...\n`);

      if (brand === 'samsung') {
        setRootLog(prev => prev + '\n📱 Rebooting to Download Mode...');
        await window.goAPI.call('rebootToDownloadMode', { deviceID: device.serial });
        setRootLog(prev => prev + '\n👉 Press Volume UP on phone\n⏳ Waiting...');
        await new Promise(r => setTimeout(r, 15000));
        
        setRootState('flashing');
        setRootLog(prev => prev + '\n🔥 Flashing with Odin...');
        
        const result = await window.goAPI.call('odinFlash', {
          deviceID: device.serial, tarFile: firmwareFile
        });
        
        if (result.success) {
          setRootState('success');
          setRootLog(prev => prev + `\n\n✅ ${result.message}`);
          setDevice(prev => ({ ...prev, rooted: true }));
        } else {
          setRootState('error');
          setRootLog(prev => prev + `\n\n❌ ${result.message}`);
        }
        
      } else if (['oneplus', 'google', 'xiaomi', 'motorola'].includes(brand)) {
        setRootLog(prev => prev + '\n📱 Rebooting to Bootloader...');
        await window.goAPI.call('rebootToBootloader', { deviceID: device.serial });
        setRootLog(prev => prev + '\n⏳ Waiting for Fastboot...');
        await new Promise(r => setTimeout(r, 15000));
        
        setRootState('flashing');
        setRootLog(prev => prev + '\n🔥 Flashing with Fastboot...');
        
        const result = await window.goAPI.call('fastbootFlash', {
          deviceID: device.serial, bootImage: firmwareFile
        });
        
        if (result.success) {
          setRootState('success');
          setRootLog(prev => prev + `\n\n✅ ${result.message}`);
          setDevice(prev => ({ ...prev, rooted: true }));
        } else {
          setRootState('error');
          setRootLog(prev => prev + `\n\n❌ ${result.message}`);
        }
        
      } else {
        throw new Error(`Unsupported brand: ${device.brand}`);
      }
      
    } catch (err) {
      setRootState('error');
      setRootLog(prev => prev + `\n\n❌ Error: ${err.message}`);
    } finally {
      // Reset ref after 15 seconds
      setTimeout(() => {
        isRootingRef.current = false;
      }, 15000);
    }
  };

  // --- AUTO-SCROLL LOG ---
  useEffect(() => {
    if (rootLogRef.current) {
      rootLogRef.current.scrollTop = rootLogRef.current.scrollHeight;
    }
  }, [rootLog]);

  // --- AUTO-CHECK ON STARTUP ---
  useEffect(() => {
    const checkOnStartup = async () => {
      try {
        const info = await window.goAPI.call('getDeviceInfo', {});
        setDevice(info);
        setStep('ready');
        setMessage(`✅ Connected: ${info.brand} ${info.displayName || info.model}`);
        
        setFirmwareStatus('checking');
        const fwRes = await window.goAPI.call('checkFirmware', {
          model: info.model, device: info.device
        });
        setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');
        
        connectionCheckInterval.current = setInterval(checkDeviceConnection, 3000);
      } catch (err) {
        // No device
      }
    };
    
    checkOnStartup();
    
    return () => {
      if (connectionCheckInterval.current) {
        clearInterval(connectionCheckInterval.current);
        connectionCheckInterval.current = null;
      }
    };
  }, []);

  // --- GLOBAL CSS ---
  useEffect(() => {
    const styleSheet = document.createElement("style");
    styleSheet.innerText = `
      html, body, #root { 
        margin: 0; padding: 0; width: 100%; height: 100%; 
        background-color: #0f172a; overflow: hidden; 
      }
      * { box-sizing: border-box; }
      ::-webkit-scrollbar { width: 8px; }
      ::-webkit-scrollbar-track { background: #0f172a; }
      ::-webkit-scrollbar-thumb { background: #334155; border-radius: 4px; }
    `;
    document.head.appendChild(styleSheet);
    return () => { document.head.removeChild(styleSheet); };
  }, []);

  // --- STYLES ---
  const styles = {
    container: { 
      width: '100%', minHeight: '100vh', 
      background: 'linear-gradient(135deg, #0f172a, #1e293b)', 
      color: '#f8fafc', fontFamily: 'system-ui, sans-serif', 
      display: 'flex', flexDirection: 'column', alignItems: 'center',
      padding: '2rem 1rem',
      overflowY: 'auto'
    },
    header: { textAlign: 'center', marginBottom: '2rem', flexShrink: 0 },
    title: { fontSize: '2.2rem', fontWeight: '700', margin: '0 0 0.25rem 0', background: 'linear-gradient(90deg, #38bdf8, #818cf8)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' },
    subtitle: { color: '#94a3b8', margin: 0, fontSize: '1rem' },
    card: { 
      background: '#1e293b', borderRadius: '16px', padding: '2rem', 
      width: '100%', maxWidth: '520px', 
      boxShadow: '0 12px 40px rgba(0,0,0,0.5)', 
      border: '1px solid #334155',
      flexShrink: 0 
    },
    stepTitle: { margin: '0 0 1.25rem 0', fontSize: '1.3rem', color: '#e2e8f0' },
    steps: { color: '#cbd5e1', lineHeight: '1.8', paddingLeft: '1.5rem', marginBottom: '1.5rem', fontSize: '1rem' },
    primaryBtn: { 
      width: '100%', padding: '1rem', 
      background: 'linear-gradient(135deg, #3b82f6, #6366f1)', 
      color: 'white', border: 'none', borderRadius: '12px', 
      fontSize: '1.05rem', fontWeight: '600', cursor: 'pointer', 
      marginTop: '1rem',
      transition: 'all 0.2s'
    },
    secondaryBtn: { 
      width: '100%', padding: '0.85rem', 
      background: 'transparent', color: '#94a3b8', 
      border: '1px solid #475569', borderRadius: '10px', 
      fontSize: '0.95rem', cursor: 'pointer', 
      marginTop: '1rem',
      transition: 'all 0.2s'
    },
    dangerBtn: {
      width: '100%', padding: '0.85rem',
      background: '#dc2626', color: 'white',
      border: 'none', borderRadius: '10px',
      fontSize: '0.95rem', fontWeight: '600',
      cursor: 'pointer', marginTop: '1rem'
    },
    center: { textAlign: 'center', padding: '2rem 0' },
    spinner: { width: '40px', height: '40px', border: '3px solid #334155', borderTop: '3px solid #38bdf8', borderRadius: '50%', animation: 'spin 0.8s linear infinite', margin: '0 auto 1.25rem' },
    statusText: { fontSize: '1.05rem', color: '#e2e8f0', margin: '0.5rem 0' },
    infoGrid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', background: '#0f172a', padding: '1.25rem', borderRadius: '12px', marginBottom: '1.5rem', color: '#cbd5e1', fontSize: '1rem' },
    deviceName: { fontSize: '1.4rem', fontWeight: '600', color: '#38bdf8', textAlign: 'center', marginBottom: '1.25rem', padding: '0.85rem', background: 'linear-gradient(135deg, rgba(56, 189, 248, 0.1), rgba(129, 140, 248, 0.1))', borderRadius: '12px', border: '1px solid rgba(56, 189, 248, 0.3)' },
    logContainer: {
      marginTop: '1rem',
      background: '#0f172a',
      borderRadius: '10px',
      border: '1px solid #334155',
      overflow: 'hidden'
    },
    logHeader: {
      padding: '0.75rem 1rem',
      background: '#1e293b',
      borderBottom: '1px solid #334155',
      fontSize: '0.9rem',
      fontWeight: '600',
      color: '#94a3b8'
    },
    logContent: {
      padding: '1rem',
      fontFamily: 'monospace',
      fontSize: '0.85rem',
      color: '#22c55e',
      whiteSpace: 'pre-line',
      maxHeight: '250px',
      overflowY: 'auto',
      lineHeight: '1.6',
      minHeight: '100px'
    }
  };

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
            <button onClick={startDetection} disabled={checking} style={styles.primaryBtn}>
              {checking ? '⏳ Waiting...' : '✅ I\'ve Enabled USB Debugging'}
            </button>
          </>
        )}

        {step === 'waiting' && (
          <div style={styles.center}>
            <div style={styles.spinner}></div>
            <p style={styles.statusText}>{message}</p>
            {error && <p style={{ color: '#ef4444', marginTop: '0.5rem' }}>{error}</p>}
          </div>
        )}

        {step === 'ready' && device && (
          <>
            <h2 style={styles.stepTitle}>Device Connected</h2>
            <div style={styles.deviceName}>
              {device.displayName || `${device.brand} ${device.model}`}
            </div>
            <div style={styles.infoGrid}>
              <div><b>Brand:</b> {device.brand}</div>
              <div><b>Model:</b> {device.model}</div>
              <div><b>Android:</b> {device.androidVersion}</div>
              <div><b>Rooted:</b> {device.rooted ? 'Yes ✓' : 'No'}</div>
            </div>

            {!device.rooted && (
              <>
                {firmwareStatus === 'unavailable' && (
                  <div style={{ textAlign: 'center', marginTop: '1rem', padding: '1rem', background: '#451a1a', borderRadius: '10px' }}>
                    <p style={{ color: '#fca5a5', fontSize: '0.9rem', margin: 0 }}>
                      Firmware not found. Re-run installer or download firmware manually from <a href="https://firmwarefile.com" target="_blank" rel="noopener noreferrer" style={{ color: '#f87171', textDecoration: 'underline' }}>firmwarefile.com</a> and place it in the firmware directory and rename it to "{device.model}_{device.androidVersion}_firmware".
                    </p>
                  </div>
                )}

                {firmwareStatus === 'available' && rootState === 'idle' && (
                  <button 
                    onClick={handleRoot}
                    style={styles.primaryBtn}
                  >
                    🔥 Root {device.brand}
                  </button>
                )}

                {/* 🔑 LOG CONTAINER - Always visible, scrollable content */}
                {rootState !== 'idle' && (
                  <div style={styles.logContainer}>
                    <div style={styles.logHeader}>
                      {rootState === 'rebooting' && '🔄 Rebooting Device...'}
                      {rootState === 'flashing' && '⚡ Flashing Firmware...'}
                      {rootState === 'success' && '✅ Success!'}
                      {rootState === 'error' && '❌ Failed'}
                    </div>
                    <div 
                      ref={rootLogRef}
                      style={{
                        ...styles.logContent,
                        color: rootState === 'error' ? '#ef4444' : '#22c55e'
                      }}
                    >
                      {rootLog}
                    </div>
                  </div>
                )}
                
                {/* 🔑 CANCEL/DONE BUTTONS - Always accessible */}
                {rootState === 'rebooting' && (
                  <button 
                    onClick={() => {
                      isRootingRef.current = false;
                      setRootState('idle');
                      setRootLog('');
                    }}
                    style={styles.dangerBtn}
                  >
                    🚫 Cancel Root
                  </button>
                )}
                
                {rootState === 'error' && (
                  <button 
                    onClick={() => {
                      isRootingRef.current = false;
                      setRootState('idle');
                      setRootLog('');
                    }}
                    style={styles.secondaryBtn}
                  >
                    ↺ Try Again
                  </button>
                )}
                
                {rootState === 'success' && (
                  <button 
                    onClick={() => {
                      isRootingRef.current = false;
                      setRootState('idle');
                      setRootLog('');
                    }}
                    style={styles.primaryBtn}
                  >
                    ✓ Done
                  </button>
                )}
              </>
            )}

            {device.rooted && (
              <div style={{ marginTop: '1.25rem', textAlign: 'center', color: '#22c55e', fontSize: '1.1rem', fontWeight: '500' }}>
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