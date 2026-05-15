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
  const [isDragging, setIsDragging] = useState(false);
  const [dropMsg, setDropMsg] = useState('');
  const [patchStep, setPatchStep] = useState('idle'); // 'idle', 'waiting', 'pulling'
  const [hasError, setHasError] = useState(false);

  // --- REFS ---
  const connectionCheckInterval = useRef(null);
  const detectionInterval = useRef(null);
  const rootLogRef = useRef(null);
  const isRootingRef = useRef(false);
  const fileInputRef = useRef(null);

  const safeGetFileName = (filePath) => {
    try {
      if (!filePath) return 'Unknown';
      return filePath.split(/[\\/]/).pop();
    } catch (e) {
      console.error('getFileName error:', e);
      return 'Error';
    }
  };

  // --- CONNECTION LOGIC ---
  const checkDeviceConnection = async () => {
    if (isRootingRef.current || rootState !== 'idle') {
      return;
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
    if (isRootingRef.current) {
      return;
    }
    
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

  // --- FILE HANDLERS ---
  const handleDragOver = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleFileSelect = async () => {
    if (!window.electronAPI?.selectFirmware) {
      alert('️ App config error: File picker bridge missing. Restart the app.');
      return;
    }

    setDropMsg('📂 Opening file picker...');
    try {
      const filePath = await window.electronAPI.selectFirmware();
      if (!filePath) {
        setDropMsg('❌ Cancelled');
        setTimeout(() => setDropMsg(''), 3000);
        return;
      }

      setDropMsg('📦 Processing...');
      const res = await window.goAPI.call('handleDroppedFirmware', { filePath });
      
      if (res?.success) {
        setDropMsg('✅ Firmware added!');
        setFirmwareStatus('available');
      } else {
        setDropMsg(`❌ ${res?.error || 'Failed'}`);
      }
    } catch (err) {
      setDropMsg(`❌ ${err.message}`);
    }
    setTimeout(() => setDropMsg(''), 5000);
  };

  const handleDrop = async (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    
    const zips = Array.from(e.dataTransfer.files).filter(f => f.name.toLowerCase().endsWith('.zip'));
    if (zips.length === 0) {
      setDropMsg('❌ Only .zip files');
      setTimeout(() => setDropMsg(''), 3000);
      return;
    }

    setDropMsg('📦 Processing...');
    for (const f of zips) {
      if (!f.path) {
        console.warn('⚠️ Drag & drop blocked: file.path missing. Use "Select Firmware" button.');
        continue;
      }
      try {
        const res = await window.goAPI.call('handleDroppedFirmware', { filePath: f.path });
        if (res?.success) setDropMsg('✅ Added!');
      } catch (err) {
        console.error('Drop error:', err);
      }
    }
    setTimeout(() => setDropMsg(''), 5000);
  };

  const processFirmwareFiles = async (files) => {
    let successCount = 0;
    let errors = [];

    for (const file of files) {
      try {
        console.log('📂 Sending to backend:', file.path);
        const result = await window.goAPI.call('handleDroppedFirmware', { filePath: file.path });
        
        if (result?.success) {
          successCount++;
        } else {
          errors.push(result?.error || 'Backend returned failure');
        }
      } catch (err) {
        console.error('❌ File processing failed:', err);
        errors.push(err.message || 'Unknown error');
      }
    }

    if (successCount > 0) {
      setDropMsg(`✅ Successfully added ${successCount} file(s)! Checking firmware...`);
      
      // Force refresh firmware status
      if (device) {
        setTimeout(async () => {
          try {
            const fwRes = await window.goAPI.call('checkFirmware', { 
              model: device.model, 
              device: device.device 
            });
            setFirmwareStatus(fwRes.available ? 'available' : 'unavailable');
            setDropMsg(`✅ Firmware updated & ready to root!`);
          } catch (err) {
            setFirmwareStatus('available');
            setDropMsg(`✅ File added successfully!`);
          }
        }, 600);
      } else {
        setFirmwareStatus('available');
        setDropMsg(`✅ File added successfully!`);
      }
    } else {
      setDropMsg(`❌ Failed to add files: ${errors.join(', ')}`);
    }

    // Clear message after 5 seconds
    setTimeout(() => setDropMsg(''), 5000);
  };

  const getFileName = (filePath) => {
    if (!filePath) return 'Unknown';
    return filePath.split(/[\\/]/).pop();
  };

  const handleRoot = async () => {
    if (!device) return;
    
    isRootingRef.current = true;
    setRootState('rebooting');
    setRootLog(`🔍 Starting root process for ${device.brand}...\n`);
    
    try {
      const fwRes = await window.goAPI.call('checkFirmware', {
        model: device.model, device: device.device
      });
      
      if (!fwRes.available || !fwRes.files || fwRes.files.length === 0) {
        throw new Error('No firmware files found. Please download or select firmware first.');
      }

      const brand = device.brand.toLowerCase();
      
      if (brand === 'samsung') {
        await handleSamsungRoot(fwRes.files);
      } else if (['oneplus', 'google', 'xiaomi', 'motorola', 'nothing'].includes(brand)) {
        await handleFastbootRoot(fwRes.files);
      } else {
        throw new Error(`Rooting not yet supported for ${device.brand} devices.`);
      }
      
    } catch (err) {
      setRootState('error');
      setRootLog(prev => prev + `\n\n❌ Error: ${err.message}`);
    } finally {
      setTimeout(() => { isRootingRef.current = false; }, 15000);
    }
  };

  // 🔑 SAMSUNG ODIN ROOTING
  const handleSamsungRoot = async (files) => {
    setHasError(false);
    setRootLog(prev => prev + '\n📱 Samsung Odin Rooting Process\n');
    
    try {
      // 🔑 Keep phone awake
      await window.goAPI.call('keepDeviceAwake', { deviceID: device.serial });
      setRootLog(prev => prev + ' Screen will stay awake during process\n');

      // 🔑 Auto-install Magisk if missing
      setRootLog(prev => prev + '📲 Checking for Magisk app...\n');
      const magiskRes = await window.goAPI.call('ensureMagiskInstalled', { deviceID: device.serial });
      if (magiskRes.message) setRootLog(prev => prev + `✅ ${magiskRes.message}\n`);
      
      // Extract if zip
      let firmwareFiles = [...files];
      const zipFile = firmwareFiles.find(f => f.toLowerCase().endsWith('.zip'));
      if (zipFile) {
        setRootLog(prev => prev + '📦 Extracting firmware (large AP file may take 2-5 mins)...\n');
        const extractResult = await window.goAPI.call('extractFirmwareToFolder', {
          zipFile, brand: device.brand, model: device.model,
          version: device.buildVersion || '', androidVersion: device.androidVersion || '',
          binaryBit: device.binaryBit || 'N/A'
        });
        if (!extractResult?.success) throw new Error(`Extraction failed: ${extractResult?.error}`);
        firmwareFiles = extractResult.files || [];
        setRootLog(prev => prev + `✅ Extracted ${firmwareFiles.length} files\n`);
      }

      // Find files
      const apFile = firmwareFiles.find(f => f.includes('AP_') && f.endsWith('.tar.md5'));
      const blFile = firmwareFiles.find(f => f.includes('BL_') && f.endsWith('.tar.md5'));
      const cpFile = firmwareFiles.find(f => f.includes('CP_') && f.endsWith('.tar.md5'));
      const cscFile = firmwareFiles.find(f => f.includes('CSC_') && !f.includes('HOME') && f.endsWith('.tar.md5'));
      
      if (!apFile || !blFile || !cpFile || !cscFile) {
        throw new Error(`Missing: ${!apFile?'AP ':''}${!blFile?'BL ':''}${!cpFile?'CP ':''}${!cscFile?'CSC':''}`);
      }
      
      setRootLog(prev => prev + `✅ Found all firmware files\n\n`);

      // Transfer AP
      setRootLog(prev => prev + '📤 Transferring AP to phone...\n');
      const transferRes = await window.goAPI.call('transferFileToDevice', {
        filePath: apFile, destination: '/sdcard/Download/AP_file.tar'
      });
      if (!transferRes?.success) throw new Error('Transfer failed. Check USB connection.');

      setRootLog(prev => prev + 
        '✅ AP transferred\n\n' +
        '📲 ON PHONE:\n' +
        '1. Open Magisk → Install → Select & Patch\n' +
        '2. Choose "AP_file.tar"\n' +
        '3. Wait for "All done!"\n\n' +
        '⏳ Tap button below when ready →'
      );
      setPatchStep('waiting');

    } catch (err) {
      setHasError(true);
      setRootState('error');
      setRootLog(prev => prev + `\n\n❌ ${err.message}`);
      console.error('Samsung root error:', err);
    }
  };

  // 🔑 Called when user clicks "I'm Done Patching"
  const handleContinueAfterPatch = async () => {
    setPatchStep('pulling');
    setRootLog(prev => prev + '\n🔍 Pulling patched file...\n');
    
    try {
      const pullRes = await window.goAPI.call('getLatestMagiskPatchedFile', {
        deviceID: device.serial
      });
      
      if (!pullRes?.success) {
        throw new Error('Patched file not found. Did you patch it? Try again.');
      }

      const patchedAp = pullRes.localPath;
      
      // 🔑 Clear the pulling message by updating log
      setRootLog(prev => prev.replace('🔍 Pulling patched file...\n', '') + 
        `✅ Patched AP received: ${getFileName(pullRes.source)}\n\n🔥 Preparing Odin flash...\n`
      );
      
      // 🔑 Reset patchStep immediately after pull completes
      setPatchStep('idle');
      
      setRootState('flashing');
      setRootLog(prev => prev + '📱 Rebooting to Download Mode...\n');
      await window.goAPI.call('rebootToDownloadMode', { deviceID: device.serial });
      
      setRootLog(prev => prev + '\n👉 Press Volume UP on phone\n⏳ Waiting...\n');
      await new Promise(r => setTimeout(r, 15000));
      
      setRootLog(prev => prev + '\n🔥 Flashing with Odin...\nDO NOT DISCONNECT!\n');
      const odinRes = await window.goAPI.call('odinFlash', {
        deviceID: device.serial,
        apFile: patchedAp,
        blFile: firmwareFiles.find(f=>f.includes('BL_')),
        cpFile: firmwareFiles.find(f=>f.includes('CP_')),
        cscFile: firmwareFiles.find(f=>f.includes('CSC_')&&!f.includes('HOME'))
      });
      
      if (odinRes?.success) {
        setRootState('success');
        setRootLog(prev => prev + `\n\n✅ ${odinRes.message}\nDevice will reboot.`);
        setDevice(prev => ({...prev, rooted: true}));
      } else {
        setRootState('error');
        setRootLog(prev => prev + `\n\n❌ ${odinRes.message || 'Odin failed'}`);
      }
    } catch (err) {
      setHasError(true);
      setRootState('error');
      setPatchStep('idle'); // 🔑 Reset on error too
      setRootLog(prev => prev + `\n\n❌ ${err.message}`);
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
              // No device on startup
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
      @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
      }
      html, body, #root { 
        margin: 0; padding: 0; width: 100%; height: 100%; 
        background-color: #0f172a; overflow: hidden; 
      }
      * { box-sizing: border-box; }
      ::-webkit-scrollbar { width: 8px; }
      ::-webkit-scrollbar-track { background: #0f172a; }
      ::-webkit-scrollbar-thumb { background: #334155; border-radius: 4px; }
      @keyframes pulse {
        0%, 100% { transform: scale(1); }
        50% { transform: scale(1.02); }
      }
      @keyframes fadeIn {
        from { opacity: 0; transform: translate(-50%, 10px); }
        to { opacity: 1; transform: translate(-50%, 0); }
      }
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
      overflowY: 'auto',
      position: 'relative'
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
    spinner: { 
      width: '40px', height: '40px', 
      border: '3px solid #334155', 
      borderTop: '3px solid #38bdf8', 
      borderRadius: '50%', 
      animation: 'spin 0.8s linear infinite', 
      margin: '0 auto 1.25rem' 
    },
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
    },
    dropOverlay: {
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      background: 'rgba(15, 23, 42, 0.9)', zIndex: 1000,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      color: '#38bdf8', fontSize: '1.5rem', fontWeight: '600',
      border: '4px dashed #38bdf8', pointerEvents: 'none'
    }
  };

  // --- RENDER ---
  return (
    <div 
      style={styles.container} 
      onDragOver={handleDragOver} 
      onDragLeave={handleDragLeave} 
      onDrop={handleDrop}
    >
      {/* Drop Overlay */}
      {isDragging && (
        <div style={styles.dropOverlay}>
          📥 Drop Firmware .zip Here
        </div>
      )}

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
              <div><b>Version:</b> {device.buildVersion || "N/A"}</div>
              <div><b>Binary/Bit:</b> {device.binaryBit || "N/A"}</div>
              <div><b>Android:</b> {device.androidVersion}</div>
              <div><b>Rooted:</b> {device.rooted ? 'Yes ✓' : 'No'}</div>
            </div>

            {!device.rooted && (
              <>
                {firmwareStatus === 'unavailable' && (
                  <div style={{ 
                    textAlign: 'center', 
                    marginTop: '1rem', 
                    padding: '1.25rem', 
                    background: '#1e293b', 
                    borderRadius: '12px',
                    border: '2px dashed #475569'
                  }}>
                    <p style={{ color: '#94a3b8', fontSize: '0.9rem', marginBottom: '1rem' }}>
                      📁 No firmware found for this device. Download the file and rename it to {device.brand}_{device.model}_{device.buildVersion}_{device.androidVersion}_{device.binaryBit}.zip
                    </p>
                    
                    {/* 🔑 NATIVE DIALOG BUTTON */}
                    <button 
                      onClick={handleFileSelect}
                      style={{
                        ...styles.primaryBtn,
                        background: 'linear-gradient(135deg, #8b5cf6, #6366f1)',
                        marginBottom: '0.75rem'
                      }}
                    >
                      📂 Select Firmware File
                    </button>
                    
                    <p style={{ color: '#64748b', fontSize: '0.8rem', margin: '0.5rem 0 0 0' }}>
                      Drag & drop may be blocked by Electron security
                    </p>
                  </div>
                )}

                {firmwareStatus === 'available' && rootState === 'idle' && (
                  <button 
                    onClick={handleRoot}
                    style={{
                      ...styles.primaryBtn,
                      background: 'linear-gradient(135deg, #10b981, #059669)',
                      marginTop: '1rem'
                    }}
                  >
                    🔥 Root {device.brand}
                  </button>
                )}

                {/* Log Container */}
                {rootState !== 'idle' && (
                  <div style={styles.logContainer}>
                    <div style={styles.logHeader}>
                      {rootState === 'rebooting' && '🔄 Rooting Device...'}
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
                {patchStep === 'waiting' && (
                  <button 
                    onClick={handleContinueAfterPatch}
                    style={{
                      ...styles.primaryBtn,
                      background: 'linear-gradient(135deg, #f59e0b, #d97706)',
                      marginTop: '1rem',
                      animation: 'pulse 1.5s infinite'
                    }}
                  >
                    ✅ I've Patched the File, Continue
                  </button>
                )}

                {patchStep === 'pulling' && (
                  <div style={{textAlign:'center', marginTop:'1rem', color:'#94a3b8'}}>
                    ⏳ Pulling patched file from phone...
                  </div>
                )}
                {/* Action Buttons */}
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

      {/* Drop Message Toast */}
      {dropMsg && (
        <div style={{
          position: 'fixed', 
          bottom: '2rem', 
          left: '50%', 
          transform: 'translateX(-50%)',
          padding: '1rem 1.5rem', 
          background: dropMsg.includes('✅') ? '#065f46' : dropMsg.includes('') ? '#7f1d1d' : '#1e293b',
          color: '#f8fafc',
          borderRadius: '12px',
          border: `1px solid ${dropMsg.includes('✅') ? '#22c55e' : dropMsg.includes('❌') ? '#ef4444' : '#475569'}`,
          fontSize: '0.95rem', 
          fontWeight: '500',
          zIndex: 9999,
          boxShadow: '0 8px 30px rgba(0,0,0,0.4)',
          textAlign: 'center',
          minWidth: '280px',
          animation: 'fadeIn 0.3s ease'
        }}>
          {dropMsg}
        </div>
      )}
    </div>
  );
}

export default App;