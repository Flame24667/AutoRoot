import { useState } from 'react';
import { useFlashStore, useDeviceStore, useAssetStore } from '../store';

export function FlashControl() {
  const [selectedFile, setSelectedFile] = useState(null);
  const { 
    isFlashing, 
    progress, 
    status, 
    logs, 
    error,
    startFlash, 
    updateProgress, 
    completeFlash,
    addLog 
  } = useFlashStore();
  
  const { connected, model, protocol } = useDeviceStore();
  const { findAssetForModel } = useAssetStore();

  const handleStartFlash = async () => {
    if (!connected) {
      alert('No device connected!');
      return;
    }

    // Auto-find asset for detected model
    const asset = selectedFile || findAssetForModel(model);
    
    if (!asset) {
      alert('No compatible firmware found for this device!');
      return;
    }

    startFlash();
    addLog(`Starting flash process for ${model}`);
    addLog(`Using file: ${asset.path}`);
    addLog(`Protocol: ${protocol}`);

    try {
      // Simulate flash process (will be replaced with actual backend call)
      updateProgress(10, 'preparing', 'Preparing device...');
      
      if (window.electronAPI) {
        const result = await window.electronAPI.startFlash({
          filePath: asset.path,
          protocol,
          model
        });
        
        if (result.success) {
          updateProgress(100, 'completed', 'Flash completed successfully!');
          completeFlash(true, null);
        } else {
          updateProgress(0, 'failed', result.message);
          completeFlash(false, result.message);
        }
      } else {
        // Fallback simulation for development
        simulateFlash();
      }
    } catch (err) {
      updateProgress(0, 'failed', err.message);
      completeFlash(false, err.message);
    }
  };

  const simulateFlash = () => {
    // Simulation for demo purposes
    let progress = 0;
    const interval = setInterval(() => {
      progress += 5;
      if (progress <= 100) {
        let stage = 'flashing';
        if (progress < 20) stage = 'preparing';
        if (progress > 80) stage = 'verifying';
        
        updateProgress(progress, stage, `Progress: ${progress}%`);
      } else {
        clearInterval(interval);
        completeFlash(true, null);
      }
    }, 200);
  };

  const getStatusColor = () => {
    switch (status) {
      case 'completed': return '#4caf50';
      case 'failed': return '#f44336';
      case 'flashing': return '#2196f3';
      default: return '#757575';
    }
  };

  return (
    <div className="flash-control">
      <h3>Flash Control</h3>
      
      <div className="flash-status">
        <div 
          className="progress-bar"
          style={{ 
            width: `${progress}%`,
            backgroundColor: getStatusColor()
          }}
        />
        <span className="progress-text">{status.toUpperCase()} - {progress}%</span>
      </div>

      {error && (
        <div className="error-message">
          ⚠ {error}
        </div>
      )}

      <div className="flash-actions">
        <button
          onClick={handleStartFlash}
          disabled={!connected || isFlashing}
          className="btn-primary"
        >
          {isFlashing ? 'Flashing...' : 'Start Root'}
        </button>
        
        <label className="btn-secondary">
          Select Custom File
          <input
            type="file"
            onChange={(e) => {
              const file = e.target.files[0];
              if (file) {
                setSelectedFile({ path: file.path, name: file.name });
                addLog(`Selected custom file: ${file.name}`);
              }
            }}
            accept=".tar,.img,.zip"
            style={{ display: 'none' }}
          />
        </label>
      </div>

      <TerminalLog logs={logs} />
    </div>
  );
}

// Terminal Log View Component
export function TerminalLog({ logs }) {
  const logRef = useState(null);

  return (
    <div className="terminal-log">
      <h4>Terminal Log</h4>
      <div className="log-container" ref={logRef}>
        {logs.length === 0 ? (
          <p className="empty-log">Waiting for process...</p>
        ) : (
          logs.map((log, idx) => (
            <div key={idx} className="log-entry">{log}</div>
          ))
        )}
      </div>
    </div>
  );
}
