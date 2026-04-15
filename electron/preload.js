const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electronAPI', {
  // Device Detection
  getDeviceInfo: () => ipcRenderer.invoke('get-device-info'),
  
  // Flashing Operations
  startFlash: (config) => ipcRenderer.invoke('start-flash', config),
  
  // Driver Check
  checkDrivers: () => ipcRenderer.invoke('check-drivers'),
  
  // Asset Management
  getAssets: () => ipcRenderer.invoke('get-assets'),
  
  // Listen for real-time updates from main process
  onDeviceChange: (callback) => {
    ipcRenderer.on('device-changed', (event, data) => callback(data));
  },
  
  onFlashProgress: (callback) => {
    ipcRenderer.on('flash-progress', (event, data) => callback(data));
  },
  
  removeDeviceChangeListener: () => {
    ipcRenderer.removeAllListeners('device-changed');
  },
  
  removeFlashProgressListener: () => {
    ipcRenderer.removeAllListeners('flash-progress');
  }
});
