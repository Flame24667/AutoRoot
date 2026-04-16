const { contextBridge, ipcRenderer } = require('electron');

try {
    contextBridge.exposeInMainWorld('goAPI', {
        call: (action, payload) => ipcRenderer.invoke('go:invoke', action, payload)
    });
    console.log('[preload] goAPI injected');
    } catch (err) {
    console.error('[preload] Failed to inject goAPI:', err);
}