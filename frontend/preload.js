const { contextBridge, ipcRenderer } = require('electron');

// 🔑 1. Go Backend Bridge
contextBridge.exposeInMainWorld('goAPI', {
  call: (action, payload) => ipcRenderer.invoke('go:invoke', { action, payload })
});

// 🔑 2. Electron Native APIs
contextBridge.exposeInMainWorld('electronAPI', {
  selectFirmware: () => ipcRenderer.invoke('select-firmware-file'),
  // Debug helper
  checkBridge: () => ({ goAPI: !!window.goAPI, electronAPI: !!window.electronAPI })
});