import { useEffect } from 'react';
import { useDeviceStore, useDriverStore, useAssetStore } from '../store';

// USB Device Monitor Component
export function DeviceMonitor() {
  const setDevice = useDeviceStore((state) => state.setDevice);
  const clearDevice = useDeviceStore((state) => state.clearDevice);
  const setDriverStatus = useDriverStore((state) => state.setDriverStatus);
  const setAssets = useAssetStore((state) => state.setAssets);

  useEffect(() => {
    // Check drivers on mount
    checkDrivers();
    
    // Load assets from local warehouse
    loadAssets();

    // Set up device change listener
    if (window.electronAPI) {
      window.electronAPI.onDeviceChange((data) => {
        if (data.connected) {
          setDevice(data);
        } else {
          clearDevice();
        }
      });

      return () => {
        window.electronAPI.removeDeviceChangeListener();
      };
    }
  }, []);

  const checkDrivers = async () => {
    if (window.electronAPI) {
      const result = await window.electronAPI.checkDrivers();
      setDriverStatus(result.installed, result.missing || []);
    }
  };

  const loadAssets = async () => {
    if (window.electronAPI) {
      const assets = await window.electronAPI.getAssets();
      setAssets(assets);
    }
  };

  return null; // This is a logic-only component
}

// Device Status Display
export function DeviceStatus() {
  const { connected, model, protocol, vendorId, productId } = useDeviceStore();

  return (
    <div className="device-status">
      <h3>Device Status</h3>
      <div className={`status-indicator ${connected ? 'connected' : 'disconnected'}`}>
        {connected ? '● Connected' : '○ Disconnected'}
      </div>
      {connected && (
        <div className="device-info">
          <p><strong>Model:</strong> {model || 'Unknown'}</p>
          <p><strong>Protocol:</strong> {protocol || 'Detecting...'}</p>
          <p><strong>VID:PID:</strong> {vendorId?.toString(16)}:{productId?.toString(16)}</p>
        </div>
      )}
    </div>
  );
}

// Driver Health Check Display
export function DriverHealth() {
  const { driversInstalled, missingDrivers, checked } = useDriverStore();

  if (!checked) return null;

  return (
    <div className="driver-health">
      <h3>Driver Status</h3>
      {driversInstalled ? (
        <p className="success">✓ All required drivers installed</p>
      ) : (
        <div className="warning">
          <p>⚠ Missing drivers:</p>
          <ul>
            {missingDrivers.map((driver, idx) => (
              <li key={idx}>{driver}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
