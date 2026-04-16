const { contextBridge, ipcRenderer } = require('electron');
contextBridge.exposeInMainWorld('goAPI', {
    call: (action, payload) => ipcRenderer.invoke('go:invoke', action, payload)
});