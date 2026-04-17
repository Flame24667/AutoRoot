const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  // Example: ping: () => ipcRenderer.invoke('ping')
});
